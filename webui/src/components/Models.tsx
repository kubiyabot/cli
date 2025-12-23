import { h, Fragment } from 'preact';
import { useState, useEffect, useRef } from 'preact/hooks';
import { Skeleton, SkeletonText } from './Skeleton';

interface ModelInfo {
  id: string;
  value: string;
  label: string;
  provider: string;
  logo?: string;
  enabled: boolean;
  recommended: boolean;
  capabilities?: Record<string, unknown>;
  pricing?: Record<string, unknown>;
  compatible_runtimes?: string[];
}

interface ProviderStatus {
  name: string;
  connected: boolean;
  latency_ms?: number;
  error?: string;
  model_count: number;
  logo?: string;
}

interface LLMInsights {
  providers: ProviderStatus[];
  models: ModelInfo[];
  default_model?: ModelInfo;
  last_updated: string;
  cached_until: string;
}

interface ChatMessage {
  role: 'user' | 'assistant' | 'system';
  content: string;
}

interface ChatTestResult {
  success: boolean;
  response?: string;
  latency_ms?: number;
  tokens_used?: number;
  error?: string;
}

// Provider logo URLs (real SVG/PNG logos)
const PROVIDER_LOGOS: Record<string, string> = {
  'anthropic': 'https://cdn.worldvectorlogo.com/logos/anthropic-1.svg',
  'openai': 'https://cdn.worldvectorlogo.com/logos/openai-2.svg',
  'google': 'https://www.gstatic.com/lamda/images/gemini_sparkle_v002_d4735304ff6292a690345.svg',
  'meta': 'https://cdn.worldvectorlogo.com/logos/meta-1.svg',
  'cohere': 'https://asset.brandfetch.io/idnRPijCxn/idHoZNvfAe.svg',
  'mistral': 'https://mistral.ai/images/logo_mistral_ai.svg',
  'groq': 'https://groq.com/wp-content/uploads/2024/03/PBG-mark1-color.svg',
  'aws': 'https://upload.wikimedia.org/wikipedia/commons/9/93/Amazon_Web_Services_Logo.svg',
  'azure': 'https://cdn.worldvectorlogo.com/logos/azure-1.svg',
  'vertex': 'https://www.gstatic.com/devrel-devsite/prod/v0e0f589edd85502a40d78d7d0825db8ea5ef3b99ab4070381ee86977c9168730/cloud/images/cloud-logo.svg',
  'deepseek': 'https://chat.deepseek.com/favicon.ico',
  'together': 'https://www.together.ai/favicon.ico',
  'fireworks': 'https://fireworks.ai/favicon.ico',
  'perplexity': 'https://www.perplexity.ai/favicon.svg',
  'replicate': 'https://replicate.com/favicon.ico',
};

// Provider brand colors
const PROVIDER_COLORS: Record<string, string> = {
  'anthropic': '#D4A27F',
  'openai': '#10A37F',
  'google': '#4285F4',
  'meta': '#0668E1',
  'cohere': '#D18EE2',
  'mistral': '#F54E42',
  'groq': '#F55036',
  'aws': '#FF9900',
  'azure': '#0089D6',
  'vertex': '#4285F4',
  'deepseek': '#536DFE',
  'together': '#6366F1',
  'fireworks': '#FF6B35',
  'perplexity': '#20808D',
  'replicate': '#000000',
};

// Provider emoji fallbacks
const PROVIDER_EMOJIS: Record<string, string> = {
  'anthropic': '🅰️',
  'openai': '🧠',
  'google': '🔵',
  'meta': '📘',
  'cohere': '💬',
  'mistral': '🌪️',
  'groq': '⚡',
  'aws': '☁️',
  'azure': '🔷',
  'vertex': '🔺',
  'deepseek': '🔍',
  'together': '🤝',
  'fireworks': '🎆',
  'perplexity': '🔮',
  'replicate': '🔄',
};

