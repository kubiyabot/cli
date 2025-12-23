import { h, Fragment } from 'preact';
import { useState, useEffect, useRef } from 'preact/hooks';
import { StreamView, StreamEvent } from './StreamView';
import { KubiyaLogo, AgentIcon } from './Logo';

interface Agent {
  id: string;
  name: string;
  description?: string;
  model?: string;
}

interface Team {
  id: string;
  name: string;
  description?: string;
}

interface Environment {
  id: string;
  name: string;
}

interface ExecutionConfig {
  prompt: string;
  mode: 'auto' | 'agent' | 'team';
  entityId?: string;
  local: boolean;
  directApi: boolean; // Use direct control plane API (more reliable streaming)
  environment?: string;
  workingDir?: string;
  streamFormat: 'text' | 'json';
  verbose: boolean;
}

export function Playground() {
  // Resources
  const [agents, setAgents] = useState<Agent[]>([]);
  const [teams, setTeams] = useState<Team[]>([]);
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [loadingResources, setLoadingResources] = useState(true);

  // Execution config
  const [config, setConfig] = useState<ExecutionConfig>({
    prompt: '',
    mode: 'auto',
    entityId: undefined,
    local: true,
    directApi: false, // Default to CLI-based, toggle for direct API
    environment: undefined,
    workingDir: '',
    streamFormat: 'json', // Default to JSON for structured tool call events
    verbose: false,
  });

  // Execution state
  const [executing, setExecuting] = useState(false);
  const [events, setEvents] = useState<StreamEvent[]>([]);
  const [executionId, setExecutionId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [showConfig, setShowConfig] = useState(true);
  const eventSourceRef = useRef<EventSource | null>(null);

  // Fetch resources on mount
  useEffect(() => {
    fetchResources();
    return () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }
    };
  }, []);

  const fetchResources = async () => {
    setLoadingResources(true);
    try {
      const [agentsRes, teamsRes, envsRes] = await Promise.all([
        fetch('/api/exec/agents'),
        fetch('/api/exec/teams'),
        fetch('/api/exec/environments'),
      ]);

      const [agentsData, teamsData, envsData] = await Promise.all([
        agentsRes.json().catch(() => ({ agents: [] })),
        teamsRes.json().catch(() => ({ teams: [] })),
        envsRes.json().catch(() => ({ environments: [] })),
      ]);

      setAgents(agentsData.agents || []);
      setTeams(teamsData.teams || []);
      setEnvironments(envsData.environments || []);
    } catch (err) {
      console.error('Failed to fetch resources:', err);
    } finally {
      setLoadingResources(false);
    }
  };

  const startExecution = async () => {
    if (!config.prompt.trim()) {
      setError('Please enter a prompt');
      return;
    }

    // Direct API requires an agent to be selected
    if (config.directApi && config.mode !== 'agent') {
      setError('Direct API mode requires selecting a specific agent');
      return;
    }

    if (config.directApi && !config.entityId) {
      setError('Please select an agent for direct API mode');
      return;
    }

    setExecuting(true);
    setEvents([]);
    setError(null);
    setExecutionId(null);
    setShowConfig(false);

    // Add initial event
    const startMsg = config.directApi
      ? 'Submitting to control plane API...'
      : (config.local ? 'Starting local execution...' : 'Starting execution...');
    addEvent({
      type: 'status',
      content: startMsg,
      timestamp: new Date().toISOString(),
      status: 'starting',
    });

    try {
      // Choose endpoint based on mode
      const endpoint = config.directApi ? '/api/exec/direct/start' : '/api/exec/start';
      const streamEndpoint = config.directApi ? '/api/exec/direct/stream' : '/api/exec/stream';

      // Start execution via API
      const response = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
      });

      const data = await response.json();

      if (!response.ok) {
        throw new Error(data.error || 'Failed to start execution');
      }

      setExecutionId(data.execution_id);

      // Connect to SSE stream for events
      const eventSource = new EventSource(`${streamEndpoint}/${data.execution_id}`);
      eventSourceRef.current = eventSource;

      eventSource.onmessage = (event) => {
        try {
          const parsed = JSON.parse(event.data);
          addEvent(parsed);

          if (parsed.type === 'done' || parsed.type === 'error') {
            eventSource.close();
            setExecuting(false);
          }
        } catch (e) {
          // Plain text event
          addEvent({
            type: 'text',
            content: event.data,
            timestamp: new Date().toISOString(),
          });
        }
      };

      eventSource.onerror = () => {
        eventSource.close();
        setExecuting(false);
        addEvent({
          type: 'error',
          content: 'Connection lost',
          timestamp: new Date().toISOString(),
        });
      };
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
      setExecuting(false);
      addEvent({
        type: 'error',
        content: err instanceof Error ? err.message : 'Unknown error',
        timestamp: new Date().toISOString(),
      });
    }
  };

  const stopExecution = async () => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }

    if (executionId) {
      try {
        const stopEndpoint = config.directApi
          ? `/api/exec/direct/stop/${executionId}`
          : `/api/exec/stop/${executionId}`;
        await fetch(stopEndpoint, { method: 'POST' });
      } catch (e) {
        // Ignore errors
      }
    }

    setExecuting(false);
    addEvent({
      type: 'status',
      content: 'Execution stopped by user',
      timestamp: new Date().toISOString(),
      status: 'stopped',
    });
  };

  const addEvent = (event: StreamEvent) => {
    setEvents((prev) => [...prev, event]);
  };

  const clearEvents = () => {
    setEvents([]);
    setExecutionId(null);
    setError(null);
  };

  const newExecution = () => {
    clearEvents();
    setShowConfig(true);
    setConfig(prev => ({ ...prev, prompt: '' }));
  };

  if (loadingResources) {
    return (
      <div class="card playground-loading">
        <div class="playground-loading-content">
          <div class="playground-loading-icon">
            <KubiyaLogo size={48} />
          </div>
          <div class="playground-loading-text">Loading resources...</div>
          <div class="playground-loading-spinner">
            <span class="spinner" />
          </div>
        </div>
      </div>
    );
  }

  return (
    <div class="playground-container">
      {/* Header */}
      <div class="playground-header">
        <div class="playground-header-left">
          <div class="playground-title-group">
            <AgentIcon size={32} />
            <div>
              <h2 class="playground-title">Execution Playground</h2>
              <div class="playground-subtitle">
                <span class="playground-stat">{agents.length} agents</span>
                <span class="playground-stat-divider" />
                <span class="playground-stat">{teams.length} teams</span>
                <span class="playground-stat-divider" />
                <span class="playground-stat">{environments.length} environments</span>
              </div>
            </div>
          </div>
        </div>
        <div class="playground-header-right">
          {!showConfig && !executing && (
            <button class="btn btn-primary playground-new-btn" onClick={newExecution}>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
                <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
              </svg>
              New Execution
            </button>
          )}
          <button
            class="btn btn-secondary playground-refresh-btn"
            onClick={fetchResources}
            disabled={loadingResources}
            title="Refresh resources"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M23 4v6h-6M1 20v-6h6"/>
              <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
            </svg>
          </button>
        </div>
      </div>

      {/* Main Content */}
      <div class="playground-main">
        {/* Config Panel */}
        {showConfig && (
          <div class="card playground-config-panel">
            <div class="card-header">
              <h3 class="card-title">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
                </svg>
                Configuration
              </h3>
            </div>

            <div class="playground-config-content">
              {/* Prompt */}
              <div class="playground-field">
                <label class="playground-label">Prompt</label>
                <textarea
                  class="playground-textarea"
                  placeholder="What would you like to do?"
                  value={config.prompt}
                  onInput={(e) => setConfig({ ...config, prompt: (e.target as HTMLTextAreaElement).value })}
                  disabled={executing}
                />
              </div>

              {/* Mode Selection */}
              <div class="playground-field">
                <label class="playground-label">Mode</label>
                <div class="playground-button-group">
                  {(['auto', 'agent', 'team'] as const).map((mode) => (
                    <button
                      key={mode}
                      class={`playground-mode-btn ${config.mode === mode ? 'active' : ''}`}
                      onClick={() => setConfig({ ...config, mode, entityId: undefined })}
                      disabled={executing}
                    >
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                        {mode === 'auto' && <><circle cx="12" cy="12" r="10"/><path d="M12 8v4l3 3"/></>}
                        {mode === 'agent' && <><path d="M12 5a3 3 0 1 0-5.997.125 4 4 0 0 0-2.526 5.77 4 4 0 0 0 .556 6.588A4 4 0 1 0 12 18Z"/><path d="M12 5a3 3 0 1 1 5.997.125 4 4 0 0 1 2.526 5.77 4 4 0 0 1-.556 6.588A4 4 0 1 1 12 18Z"/></>}
                        {mode === 'team' && <><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></>}
                      </svg>
                      {mode === 'auto' ? 'Auto' : mode === 'agent' ? 'Agent' : 'Team'}
                    </button>
                  ))}
                </div>
              </div>

              {/* Agent/Team Selection */}
              {config.mode !== 'auto' && (
                <div class="playground-field">
                  <label class="playground-label">
                    {config.mode === 'agent' ? 'Select Agent' : 'Select Team'}
                  </label>
                  <select
                    class="playground-select"
                    value={config.entityId || ''}
                    onChange={(e) => setConfig({ ...config, entityId: (e.target as HTMLSelectElement).value || undefined })}
                    disabled={executing}
                  >
                    <option value="">Choose...</option>
                    {config.mode === 'agent'
                      ? agents.map((a) => (
                          <option key={a.id} value={a.id}>{a.name}</option>
                        ))
                      : teams.map((t) => (
                          <option key={t.id} value={t.id}>{t.name}</option>
                        ))}
                  </select>
                </div>
              )}

              {/* Location Toggle */}
              <div class="playground-field">
                <label class="playground-label">Location</label>
                <div class="playground-button-group">
                  <button
                    class={`playground-mode-btn ${config.local ? 'active' : ''}`}
                    onClick={() => setConfig({ ...config, local: true })}
                    disabled={executing}
                  >
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/>
                    </svg>
                    Local
                  </button>
                  <button
                    class={`playground-mode-btn ${!config.local ? 'active' : ''}`}
                    onClick={() => setConfig({ ...config, local: false })}
                    disabled={executing}
                  >
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <path d="M18 10h-1.26A8 8 0 1 0 9 20h9a5 5 0 0 0 0-10z"/>
                    </svg>
                    Remote
                  </button>
                </div>
                <div class="playground-field-hint">
                  {config.local ? 'Ephemeral local worker' : 'Control plane queue'}
                </div>
              </div>

              {/* Environment (for local mode) */}
              {config.local && environments.length > 0 && (
                <div class="playground-field">
                  <label class="playground-label">Environment</label>
                  <select
                    class="playground-select"
                    value={config.environment || ''}
                    onChange={(e) => setConfig({ ...config, environment: (e.target as HTMLSelectElement).value || undefined })}
                    disabled={executing}
                  >
                    <option value="">Auto-detect</option>
                    {environments.map((env) => (
                      <option key={env.id} value={env.id}>{env.name}</option>
                    ))}
                  </select>
                </div>
              )}

              {/* Working Directory */}
              <div class="playground-field">
                <label class="playground-label">Working Directory</label>
                <input
                  type="text"
                  class="playground-input"
                  placeholder="/path/to/dir (optional)"
                  value={config.workingDir}
                  onInput={(e) => setConfig({ ...config, workingDir: (e.target as HTMLInputElement).value })}
                  disabled={executing}
                />
              </div>

              {/* Advanced */}
              <details class="playground-advanced">
                <summary class="playground-advanced-summary">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                    <polyline points="6 9 12 15 18 9"/>
                  </svg>
                  Advanced Options
                </summary>
                <div class="playground-advanced-content">
                  {/* Output Format */}
                  <div class="playground-field" style={{ marginBottom: '0.75rem' }}>
                    <label class="playground-label" style={{ fontSize: '0.75rem' }}>Output Format</label>
                    <div class="playground-button-group">
                      <button
                        class={`playground-mode-btn ${config.streamFormat === 'text' ? 'active' : ''}`}
                        onClick={() => setConfig({ ...config, streamFormat: 'text' })}
                        disabled={executing}
                        style={{ fontSize: '0.6875rem', padding: '0.25rem 0.5rem' }}
                      >
                        Text
                      </button>
                      <button
                        class={`playground-mode-btn ${config.streamFormat === 'json' ? 'active' : ''}`}
                        onClick={() => setConfig({ ...config, streamFormat: 'json' })}
                        disabled={executing}
                        style={{ fontSize: '0.6875rem', padding: '0.25rem 0.5rem' }}
                      >
                        JSON
                      </button>
                    </div>
                    <div class="playground-field-hint" style={{ fontSize: '0.6875rem', marginTop: '0.25rem' }}>
                      {config.streamFormat === 'json' ? 'Structured events with tool calls (recommended)' : 'Plain text output'}
                    </div>
                  </div>
                  <label class="playground-checkbox">
                    <input
                      type="checkbox"
                      checked={config.directApi}
                      onChange={(e) => setConfig({ ...config, directApi: (e.target as HTMLInputElement).checked, mode: (e.target as HTMLInputElement).checked ? 'agent' : config.mode })}
                      disabled={executing}
                    />
                    <span class="playground-checkbox-label">Direct API (reliable streaming)</span>
                  </label>
                  {config.directApi && (
                    <div class="playground-checkbox-hint">
                      Uses control plane API directly. Requires selecting an agent.
                    </div>
                  )}
                  <label class="playground-checkbox">
                    <input
                      type="checkbox"
                      checked={config.verbose}
                      onChange={(e) => setConfig({ ...config, verbose: (e.target as HTMLInputElement).checked })}
                      disabled={executing}
                    />
                    <span class="playground-checkbox-label">Verbose output</span>
                  </label>
                </div>
              </details>

              {/* Error Display */}
              {error && (
                <div class="playground-error">
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                    <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
                  </svg>
                  {error}
                </div>
              )}
            </div>

            {/* Execute Button */}
            <div class="playground-config-footer">
              <button
                class="btn btn-primary playground-execute-btn"
                onClick={startExecution}
                disabled={!config.prompt.trim() || executing}
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <polygon points="5 3 19 12 5 21 5 3"/>
                </svg>
                Execute
              </button>
            </div>
          </div>
        )}

        {/* Output Panel */}
        <div class="card playground-output-panel">
          {/* Output Header */}
          <div class="playground-output-header">
            <div class="playground-output-title">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>
              </svg>
              Output
            </div>
            {executionId && (
              <code class="playground-exec-id">{executionId}</code>
            )}
            <div style={{ flex: 1 }} />
            {executing && (
              <button class="btn btn-danger playground-stop-btn" onClick={stopExecution}>
                <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor">
                  <rect x="4" y="4" width="16" height="16" rx="2"/>
                </svg>
                Stop
              </button>
            )}
            {!showConfig && !executing && (
              <button class="btn btn-secondary" onClick={() => setShowConfig(true)}>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <circle cx="12" cy="12" r="3"/>
                </svg>
                Config
              </button>
            )}
          </div>

          {/* Stream View */}
          <div class="playground-output-content">
            {events.length === 0 && !executing ? (
              <div class="playground-empty-state">
                <div class="playground-empty-icon">
                  <KubiyaLogo size={64} />
                </div>
                <h3 class="playground-empty-title">Ready to execute</h3>
                <p class="playground-empty-text">Configure your execution and press Execute to run</p>
              </div>
            ) : (
              <StreamView
                events={events}
                isRunning={executing}
                onClear={!executing ? clearEvents : undefined}
              />
            )}
          </div>
        </div>
      </div>

      {/* Quick Stats Bar */}
      <div class="playground-stats">
        <div class="playground-stat-card">
          <div class="playground-stat-icon mode">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              {config.mode === 'auto' && <><circle cx="12" cy="12" r="10"/><path d="M12 8v4l3 3"/></>}
              {config.mode === 'agent' && <><path d="M12 5a3 3 0 1 0-5.997.125 4 4 0 0 0-2.526 5.77 4 4 0 0 0 .556 6.588A4 4 0 1 0 12 18Z"/></>}
              {config.mode === 'team' && <><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/></>}
            </svg>
          </div>
          <div class="playground-stat-info">
            <span class="playground-stat-label">Mode</span>
            <span class="playground-stat-value">{config.mode}</span>
          </div>
        </div>
        <div class="playground-stat-card">
          <div class={`playground-stat-icon ${config.local ? 'local' : 'remote'}`}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              {config.local
                ? <><rect x="2" y="3" width="20" height="14" rx="2"/><line x1="12" y1="17" x2="12" y2="21"/></>
                : <path d="M18 10h-1.26A8 8 0 1 0 9 20h9a5 5 0 0 0 0-10z"/>
              }
            </svg>
          </div>
          <div class="playground-stat-info">
            <span class="playground-stat-label">Location</span>
            <span class="playground-stat-value">{config.local ? 'Local' : 'Remote'}</span>
          </div>
        </div>
        <div class="playground-stat-card">
          <div class="playground-stat-icon events">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M3 3v18h18"/><path d="M18 17V9"/><path d="M13 17V5"/><path d="M8 17v-3"/>
            </svg>
          </div>
          <div class="playground-stat-info">
            <span class="playground-stat-label">Events</span>
            <span class="playground-stat-value">{events.length}</span>
          </div>
        </div>
        <div class="playground-stat-card">
          <div class={`playground-stat-icon status ${executing ? 'running' : events.length > 0 ? 'complete' : 'ready'}`}>
            {executing ? (
              <span class="spinner" style={{ width: 14, height: 14 }} />
            ) : (
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                {events.length > 0
                  ? <><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></>
                  : <><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></>
                }
              </svg>
            )}
          </div>
          <div class="playground-stat-info">
            <span class="playground-stat-label">Status</span>
            <span class="playground-stat-value">{executing ? 'Running' : events.length > 0 ? 'Complete' : 'Ready'}</span>
          </div>
        </div>
      </div>
    </div>
  );
}
