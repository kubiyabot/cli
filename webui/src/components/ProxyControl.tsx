import { h, Fragment } from 'preact';
import { useState, useEffect, useRef } from 'preact/hooks';

interface ProxyStatus {
  running: boolean;
  pid?: number;
  port?: number;
  base_url?: string;
  started_at?: string;
  health_status: string;
  config_path?: string;
  models?: string[];
  log_file?: string;
}

interface LangfuseConfig {
  enabled: boolean;
  public_key?: string;
  secret_key?: string;
  host?: string;
}

export function ProxyControl() {
  const [status, setStatus] = useState<ProxyStatus | null>(null);
  const [logs, setLogs] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [logSearch, setLogSearch] = useState('');
  const [logLevel, setLogLevel] = useState('');
  const [showLangfuse, setShowLangfuse] = useState(false);
  const [langfuseConfig, setLangfuseConfig] = useState<LangfuseConfig>({
    enabled: false,
    public_key: '',
    secret_key: '',
    host: 'https://cloud.langfuse.com',
  });
  const logViewerRef = useRef<HTMLDivElement>(null);

  const fetchStatus = async () => {
    try {
      const res = await fetch('/api/proxy/status');
      const data = await res.json();
      setStatus(data);
    } catch (err) {
      setError(`Failed to fetch proxy status: ${err}`);
    } finally {
      setLoading(false);
    }
  };

  const fetchLogs = async () => {
    try {
      const params = new URLSearchParams();
      params.set('lines', '200');
      if (logSearch) params.set('search', logSearch);
      if (logLevel) params.set('level', logLevel);

      const res = await fetch(`/api/proxy/logs?${params}`);
      const data = await res.json();
      setLogs(data.logs || []);
    } catch (err) {
      console.error('Failed to fetch proxy logs:', err);
    }
  };

  useEffect(() => {
    fetchStatus();
    fetchLogs();

    // Refresh status periodically
    const interval = setInterval(fetchStatus, 10000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    fetchLogs();
  }, [logSearch, logLevel]);

  // Auto-scroll logs
  useEffect(() => {
    if (logViewerRef.current) {
      logViewerRef.current.scrollTop = logViewerRef.current.scrollHeight;
    }
  }, [logs]);

  const handleProxyAction = async (action: string) => {
    setActionLoading(true);
    try {
      const res = await fetch('/api/proxy/control', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action }),
      });
      const data = await res.json();
      if (!data.success) {
        alert(data.message || data.error || 'Action failed');
      }
      // Refresh status
      fetchStatus();
    } catch (err) {
      alert(`Action failed: ${err}`);
    } finally {
      setActionLoading(false);
    }
  };

  const handleLangfuseSave = async () => {
    try {
      const res = await fetch('/api/proxy/langfuse', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(langfuseConfig),
      });
      const data = await res.json();
      if (!data.success) {
        alert(data.message || data.error || 'Failed to save Langfuse config');
      } else {
        alert('Langfuse configuration saved');
        setShowLangfuse(false);
      }
    } catch (err) {
      alert(`Failed to save: ${err}`);
    }
  };

  const getHealthColor = (health: string) => {
    switch (health) {
      case 'healthy':
        return 'var(--success)';
      case 'unhealthy':
        return 'var(--error)';
      case 'disabled':
        return 'var(--text-muted)';
      default:
        return 'var(--warning)';
    }
  };

  if (loading) {
    return (
      <div class="card">
        <div style={{ textAlign: 'center', padding: '3rem' }}>
          <span class="spinner" style={{ width: '32px', height: '32px' }} />
          <div style={{ marginTop: '1rem', color: 'var(--text-secondary)' }}>
            Loading proxy status...
          </div>
        </div>
      </div>
    );
  }

  return (
    <Fragment>
      {/* Header */}
      <div style={{ marginBottom: '1rem', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2 style={{ fontSize: '1.25rem', fontWeight: 600 }}>LiteLLM Proxy</h2>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <button
            class="btn btn-secondary"
            onClick={() => setShowLangfuse(!showLangfuse)}
          >
            🔧 Langfuse
          </button>
          <button
            class="btn btn-secondary"
            onClick={() => { fetchStatus(); fetchLogs(); }}
          >
            ↻ Refresh
          </button>
        </div>
      </div>

      {error && (
        <div class="card" style={{ borderColor: 'var(--error)', marginBottom: '1rem' }}>
          <div style={{ color: 'var(--error)' }}>{error}</div>
        </div>
      )}

      {/* Status Card */}
      <div
        class="card"
        style={{
          marginBottom: '1rem',
          borderColor: status?.running ? 'var(--success)' : 'var(--text-muted)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
            <div
              style={{
                width: '48px',
                height: '48px',
                borderRadius: '50%',
                background: status?.running ? 'var(--success)' : 'var(--bg-hover)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: '1.5rem',
              }}
            >
              {status?.running ? '🟢' : '⚪'}
            </div>
            <div>
              <div style={{ fontSize: '1.125rem', fontWeight: 600 }}>
                {status?.running ? 'Running' : 'Stopped'}
              </div>
              <div style={{ color: 'var(--text-secondary)', fontSize: '0.875rem' }}>
                Health: <span style={{ color: getHealthColor(status?.health_status || 'unknown') }}>
                  {status?.health_status || 'unknown'}
                </span>
              </div>
            </div>
          </div>

          <div style={{ display: 'flex', gap: '0.5rem' }}>
            {status?.running ? (
              <Fragment>
                <button
                  class="btn btn-secondary"
                  onClick={() => handleProxyAction('restart')}
                  disabled={actionLoading}
                >
                  ↻ Restart
                </button>
                <button
                  class="btn btn-danger"
                  onClick={() => handleProxyAction('stop')}
                  disabled={actionLoading}
                >
                  ■ Stop
                </button>
              </Fragment>
            ) : (
              <button
                class="btn btn-primary"
                onClick={() => handleProxyAction('start')}
                disabled={actionLoading}
              >
                ▶ Start
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Langfuse Config Modal */}
      {showLangfuse && (
        <div class="card" style={{ marginBottom: '1rem' }}>
          <div class="card-header">
            <h3 class="card-title">Langfuse Configuration</h3>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <input
                type="checkbox"
                checked={langfuseConfig.enabled}
                onChange={(e) =>
                  setLangfuseConfig({ ...langfuseConfig, enabled: (e.target as HTMLInputElement).checked })
                }
              />
              <span>Enable Langfuse</span>
            </label>
            {langfuseConfig.enabled && (
              <Fragment>
                <div>
                  <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>Public Key</label>
                  <input
                    type="text"
                    class="filter-input"
                    style={{ width: '100%' }}
                    value={langfuseConfig.public_key}
                    onInput={(e) =>
                      setLangfuseConfig({ ...langfuseConfig, public_key: (e.target as HTMLInputElement).value })
                    }
                    placeholder="pk-lf-..."
                  />
                </div>
                <div>
                  <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>Secret Key</label>
                  <input
                    type="password"
                    class="filter-input"
                    style={{ width: '100%' }}
                    value={langfuseConfig.secret_key}
                    onInput={(e) =>
                      setLangfuseConfig({ ...langfuseConfig, secret_key: (e.target as HTMLInputElement).value })
                    }
                    placeholder="sk-lf-..."
                  />
                </div>
                <div>
                  <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>Host</label>
                  <input
                    type="text"
                    class="filter-input"
                    style={{ width: '100%' }}
                    value={langfuseConfig.host}
                    onInput={(e) =>
                      setLangfuseConfig({ ...langfuseConfig, host: (e.target as HTMLInputElement).value })
                    }
                    placeholder="https://cloud.langfuse.com"
                  />
                </div>
              </Fragment>
            )}
            <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'flex-end' }}>
              <button class="btn btn-secondary" onClick={() => setShowLangfuse(false)}>
                Cancel
              </button>
              <button class="btn btn-primary" onClick={handleLangfuseSave}>
                Save
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Details Grid */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginBottom: '1rem' }}>
        {/* Connection Info */}
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">Connection</h3>
          </div>
          <table class="table">
            <tbody>
              <tr>
                <td style={{ color: 'var(--text-secondary)' }}>Base URL</td>
                <td style={{ fontFamily: 'monospace', fontSize: '0.8125rem' }}>
                  {status?.base_url || 'N/A'}
                </td>
              </tr>
              <tr>
                <td style={{ color: 'var(--text-secondary)' }}>Port</td>
                <td>{status?.port || 'N/A'}</td>
              </tr>
              <tr>
                <td style={{ color: 'var(--text-secondary)' }}>PID</td>
                <td>{status?.pid || 'N/A'}</td>
              </tr>
              {status?.config_path && (
                <tr>
                  <td style={{ color: 'var(--text-secondary)' }}>Config</td>
                  <td style={{ fontSize: '0.75rem', wordBreak: 'break-all' }}>
                    {status.config_path}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        {/* Models */}
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">Configured Models</h3>
          </div>
          {status?.models && status.models.length > 0 ? (
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem' }}>
              {status.models.map((model) => (
                <span
                  key={model}
                  class="badge badge-info"
                  style={{ fontSize: '0.75rem' }}
                >
                  {model}
                </span>
              ))}
            </div>
          ) : (
            <div style={{ color: 'var(--text-muted)', textAlign: 'center', padding: '1rem' }}>
              No models configured
            </div>
          )}
        </div>
      </div>

      {/* Logs */}
      <div class="card">
        <div class="card-header">
          <h3 class="card-title">
            Proxy Logs
            <span style={{ marginLeft: '0.5rem', fontSize: '0.75rem', color: 'var(--text-muted)' }}>
              ({logs.length} lines)
            </span>
          </h3>
        </div>

        {/* Log filters */}
        <div class="filter-bar" style={{ marginBottom: '0.75rem' }}>
          <input
            type="text"
            class="filter-input"
            placeholder="Search logs..."
            value={logSearch}
            onInput={(e) => setLogSearch((e.target as HTMLInputElement).value)}
            style={{ flex: 1 }}
          />
          <select
            class="filter-select"
            value={logLevel}
            onChange={(e) => setLogLevel((e.target as HTMLSelectElement).value)}
          >
            <option value="">All Levels</option>
            <option value="ERROR">ERROR</option>
            <option value="WARNING">WARNING</option>
            <option value="INFO">INFO</option>
          </select>
          <button
            class="btn btn-secondary"
            onClick={() => { setLogSearch(''); setLogLevel(''); }}
            style={{ fontSize: '0.75rem', padding: '0.4rem 0.6rem' }}
          >
            Clear
          </button>
        </div>

        {/* Log viewer */}
        <div
          class="log-viewer"
          ref={logViewerRef}
          style={{ maxHeight: '400px', fontSize: '0.75rem' }}
        >
          {logs.length === 0 ? (
            <div style={{ color: 'var(--text-muted)', textAlign: 'center', padding: '2rem' }}>
              <div style={{ fontSize: '2rem', marginBottom: '0.5rem' }}>📝</div>
              <div>No proxy logs available</div>
              <div style={{ fontSize: '0.75rem', marginTop: '0.5rem' }}>
                Logs will appear here when the proxy is running
              </div>
            </div>
          ) : (
            logs.map((line, idx) => {
              const isError = line.includes('ERROR');
              const isWarning = line.includes('WARNING') || line.includes('WARN');
              return (
                <div
                  key={idx}
                  class={`log-entry ${isError ? 'error' : isWarning ? 'warning' : ''}`}
                  style={{ padding: '0.25rem 0' }}
                >
                  <span class="log-message">{line}</span>
                </div>
              );
            })
          )}
        </div>
      </div>
    </Fragment>
  );
}
