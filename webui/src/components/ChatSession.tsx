import { h, Fragment } from 'preact';
import { useState, useEffect, useRef } from 'preact/hooks';

interface Agent {
  id: string;
  name: string;
  description?: string;
  model?: string;
}

interface Message {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  timestamp: string;
  tool_calls?: ToolCall[];
  thinking?: string;
  isStreaming?: boolean;
}

interface ToolCall {
  id: string;
  name: string;
  input: string;
  output?: string;
  status: 'pending' | 'running' | 'complete' | 'error';
  isError?: boolean;
}

interface ChatSessionProps {
  onClose: () => void;
  initialAgentId?: string;
}

export function ChatSession({ onClose, initialAgentId }: ChatSessionProps) {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [selectedAgent, setSelectedAgent] = useState<Agent | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [inputValue, setInputValue] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [isConnected, setIsConnected] = useState(false);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [showAgentSelector, setShowAgentSelector] = useState(!initialAgentId);
  const [expandedTools, setExpandedTools] = useState<Set<string>>(new Set());

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const eventSourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    fetchAgents();
    return () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }
    };
  }, []);

  useEffect(() => {
    if (initialAgentId && agents.length > 0) {
      const agent = agents.find(a => a.id === initialAgentId);
      if (agent) {
        setSelectedAgent(agent);
        setShowAgentSelector(false);
      }
    }
  }, [initialAgentId, agents]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  useEffect(() => {
    if (selectedAgent && inputRef.current) {
      inputRef.current.focus();
    }
  }, [selectedAgent]);

  const fetchAgents = async () => {
    try {
      const res = await fetch('/api/exec/agents');
      const data = await res.json();
      setAgents(data.agents || []);
    } catch (err) {
      console.error('Failed to fetch agents:', err);
    }
  };

  const startSession = async (agent: Agent) => {
    setSelectedAgent(agent);
    setShowAgentSelector(false);
    setError(null);

    try {
      const res = await fetch('/api/chat/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ agent_id: agent.id }),
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || 'Failed to start session');
      }

      setSessionId(data.session_id);
      setIsConnected(true);
      connectToStream(data.session_id);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to start session');
    }
  };

  const connectToStream = (sessId: string) => {
    const eventSource = new EventSource(`/api/chat/stream/${sessId}`);
    eventSourceRef.current = eventSource;

    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        handleStreamEvent(data);
      } catch (e) {
        console.error('Failed to parse stream event:', e);
      }
    };

    eventSource.onerror = () => {
      setIsConnected(false);
    };
  };

  const handleStreamEvent = (event: any) => {
    switch (event.type) {
      case 'connected':
        // Already connected
        break;

      case 'message_start':
        setMessages(prev => [...prev, {
          id: event.message_id || `msg-${Date.now()}`,
          role: 'assistant',
          content: '',
          timestamp: new Date().toISOString(),
          isStreaming: true,
        }]);
        break;

      case 'content_delta':
        setMessages(prev => {
          const last = prev[prev.length - 1];
          if (last?.role === 'assistant' && last.isStreaming) {
            const newContent = event.content || '';
            // Don't add duplicate content or JSON
            if (newContent && !newContent.startsWith('{') && !last.content.endsWith(newContent)) {
              return [...prev.slice(0, -1), {
                ...last,
                content: last.content + newContent,
              }];
            }
          }
          return prev;
        });
        break;

      case 'thinking':
        setMessages(prev => {
          const last = prev[prev.length - 1];
          if (last?.role === 'assistant') {
            return [...prev.slice(0, -1), {
              ...last,
              thinking: (last.thinking || '') + (event.content || ''),
            }];
          }
          return prev;
        });
        break;

      case 'tool_call':
        setMessages(prev => {
          const last = prev[prev.length - 1];
          if (last?.role === 'assistant') {
            const toolCalls = last.tool_calls || [];
            // Avoid duplicate tool calls
            if (!toolCalls.find(tc => tc.id === event.tool_call_id)) {
              return [...prev.slice(0, -1), {
                ...last,
                tool_calls: [...toolCalls, {
                  id: event.tool_call_id || `tool-${Date.now()}`,
                  name: event.name || 'Tool',
                  input: event.input || '',
                  status: 'running',
                }],
              }];
            }
          }
          return prev;
        });
        break;

      case 'tool_result':
        setMessages(prev => {
          const last = prev[prev.length - 1];
          if (last?.role === 'assistant' && last.tool_calls) {
            const updatedCalls = last.tool_calls.map(tc =>
              tc.id === event.tool_call_id
                ? { ...tc, status: event.is_error ? 'error' as const : 'complete' as const, output: event.output, isError: event.is_error }
                : tc
            );
            return [...prev.slice(0, -1), {
              ...last,
              tool_calls: updatedCalls,
            }];
          }
          return prev;
        });
        break;

      case 'message_end':
      case 'done':
        setMessages(prev => {
          const last = prev[prev.length - 1];
          if (last?.role === 'assistant') {
            return [...prev.slice(0, -1), { ...last, isStreaming: false }];
          }
          return prev;
        });
        setIsLoading(false);
        break;

      case 'error':
        if (event.content) {
          setError(event.content);
        }
        setIsLoading(false);
        break;

      case 'status':
        // Ignore status events
        break;
    }
  };

  const sendMessage = async () => {
    if (!inputValue.trim() || !sessionId || isLoading) return;

    const userMessage: Message = {
      id: `user-${Date.now()}`,
      role: 'user',
      content: inputValue.trim(),
      timestamp: new Date().toISOString(),
    };

    setMessages(prev => [...prev, userMessage]);
    setInputValue('');
    setIsLoading(true);
    setError(null);

    try {
      const res = await fetch('/api/chat/send', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          session_id: sessionId,
          content: userMessage.content,
        }),
      });

      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || 'Failed to send message');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to send message');
      setIsLoading(false);
    }
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  };

  const toggleToolExpand = (toolId: string) => {
    setExpandedTools(prev => {
      const next = new Set(prev);
      if (next.has(toolId)) next.delete(toolId);
      else next.add(toolId);
      return next;
    });
  };

  const formatTime = (ts: string) => {
    try {
      return new Date(ts).toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' });
    } catch { return ''; }
  };

  // Agent selector
  if (showAgentSelector) {
    return (
      <div style={{
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        background: 'var(--bg-primary)',
        borderRadius: '16px',
        overflow: 'hidden',
        boxShadow: '0 4px 24px rgba(0, 0, 0, 0.3)',
      }}>
        <div style={{
          padding: '1.25rem 1.5rem',
          background: 'linear-gradient(135deg, #6366F1, #8B5CF6)',
          display: 'flex',
          alignItems: 'center',
          gap: '1rem',
        }}>
          <div style={{
            width: '48px',
            height: '48px',
            borderRadius: '12px',
            background: 'rgba(255, 255, 255, 0.2)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: '1.5rem',
          }}>
            💬
          </div>
          <div style={{ flex: 1 }}>
            <h3 style={{ margin: 0, fontSize: '1.125rem', fontWeight: 600, color: 'white' }}>
              New Conversation
            </h3>
            <div style={{ fontSize: '0.8125rem', color: 'rgba(255, 255, 255, 0.8)' }}>
              Select an agent to chat with
            </div>
          </div>
          <button
            onClick={onClose}
            style={{
              background: 'rgba(255, 255, 255, 0.2)',
              border: 'none',
              borderRadius: '8px',
              padding: '0.5rem',
              cursor: 'pointer',
              color: 'white',
              fontSize: '1.25rem',
              lineHeight: 1,
            }}
          >
            ×
          </button>
        </div>

        <div style={{ flex: 1, overflow: 'auto', padding: '1rem' }}>
          {agents.length === 0 ? (
            <div style={{ textAlign: 'center', padding: '3rem', color: 'var(--text-muted)' }}>
              <div style={{ fontSize: '3rem', marginBottom: '1rem', opacity: 0.5 }}>🤖</div>
              <div>No agents available</div>
            </div>
          ) : (
            <div style={{ display: 'grid', gap: '0.75rem' }}>
              {agents.map(agent => (
                <div
                  key={agent.id}
                  onClick={() => startSession(agent)}
                  style={{
                    padding: '1rem 1.25rem',
                    background: 'var(--bg-secondary)',
                    borderRadius: '12px',
                    border: '2px solid transparent',
                    cursor: 'pointer',
                    transition: 'all 0.2s',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '1rem',
                  }}
                  onMouseOver={(e) => {
                    (e.currentTarget as HTMLElement).style.borderColor = 'var(--accent)';
                    (e.currentTarget as HTMLElement).style.transform = 'translateX(4px)';
                  }}
                  onMouseOut={(e) => {
                    (e.currentTarget as HTMLElement).style.borderColor = 'transparent';
                    (e.currentTarget as HTMLElement).style.transform = 'translateX(0)';
                  }}
                >
                  <div style={{
                    width: '44px',
                    height: '44px',
                    borderRadius: '12px',
                    background: 'linear-gradient(135deg, #10B981, #34D399)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: '1.25rem',
                    flexShrink: 0,
                  }}>
                    🤖
                  </div>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontWeight: 600, fontSize: '0.9375rem', marginBottom: '0.125rem' }}>
                      {agent.name}
                    </div>
                    {agent.description && (
                      <div style={{
                        fontSize: '0.75rem',
                        color: 'var(--text-muted)',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                      }}>
                        {agent.description}
                      </div>
                    )}
                  </div>
                  <span style={{ color: 'var(--accent)', fontSize: '1.25rem' }}>→</span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    );
  }

  // Chat view
  return (
    <div style={{
      height: '100%',
      display: 'flex',
      flexDirection: 'column',
      background: 'var(--bg-primary)',
      borderRadius: '16px',
      overflow: 'hidden',
      boxShadow: '0 4px 24px rgba(0, 0, 0, 0.3)',
    }}>
      {/* Header */}
      <div style={{
        padding: '0.875rem 1.25rem',
        background: 'linear-gradient(135deg, #6366F1, #8B5CF6)',
        display: 'flex',
        alignItems: 'center',
        gap: '0.875rem',
      }}>
        <div style={{
          width: '40px',
          height: '40px',
          borderRadius: '10px',
          background: 'rgba(255, 255, 255, 0.2)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontSize: '1.25rem',
        }}>
          🤖
        </div>
        <div style={{ flex: 1 }}>
          <div style={{ fontWeight: 600, fontSize: '1rem', color: 'white' }}>
            {selectedAgent?.name || 'Chat'}
          </div>
          <div style={{ fontSize: '0.6875rem', color: 'rgba(255, 255, 255, 0.8)', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <span style={{
              width: '8px',
              height: '8px',
              borderRadius: '50%',
              background: isConnected ? '#4ADE80' : '#F87171',
              boxShadow: isConnected ? '0 0 8px #4ADE80' : '0 0 8px #F87171',
            }} />
            {isConnected ? 'Connected' : 'Disconnected'}
          </div>
        </div>
        <button
          onClick={() => setShowAgentSelector(true)}
          style={{
            background: 'rgba(255, 255, 255, 0.2)',
            border: 'none',
            borderRadius: '8px',
            padding: '0.5rem 0.75rem',
            cursor: 'pointer',
            color: 'white',
            fontSize: '0.75rem',
          }}
        >
          Switch
        </button>
        <button
          onClick={onClose}
          style={{
            background: 'rgba(255, 255, 255, 0.2)',
            border: 'none',
            borderRadius: '8px',
            padding: '0.5rem',
            cursor: 'pointer',
            color: 'white',
            fontSize: '1.25rem',
            lineHeight: 1,
          }}
        >
          ×
        </button>
      </div>

      {/* Error banner */}
      {error && (
        <div style={{
          padding: '0.75rem 1.25rem',
          background: 'rgba(239, 68, 68, 0.15)',
          borderBottom: '1px solid rgba(239, 68, 68, 0.3)',
          color: '#F87171',
          fontSize: '0.8125rem',
          display: 'flex',
          alignItems: 'center',
          gap: '0.5rem',
        }}>
          <span>⚠️</span>
          <span style={{ flex: 1 }}>{error}</span>
          <button
            onClick={() => setError(null)}
            style={{ background: 'none', border: 'none', color: '#F87171', cursor: 'pointer', padding: '0.25rem' }}
          >
            ×
          </button>
        </div>
      )}

      {/* Messages */}
      <div style={{
        flex: 1,
        overflow: 'auto',
        padding: '1.25rem',
        display: 'flex',
        flexDirection: 'column',
        gap: '1rem',
      }}>
        {messages.length === 0 && (
          <div style={{
            flex: 1,
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            color: 'var(--text-muted)',
            textAlign: 'center',
            padding: '2rem',
          }}>
            <div style={{ fontSize: '3rem', marginBottom: '1rem', opacity: 0.5 }}>👋</div>
            <div style={{ fontSize: '1rem', marginBottom: '0.25rem' }}>Ready to chat!</div>
            <div style={{ fontSize: '0.8125rem' }}>Send a message to start the conversation</div>
          </div>
        )}

        {messages.map(message => (
          <div
            key={message.id}
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: message.role === 'user' ? 'flex-end' : 'flex-start',
              gap: '0.375rem',
            }}
          >
            {/* Thinking bubble */}
            {message.thinking && (
              <div style={{
                maxWidth: '85%',
                padding: '0.75rem 1rem',
                background: 'rgba(139, 92, 246, 0.15)',
                borderRadius: '12px',
                borderLeft: '3px solid #8B5CF6',
                fontSize: '0.8125rem',
                color: '#C4B5FD',
                fontStyle: 'italic',
              }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.375rem', marginBottom: '0.375rem', fontWeight: 500 }}>
                  💭 Thinking
                </div>
                {message.thinking}
              </div>
            )}

            {/* Tool calls */}
            {message.tool_calls && message.tool_calls.length > 0 && (
              <div style={{ maxWidth: '85%', display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
                {message.tool_calls.map(tool => {
                  const isExpanded = expandedTools.has(tool.id);
                  return (
                    <div
                      key={tool.id}
                      style={{
                        background: 'var(--bg-secondary)',
                        borderRadius: '12px',
                        overflow: 'hidden',
                        border: `1px solid ${tool.status === 'error' ? 'rgba(239, 68, 68, 0.5)' : 'var(--border)'}`,
                      }}
                    >
                      <div
                        onClick={() => toggleToolExpand(tool.id)}
                        style={{
                          padding: '0.75rem 1rem',
                          display: 'flex',
                          alignItems: 'center',
                          gap: '0.625rem',
                          cursor: 'pointer',
                          background: tool.status === 'running' ? 'rgba(99, 102, 241, 0.1)' :
                                      tool.status === 'error' ? 'rgba(239, 68, 68, 0.1)' :
                                      tool.status === 'complete' ? 'rgba(16, 185, 129, 0.1)' : 'transparent',
                        }}
                      >
                        <div style={{
                          width: '28px',
                          height: '28px',
                          borderRadius: '8px',
                          background: tool.status === 'error' ? '#EF4444' :
                                      tool.status === 'complete' ? '#10B981' :
                                      'linear-gradient(135deg, #6366F1, #8B5CF6)',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          fontSize: '0.875rem',
                          color: 'white',
                        }}>
                          {tool.status === 'running' ? (
                            <span class="spinner" style={{ width: 14, height: 14 }} />
                          ) : tool.status === 'error' ? '✕' : tool.status === 'complete' ? '✓' : '🔧'}
                        </div>
                        <div style={{ flex: 1, minWidth: 0 }}>
                          <div style={{ fontWeight: 600, fontSize: '0.8125rem', color: '#A78BFA' }}>
                            {tool.name}
                          </div>
                          {!isExpanded && tool.input && (
                            <div style={{
                              fontSize: '0.6875rem',
                              color: 'var(--text-muted)',
                              overflow: 'hidden',
                              textOverflow: 'ellipsis',
                              whiteSpace: 'nowrap',
                            }}>
                              {tool.input.length > 60 ? tool.input.slice(0, 60) + '...' : tool.input}
                            </div>
                          )}
                        </div>
                        <span style={{
                          fontSize: '0.75rem',
                          color: 'var(--text-muted)',
                          transform: isExpanded ? 'rotate(180deg)' : 'rotate(0)',
                          transition: 'transform 0.2s',
                        }}>
                          ▼
                        </span>
                      </div>

                      {isExpanded && (
                        <div style={{
                          padding: '0.75rem 1rem',
                          borderTop: '1px solid var(--border)',
                          fontSize: '0.75rem',
                          fontFamily: 'ui-monospace, monospace',
                        }}>
                          {tool.input && (
                            <div style={{ marginBottom: tool.output ? '0.75rem' : 0 }}>
                              <div style={{ color: 'var(--text-muted)', marginBottom: '0.375rem', fontWeight: 500 }}>Input</div>
                              <pre style={{
                                margin: 0,
                                padding: '0.625rem',
                                background: 'var(--bg-hover)',
                                borderRadius: '6px',
                                whiteSpace: 'pre-wrap',
                                wordBreak: 'break-word',
                                maxHeight: '150px',
                                overflow: 'auto',
                                color: 'var(--text-secondary)',
                              }}>
                                {tool.input}
                              </pre>
                            </div>
                          )}
                          {tool.output && (
                            <div>
                              <div style={{
                                color: tool.isError ? '#F87171' : 'var(--text-muted)',
                                marginBottom: '0.375rem',
                                fontWeight: 500,
                              }}>
                                {tool.isError ? 'Error' : 'Output'}
                              </div>
                              <pre style={{
                                margin: 0,
                                padding: '0.625rem',
                                background: tool.isError ? 'rgba(239, 68, 68, 0.1)' : 'var(--bg-hover)',
                                borderRadius: '6px',
                                whiteSpace: 'pre-wrap',
                                wordBreak: 'break-word',
                                maxHeight: '200px',
                                overflow: 'auto',
                                color: tool.isError ? '#F87171' : 'var(--text-secondary)',
                              }}>
                                {tool.output}
                              </pre>
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            )}

            {/* Message bubble */}
            {message.content && (
              <div style={{
                maxWidth: '85%',
                padding: '0.875rem 1.125rem',
                borderRadius: message.role === 'user' ? '18px 18px 4px 18px' : '18px 18px 18px 4px',
                background: message.role === 'user' ? 'linear-gradient(135deg, #6366F1, #8B5CF6)' : 'var(--bg-secondary)',
                color: message.role === 'user' ? 'white' : 'var(--text-primary)',
                fontSize: '0.9375rem',
                lineHeight: 1.5,
                wordBreak: 'break-word',
                whiteSpace: 'pre-wrap',
              }}>
                {message.content}
                {message.isStreaming && (
                  <span class="spinner" style={{ width: 12, height: 12, marginLeft: '0.5rem', verticalAlign: 'middle' }} />
                )}
              </div>
            )}

            {/* Timestamp */}
            <div style={{
              fontSize: '0.625rem',
              color: 'var(--text-muted)',
              paddingLeft: message.role === 'user' ? 0 : '0.5rem',
              paddingRight: message.role === 'user' ? '0.5rem' : 0,
            }}>
              {formatTime(message.timestamp)}
            </div>
          </div>
        ))}

        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <div style={{
        padding: '1rem 1.25rem',
        borderTop: '1px solid var(--border)',
        background: 'var(--bg-secondary)',
      }}>
        <div style={{ display: 'flex', gap: '0.75rem', alignItems: 'flex-end' }}>
          <textarea
            ref={inputRef}
            value={inputValue}
            onInput={(e) => setInputValue((e.target as HTMLTextAreaElement).value)}
            onKeyDown={handleKeyDown}
            disabled={!isConnected || isLoading}
            placeholder={isConnected ? 'Type a message...' : 'Connecting...'}
            rows={1}
            style={{
              flex: 1,
              padding: '0.875rem 1rem',
              borderRadius: '16px',
              border: '2px solid var(--border)',
              background: 'var(--bg-primary)',
              color: 'var(--text-primary)',
              fontSize: '0.9375rem',
              resize: 'none',
              minHeight: '48px',
              maxHeight: '120px',
              fontFamily: 'inherit',
              lineHeight: 1.4,
              outline: 'none',
              transition: 'border-color 0.2s',
            }}
            onFocus={(e) => (e.target as HTMLElement).style.borderColor = 'var(--accent)'}
            onBlur={(e) => (e.target as HTMLElement).style.borderColor = 'var(--border)'}
          />
          <button
            onClick={sendMessage}
            disabled={!inputValue.trim() || !isConnected || isLoading}
            style={{
              padding: '0.875rem 1.5rem',
              borderRadius: '16px',
              border: 'none',
              background: !inputValue.trim() || !isConnected || isLoading
                ? 'var(--bg-hover)'
                : 'linear-gradient(135deg, #6366F1, #8B5CF6)',
              color: !inputValue.trim() || !isConnected || isLoading
                ? 'var(--text-muted)'
                : 'white',
              fontSize: '0.9375rem',
              fontWeight: 600,
              cursor: !inputValue.trim() || !isConnected || isLoading ? 'not-allowed' : 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '0.5rem',
              height: '48px',
              transition: 'all 0.2s',
            }}
          >
            {isLoading ? (
              <span class="spinner" style={{ width: 18, height: 18 }} />
            ) : (
              <>
                Send
                <span style={{ fontSize: '1.125rem' }}>↑</span>
              </>
            )}
          </button>
        </div>
        <div style={{
          marginTop: '0.5rem',
          fontSize: '0.6875rem',
          color: 'var(--text-muted)',
          display: 'flex',
          justifyContent: 'space-between',
        }}>
          <span>Enter to send • Shift+Enter for new line</span>
          <span>{messages.filter(m => m.role !== 'system').length} messages</span>
        </div>
      </div>
    </div>
  );
}