export function Models() {
  const [insights, setInsights] = useState<LLMInsights | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchTerm, setSearchTerm] = useState('');
  const [providerFilter, setProviderFilter] = useState('');
  const [showCapabilities, setShowCapabilities] = useState<Set<string>>(new Set());

  // Chat testing state
  const [chatModel, setChatModel] = useState<string | null>(null);
  const [chatInput, setChatInput] = useState('');
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([]);
  const [chatLoading, setChatLoading] = useState(false);
  const [chatResult, setChatResult] = useState<ChatTestResult | null>(null);
  const chatEndRef = useRef<HTMLDivElement>(null);

  const fetchInsights = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch('/api/llm/insights');
      const data = await res.json();
      if (data.error) {
        setError(data.error);
      }
      setInsights(data);
    } catch (err) {
      setError(`Failed to fetch LLM insights: ${err}`);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchInsights();
  }, []);

  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [chatMessages]);

  const toggleCapabilities = (modelId: string) => {
    const newSet = new Set(showCapabilities);
    if (newSet.has(modelId)) {
      newSet.delete(modelId);
    } else {
      newSet.add(modelId);
    }
    setShowCapabilities(newSet);
  };

  const normalizeProviderName = (provider: string): string => {
    return provider.toLowerCase().replace(/[\s-_]+/g, '');
  };

  const getProviderLogo = (provider: string): string | null => {
    const normalized = normalizeProviderName(provider);
    return PROVIDER_LOGOS[normalized] || null;
  };

  const getProviderColor = (provider: string): string => {
    const normalized = normalizeProviderName(provider);
    return PROVIDER_COLORS[normalized] || '#6B7280';
  };

  const getProviderEmoji = (provider: string): string => {
    const normalized = normalizeProviderName(provider);
    return PROVIDER_EMOJIS[normalized] || '🤖';
  };

  const sendChatMessage = async () => {
    if (!chatInput.trim() || !chatModel) return;

    const userMessage: ChatMessage = { role: 'user', content: chatInput };
    setChatMessages((prev) => [...prev, userMessage]);
    setChatInput('');
    setChatLoading(true);
    setChatResult(null);

    try {
      const res = await fetch('/api/llm/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          model: chatModel,
          messages: [...chatMessages, userMessage],
        }),
      });
      const data = await res.json();

      if (data.success && data.response) {
        const assistantMessage: ChatMessage = { role: 'assistant', content: data.response };
        setChatMessages((prev) => [...prev, assistantMessage]);
      }
      setChatResult(data);
    } catch (err) {
      setChatResult({
        success: false,
        error: `Request failed: ${err}`,
      });
    } finally {
      setChatLoading(false);
    }
  };

  const openChatTest = (modelValue: string) => {
    setChatModel(modelValue);
    setChatMessages([]);
    setChatResult(null);
    setChatInput('');
  };

  const closeChatTest = () => {
    setChatModel(null);
    setChatMessages([]);
    setChatResult(null);
  };

  const filteredModels = insights?.models?.filter((model) => {
    if (searchTerm && !model.label.toLowerCase().includes(searchTerm.toLowerCase()) &&
        !model.value.toLowerCase().includes(searchTerm.toLowerCase())) {
      return false;
    }
    if (providerFilter && model.provider !== providerFilter) {
      return false;
    }
    return true;
  }) || [];

  const uniqueProviders = [...new Set(insights?.models?.map((m) => m.provider) || [])];

  if (loading && !insights) {
    return (
      <Fragment>
        {/* Header skeleton */}
        <div style={{ marginBottom: '1rem', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Skeleton variant="text" width={220} height="1.5rem" />
          <Skeleton variant="rectangular" width={90} height={36} />
        </div>

        {/* Provider cards skeleton */}
        <div style={{ marginBottom: '1.5rem' }}>
          <Skeleton variant="text" width={100} height="1.125rem" className="skeleton-mb-sm" />
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: '0.75rem', marginTop: '0.75rem' }}>
            {[1, 2, 3, 4].map((i) => (
              <div key={i} class="card" style={{ padding: '1rem' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '0.75rem' }}>
                  <Skeleton variant="rectangular" width={40} height={40} />
                  <div style={{ flex: 1 }}>
                    <Skeleton variant="text" width="70%" height="1rem" />
                    <Skeleton variant="text" width="40%" height="0.75rem" />
                  </div>
                </div>
                <Skeleton variant="text" width={80} height="1.25rem" />
              </div>
            ))}
          </div>
        </div>

        {/* Default model skeleton */}
        <div class="card" style={{ marginBottom: '1rem', padding: '1rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
            <Skeleton variant="rectangular" width={56} height={56} />
            <div style={{ flex: 1 }}>
              <Skeleton variant="text" width={100} height="0.75rem" />
              <Skeleton variant="text" width={180} height="1.5rem" />
              <Skeleton variant="text" width={140} height="0.875rem" />
            </div>
            <Skeleton variant="rectangular" width={70} height={36} />
          </div>
        </div>

        {/* Filter bar skeleton */}
        <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem' }}>
          <Skeleton variant="rectangular" width="100%" height={38} />
          <Skeleton variant="rectangular" width={150} height={38} />
          <Skeleton variant="rectangular" width={60} height={38} />
        </div>

        {/* Models count skeleton */}
        <Skeleton variant="text" width={180} height="0.875rem" className="skeleton-mb-sm" />

        {/* Model cards skeleton */}
        <div style={{ display: 'grid', gap: '0.75rem', marginTop: '0.75rem' }}>
          {[1, 2, 3, 4, 5].map((i) => (
            <div key={i} class="card" style={{ padding: '1rem' }}>
              <div style={{ display: 'flex', alignItems: 'flex-start', gap: '1rem' }}>
                <Skeleton variant="rectangular" width={44} height={44} />
                <div style={{ flex: 1 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '0.25rem' }}>
                    <Skeleton variant="text" width={150} height="1rem" />
                    <Skeleton variant="rectangular" width={70} height={18} />
                  </div>
                  <Skeleton variant="text" width="60%" height="0.75rem" />
                  <div style={{ display: 'flex', gap: '0.25rem', marginTop: '0.5rem' }}>
                    <Skeleton variant="rectangular" width={60} height={18} />
                    <Skeleton variant="rectangular" width={50} height={18} />
                    <Skeleton variant="rectangular" width={70} height={18} />
                  </div>
                </div>
                <div style={{ display: 'flex', gap: '0.375rem' }}>
                  <Skeleton variant="rectangular" width={65} height={32} />
                  <Skeleton variant="rectangular" width={32} height={32} />
                </div>
              </div>
            </div>
          ))}
        </div>
      </Fragment>
    );
  }

  // Chat Test Modal
  const renderChatModal = () => {
    if (!chatModel) return null;

    const selectedModel = insights?.models?.find((m) => m.value === chatModel);

    return (
      <div
        style={{
          position: 'fixed',
          inset: 0,
          background: 'rgba(0,0,0,0.7)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          zIndex: 1000,
          padding: '1rem',
        }}
        onClick={(e) => {
          if (e.target === e.currentTarget) closeChatTest();
        }}
      >
        <div
          class="card"
          style={{
            width: '100%',
            maxWidth: '700px',
            maxHeight: '80vh',
            display: 'flex',
            flexDirection: 'column',
            overflow: 'hidden',
          }}
        >
          {/* Header */}
          <div
            style={{
              padding: '1rem',
              borderBottom: '1px solid var(--border)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
              {selectedModel && (
                <div
                  style={{
                    width: '36px',
                    height: '36px',
                    borderRadius: '8px',
                    background: getProviderColor(selectedModel.provider),
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    overflow: 'hidden',
                  }}
                >
                  {getProviderLogo(selectedModel.provider) ? (
                    <img
                      src={getProviderLogo(selectedModel.provider)!}
                      alt={selectedModel.provider}
                      style={{ width: '24px', height: '24px', objectFit: 'contain' }}
                      onError={(e) => {
                        (e.target as HTMLImageElement).style.display = 'none';
                        (e.target as HTMLImageElement).nextElementSibling!.removeAttribute('style');
                      }}
                    />
                  ) : null}
                  <span style={{ fontSize: '1.25rem', display: getProviderLogo(selectedModel.provider) ? 'none' : 'block' }}>
                    {getProviderEmoji(selectedModel.provider)}
                  </span>
                </div>
              )}
              <div>
                <div style={{ fontWeight: 600 }}>{selectedModel?.label || chatModel}</div>
                <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                  Chat Completion Test
                </div>
              </div>
            </div>
            <button
              class="btn btn-secondary"
              style={{ padding: '0.25rem 0.5rem' }}
              onClick={closeChatTest}
            >
              ✕
            </button>
          </div>

          {/* Messages */}
          <div
            style={{
              flex: 1,
              overflow: 'auto',
              padding: '1rem',
              display: 'flex',
              flexDirection: 'column',
              gap: '0.75rem',
              minHeight: '300px',
            }}
          >
            {chatMessages.length === 0 && (
              <div style={{ textAlign: 'center', color: 'var(--text-muted)', padding: '2rem' }}>
                <div style={{ fontSize: '2rem', marginBottom: '0.5rem' }}>💬</div>
                <div>Send a message to test the model</div>
              </div>
            )}
            {chatMessages.map((msg, idx) => (
              <div
                key={idx}
                style={{
                  alignSelf: msg.role === 'user' ? 'flex-end' : 'flex-start',
                  maxWidth: '80%',
                  padding: '0.75rem 1rem',
                  borderRadius: '12px',
                  background: msg.role === 'user' ? 'var(--accent)' : 'var(--bg-hover)',
                  color: msg.role === 'user' ? 'white' : 'var(--text-primary)',
                }}
              >
                <div style={{ fontSize: '0.8125rem', whiteSpace: 'pre-wrap' }}>{msg.content}</div>
              </div>
            ))}
            {chatLoading && (
              <div
                style={{
                  alignSelf: 'flex-start',
                  padding: '0.75rem 1rem',
                  borderRadius: '12px',
                  background: 'var(--bg-hover)',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '0.5rem',
                }}
              >
                <span class="spinner" style={{ width: 14, height: 14 }} />
                <span style={{ color: 'var(--text-secondary)' }}>Thinking...</span>
              </div>
            )}
            <div ref={chatEndRef} />
          </div>

          {/* Result info */}
          {chatResult && (
            <div
              style={{
                padding: '0.5rem 1rem',
                borderTop: '1px solid var(--border)',
                fontSize: '0.75rem',
                color: chatResult.success ? 'var(--success)' : 'var(--error)',
                display: 'flex',
                alignItems: 'center',
                gap: '1rem',
              }}
            >
              {chatResult.success ? (
                <Fragment>
                  <span>✓ Success</span>
                  {chatResult.latency_ms && <span>Latency: {chatResult.latency_ms}ms</span>}
                  {chatResult.tokens_used && <span>Tokens: {chatResult.tokens_used}</span>}
                </Fragment>
              ) : (
                <span>✗ {chatResult.error}</span>
              )}
            </div>
          )}

          {/* Input */}
          <div
            style={{
              padding: '1rem',
              borderTop: '1px solid var(--border)',
              display: 'flex',
              gap: '0.5rem',
            }}
          >
            <input
              type="text"
              class="filter-input"
              style={{ flex: 1 }}
              placeholder="Type a message..."
              value={chatInput}
              onInput={(e) => setChatInput((e.target as HTMLInputElement).value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault();
                  sendChatMessage();
                }
              }}
              disabled={chatLoading}
            />
            <button
              class="btn btn-primary"
              onClick={sendChatMessage}
              disabled={chatLoading || !chatInput.trim()}
            >
              {chatLoading ? (
                <span class="spinner" style={{ width: 14, height: 14 }} />
              ) : (
                'Send'
              )}
            </button>
          </div>
        </div>
      </div>
    );
  };

  return (
    <Fragment>
      {renderChatModal()}

      {/* Header */}
      <div style={{ marginBottom: '1rem', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2 style={{ fontSize: '1.25rem', fontWeight: 600 }}>LLM Models & Providers</h2>
        <button
          class="btn btn-secondary"
          onClick={fetchInsights}
          disabled={loading}
        >
          {loading ? (
            <Fragment>
              <span class="spinner" style={{ width: 14, height: 14 }} />
              Loading...
            </Fragment>
          ) : (
            '↻ Refresh'
          )}
        </button>
      </div>

      {error && (
        <div class="card" style={{ borderColor: 'var(--warning)', marginBottom: '1rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', color: 'var(--warning)' }}>
            <span>⚠️</span>
            <span>{error}</span>
          </div>
        </div>
      )}

      {/* Provider Cards */}
      {insights?.providers && insights.providers.length > 0 && (
        <div style={{ marginBottom: '1.5rem' }}>
          <h3 style={{ fontSize: '1rem', fontWeight: 600, marginBottom: '0.75rem' }}>Providers</h3>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: '0.75rem' }}>
            {insights.providers.map((provider) => {
              const logo = getProviderLogo(provider.name);
              const color = getProviderColor(provider.name);
              const emoji = getProviderEmoji(provider.name);

              return (
                <div
                  key={provider.name}
                  class="card"
                  style={{
                    padding: '1rem',
                    cursor: 'pointer',
                    borderColor: providerFilter === provider.name ? 'var(--accent)' : undefined,
                    transition: 'all 0.2s ease',
                  }}
                  onClick={() => setProviderFilter(providerFilter === provider.name ? '' : provider.name)}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '0.75rem' }}>
                    <div
                      style={{
                        width: '40px',
                        height: '40px',
                        borderRadius: '10px',
                        background: `linear-gradient(135deg, ${color}20, ${color}40)`,
                        border: `2px solid ${color}`,
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        overflow: 'hidden',
                        flexShrink: 0,
                      }}
                    >
                      {logo ? (
                        <img
                          src={logo}
                          alt={provider.name}
                          style={{ width: '24px', height: '24px', objectFit: 'contain' }}
                          onError={(e) => {
                            const target = e.target as HTMLImageElement;
                            target.style.display = 'none';
                            const parent = target.parentElement;
                            if (parent) {
                              const span = document.createElement('span');
                              span.style.fontSize = '1.5rem';
                              span.textContent = emoji;
                              parent.appendChild(span);
                            }
                          }}
                        />
                      ) : (
                        <span style={{ fontSize: '1.5rem' }}>{emoji}</span>
                      )}
                    </div>
                    <div style={{ flex: 1 }}>
                      <div style={{ fontWeight: 600, fontSize: '0.9375rem' }}>{provider.name}</div>
                      <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                        {provider.model_count || 0} model{(provider.model_count || 0) !== 1 ? 's' : ''}
                      </div>
                    </div>
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <span
                      class={`badge ${provider.connected ? 'badge-success' : 'badge-error'}`}
                      style={{ fontSize: '0.6875rem' }}
                    >
                      {provider.connected ? '● Connected' : '○ Disconnected'}
                    </span>
                    {provider.latency_ms && (
                      <span style={{ fontSize: '0.6875rem', color: 'var(--text-muted)' }}>
                        {provider.latency_ms}ms
                      </span>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Default Model */}
      {insights?.default_model && (
        <div class="card" style={{ marginBottom: '1rem', borderColor: 'var(--accent)', background: 'linear-gradient(135deg, var(--accent)08, var(--accent)15)' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
            <div
              style={{
                width: '56px',
                height: '56px',
                borderRadius: '12px',
                background: `linear-gradient(135deg, ${getProviderColor(insights.default_model.provider)}, ${getProviderColor(insights.default_model.provider)}CC)`,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
              }}
            >
              {getProviderLogo(insights.default_model.provider) ? (
                <img
                  src={getProviderLogo(insights.default_model.provider)!}
                  alt={insights.default_model.provider}
                  style={{ width: '32px', height: '32px', objectFit: 'contain', filter: 'brightness(0) invert(1)' }}
                  onError={(e) => {
                    (e.target as HTMLImageElement).style.display = 'none';
                  }}
                />
              ) : (
                <span style={{ fontSize: '1.75rem' }}>{getProviderEmoji(insights.default_model.provider)}</span>
              )}
            </div>
            <div style={{ flex: 1 }}>
              <div style={{ fontSize: '0.75rem', color: 'var(--accent)', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                ⭐ Default Model
              </div>
              <div style={{ fontSize: '1.25rem', fontWeight: 700, marginTop: '0.125rem' }}>{insights.default_model.label}</div>
              <div style={{ fontSize: '0.8125rem', color: 'var(--text-secondary)', marginTop: '0.125rem' }}>
                {insights.default_model.provider} • <code style={{ background: 'var(--bg-hover)', padding: '0.125rem 0.375rem', borderRadius: '4px', fontSize: '0.75rem' }}>{insights.default_model.value}</code>
              </div>
            </div>
            <button
              class="btn btn-primary"
              onClick={() => openChatTest(insights.default_model!.value)}
              style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}
            >
              💬 Test
            </button>
          </div>
        </div>
      )}

      {/* Search and Filter */}
      <div class="filter-bar" style={{ marginBottom: '1rem' }}>
        <input
          type="text"
          class="filter-input"
          placeholder="Search models..."
          value={searchTerm}
          onInput={(e) => setSearchTerm((e.target as HTMLInputElement).value)}
          style={{ flex: 1 }}
        />
        <select
          class="filter-select"
          value={providerFilter}
          onChange={(e) => setProviderFilter((e.target as HTMLSelectElement).value)}
        >
          <option value="">All Providers</option>
          {uniqueProviders.map((p) => (
            <option key={p} value={p}>{p}</option>
          ))}
        </select>
        <button
          class="btn btn-secondary"
          onClick={() => { setSearchTerm(''); setProviderFilter(''); }}
          style={{ fontSize: '0.75rem', padding: '0.4rem 0.6rem' }}
        >
          Clear
        </button>
      </div>

      {/* Models Count */}
      <div style={{ marginBottom: '0.75rem', fontSize: '0.8125rem', color: 'var(--text-secondary)' }}>
        Showing {filteredModels.length} of {insights?.models?.length || 0} models
      </div>

      {/* Models Grid */}
      <div style={{ display: 'grid', gap: '0.75rem' }}>
        {filteredModels.length === 0 ? (
          <div class="card" style={{ textAlign: 'center', padding: '3rem' }}>
            <div style={{ fontSize: '2rem', marginBottom: '0.5rem', opacity: 0.5 }}>🔮</div>
            <div style={{ color: 'var(--text-secondary)' }}>
              {insights?.models?.length === 0 ? 'No models available' : 'No models match your search'}
            </div>
          </div>
        ) : (
          filteredModels.map((model) => {
            const logo = getProviderLogo(model.provider);
            const color = getProviderColor(model.provider);
            const emoji = getProviderEmoji(model.provider);

            return (
              <div key={model.id} class="card" style={{ padding: '1rem' }}>
                <div style={{ display: 'flex', alignItems: 'flex-start', gap: '1rem' }}>
                  {/* Provider icon */}
                  <div
                    style={{
                      width: '44px',
                      height: '44px',
                      borderRadius: '10px',
                      background: `linear-gradient(135deg, ${color}15, ${color}30)`,
                      border: `1px solid ${color}50`,
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      flexShrink: 0,
                    }}
                  >
                    {logo ? (
                      <img
                        src={logo}
                        alt={model.provider}
                        style={{ width: '26px', height: '26px', objectFit: 'contain' }}
                        onError={(e) => {
                          const target = e.target as HTMLImageElement;
                          target.style.display = 'none';
                          const parent = target.parentElement;
                          if (parent) {
                            const span = document.createElement('span');
                            span.style.fontSize = '1.5rem';
                            span.textContent = emoji;
                            parent.appendChild(span);
                          }
                        }}
                      />
                    ) : (
                      <span style={{ fontSize: '1.5rem' }}>{emoji}</span>
                    )}
                  </div>

                  {/* Model info */}
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '0.25rem', flexWrap: 'wrap' }}>
                      <span style={{ fontWeight: 600, fontSize: '0.9375rem' }}>{model.label}</span>
                      {model.recommended && (
                        <span class="badge badge-success" style={{ fontSize: '0.625rem', padding: '0.125rem 0.375rem' }}>
                          ⭐ Recommended
                        </span>
                      )}
                      {!model.enabled && (
                        <span class="badge badge-warning" style={{ fontSize: '0.625rem', padding: '0.125rem 0.375rem' }}>
                          Disabled
                        </span>
                      )}
                    </div>
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginBottom: '0.5rem' }}>
                      <code style={{ background: 'var(--bg-hover)', padding: '0.125rem 0.375rem', borderRadius: '4px', fontSize: '0.6875rem' }}>
                        {model.value}
                      </code>
                      <span style={{ margin: '0 0.5rem', color: 'var(--border)' }}>•</span>
                      <span style={{ color: color, fontWeight: 500 }}>{model.provider}</span>
                    </div>

                    {/* Capabilities badges */}
                    {model.capabilities && Object.keys(model.capabilities).length > 0 && (
                      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.25rem' }}>
                        {Object.entries(model.capabilities).slice(0, showCapabilities.has(model.id) ? undefined : 4).map(([key, value]) => {
                          if (value === true || value === 'true') {
                            return (
                              <span
                                key={key}
                                class="badge badge-info"
                                style={{ fontSize: '0.5625rem', padding: '0.125rem 0.375rem', textTransform: 'capitalize' }}
                              >
                                {key.replace(/_/g, ' ')}
                              </span>
                            );
                          }
                          return null;
                        })}
                        {Object.keys(model.capabilities).filter(k => model.capabilities![k] === true || model.capabilities![k] === 'true').length > 4 && !showCapabilities.has(model.id) && (
                          <span
                            class="badge"
                            style={{ fontSize: '0.5625rem', padding: '0.125rem 0.375rem', cursor: 'pointer' }}
                            onClick={() => toggleCapabilities(model.id)}
                          >
                            +{Object.keys(model.capabilities).filter(k => model.capabilities![k] === true || model.capabilities![k] === 'true').length - 4} more
                          </span>
                        )}
                      </div>
                    )}

                    {/* Expanded capabilities */}
                    {showCapabilities.has(model.id) && model.capabilities && (
                      <div style={{ marginTop: '0.75rem', padding: '0.75rem', background: 'var(--bg-hover)', borderRadius: '6px' }}>
                        <div style={{ fontSize: '0.6875rem', color: 'var(--text-muted)', marginBottom: '0.5rem', fontWeight: 600, textTransform: 'uppercase' }}>
                          Full Capabilities
                        </div>
                        <pre style={{ fontSize: '0.625rem', margin: 0, overflow: 'auto', maxHeight: '200px' }}>
                          {JSON.stringify(model.capabilities, null, 2)}
                        </pre>
                      </div>
                    )}
                  </div>

                  {/* Actions */}
                  <div style={{ display: 'flex', gap: '0.375rem', flexShrink: 0 }}>
                    <button
                      class="btn btn-primary"
                      style={{ padding: '0.375rem 0.75rem', fontSize: '0.75rem' }}
                      onClick={() => openChatTest(model.value)}
                      title="Test chat completion"
                    >
                      💬 Test
                    </button>
                    <button
                      class="btn btn-secondary"
                      style={{ padding: '0.375rem 0.5rem', fontSize: '0.75rem' }}
                      onClick={() => toggleCapabilities(model.id)}
                      title="View details"
                    >
                      {showCapabilities.has(model.id) ? '▼' : '▶'}
                    </button>
                  </div>
                </div>
              </div>
            );
          })
        )}
      </div>

      {/* Cache info */}
      {insights?.last_updated && (
        <div style={{ marginTop: '1rem', fontSize: '0.75rem', color: 'var(--text-muted)', textAlign: 'center' }}>
          Last updated: {new Date(insights.last_updated).toLocaleString()}
          {insights.cached_until && (
            <span> • Cached until: {new Date(insights.cached_until).toLocaleString()}</span>
          )}
        </div>
      )}
    </Fragment>
  );
}
