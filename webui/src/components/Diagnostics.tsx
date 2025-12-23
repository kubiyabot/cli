import { h, Fragment } from 'preact';
import { useState, useEffect, useRef } from 'preact/hooks';
import type { LogEntry, WorkerConfig } from '../App';

interface DiagnosticsProps {
  logs: LogEntry[];
  config: WorkerConfig | null;
}

export function Diagnostics({ logs, config }: DiagnosticsProps) {
  const [levelFilter, setLevelFilter] = useState<string>('');
  const [componentFilter, setComponentFilter] = useState<string>('');
  const [search, setSearch] = useState('');
  const [autoScroll, setAutoScroll] = useState(true);
  const [showTimestamp, setShowTimestamp] = useState(true);
  const logViewerRef = useRef<HTMLDivElement>(null);

  // Get unique components for filter dropdown
  const components = [...new Set(logs.map((log) => log.component).filter(Boolean))];

  // Auto-scroll to bottom when new logs arrive
  useEffect(() => {
    if (autoScroll && logViewerRef.current) {
      logViewerRef.current.scrollTop = logViewerRef.current.scrollHeight;
    }
  }, [logs, autoScroll]);

  const filteredLogs = logs.filter((log) => {
    if (levelFilter && log.level !== levelFilter) return false;
    if (componentFilter && log.component !== componentFilter) return false;
    if (search && !log.message.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });

  const formatTime = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString('en-US', { hour12: false });
  };

  const getComponentBadge = (component: string) => {
    if (component === 'worker-stdout') {
      return <span class="log-component stdout">stdout</span>;
    }
    if (component === 'worker-stderr') {
      return <span class="log-component stderr">stderr</span>;
    }
    if (component === 'worker') {
      return <span class="log-component worker">worker</span>;
    }
    if (component === 'metrics') {
      return <span class="log-component metrics">metrics</span>;
    }
    if (component === 'webui') {
      return <span class="log-component webui">webui</span>;
    }
    return <span class="log-component">{component || 'system'}</span>;
  };

  // Calculate log statistics
  const logStats = {
    total: logs.length,
    errors: logs.filter((l) => l.level === 'ERROR').length,
    warnings: logs.filter((l) => l.level === 'WARNING').length,
    info: logs.filter((l) => l.level === 'INFO').length,
  };

  return (
    <Fragment>
      {/* Log Statistics */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: '1rem', marginBottom: '1rem' }}>
        <div class="stat-card">
          <div class="stat-label">Total Logs</div>
          <div class="stat-value">{logStats.total}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Errors</div>
          <div class="stat-value error">{logStats.errors}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Warnings</div>
          <div class="stat-value warning">{logStats.warnings}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Info</div>
          <div class="stat-value">{logStats.info}</div>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: '1rem' }}>
        {/* Log Viewer */}
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">
              Live Logs
              <span style={{ marginLeft: '0.5rem', fontSize: '0.75rem', color: 'var(--success)' }}>
                ● streaming
              </span>
            </h3>
            <div style={{ display: 'flex', gap: '1rem', alignItems: 'center' }}>
              <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', display: 'flex', alignItems: 'center', gap: '0.25rem' }}>
                <input
                  type="checkbox"
                  checked={showTimestamp}
                  onChange={(e) => setShowTimestamp((e.target as HTMLInputElement).checked)}
                />
                Timestamps
              </label>
              <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', display: 'flex', alignItems: 'center', gap: '0.25rem' }}>
                <input
                  type="checkbox"
                  checked={autoScroll}
                  onChange={(e) => setAutoScroll((e.target as HTMLInputElement).checked)}
                />
                Auto-scroll
              </label>
            </div>
          </div>

          {/* Filters */}
          <div class="filter-bar">
            <select
              class="filter-select"
              value={levelFilter}
              onChange={(e) => setLevelFilter((e.target as HTMLSelectElement).value)}
            >
              <option value="">All Levels</option>
              <option value="DEBUG">DEBUG</option>
              <option value="INFO">INFO</option>
              <option value="WARNING">WARNING</option>
              <option value="ERROR">ERROR</option>
            </select>
            <select
              class="filter-select"
              value={componentFilter}
              onChange={(e) => setComponentFilter((e.target as HTMLSelectElement).value)}
            >
              <option value="">All Components</option>
              {components.map((comp) => (
                <option key={comp} value={comp}>{comp}</option>
              ))}
            </select>
            <input
              type="text"
              class="filter-input"
              placeholder="Search logs..."
              value={search}
              onInput={(e) => setSearch((e.target as HTMLInputElement).value)}
              style={{ flex: 1 }}
            />
            <button
              class="btn btn-secondary"
              onClick={() => {
                setLevelFilter('');
                setComponentFilter('');
                setSearch('');
              }}
              style={{ fontSize: '0.75rem', padding: '0.4rem 0.6rem' }}
            >
              Clear
            </button>
          </div>

          {/* Log entries */}
          <div class="log-viewer" ref={logViewerRef} style={{ maxHeight: '500px' }}>
            {filteredLogs.length === 0 ? (
              <div style={{ color: 'var(--text-muted)', textAlign: 'center', padding: '2rem' }}>
                <div style={{ fontSize: '2rem', marginBottom: '0.5rem' }}>📝</div>
                <div>No logs to display</div>
                <div style={{ fontSize: '0.75rem', marginTop: '0.5rem' }}>
                  Logs from the worker process will appear here in real-time
                </div>
              </div>
            ) : (
              filteredLogs.map((log, idx) => (
                <div key={idx} class={`log-entry ${log.level.toLowerCase()}`}>
                  {showTimestamp && <span class="log-time">{formatTime(log.timestamp)}</span>}
                  <span class={`log-level ${log.level.toLowerCase()}`}>{log.level}</span>
                  {getComponentBadge(log.component)}
                  <span class="log-message">{log.message}</span>
                </div>
              ))
            )}
          </div>

          {/* Footer with count */}
          <div style={{
            marginTop: '0.5rem',
            fontSize: '0.75rem',
            color: 'var(--text-muted)',
            display: 'flex',
            justifyContent: 'space-between'
          }}>
            <span>Showing {filteredLogs.length} of {logs.length} logs</span>
            <span>Max buffer: 1000 entries</span>
          </div>
        </div>

        {/* Config & Health */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
          {/* Configuration */}
          <div class="card">
            <div class="card-header">
              <h3 class="card-title">Configuration</h3>
            </div>
            {config ? (
              <table class="table">
                <tbody>
                  <tr>
                    <td style={{ color: 'var(--text-secondary)' }}>Queue ID</td>
                    <td style={{ wordBreak: 'break-all', fontSize: '0.8125rem' }}>{config.queue_id}</td>
                  </tr>
                  <tr>
                    <td style={{ color: 'var(--text-secondary)' }}>Deployment</td>
                    <td>{config.deployment_type}</td>
                  </tr>
                  <tr>
                    <td style={{ color: 'var(--text-secondary)' }}>Daemon Mode</td>
                    <td>{config.daemon_mode ? 'Yes' : 'No'}</td>
                  </tr>
                  <tr>
                    <td style={{ color: 'var(--text-secondary)' }}>Auto Update</td>
                    <td>{config.auto_update ? 'Enabled' : 'Disabled'}</td>
                  </tr>
                  <tr>
                    <td style={{ color: 'var(--text-secondary)' }}>Local Proxy</td>
                    <td>
                      {config.enable_local_proxy ? (
                        <span class="badge badge-success">Enabled</span>
                      ) : (
                        <span class="badge">Disabled</span>
                      )}
                    </td>
                  </tr>
                  {config.proxy_port && (
                    <tr>
                      <td style={{ color: 'var(--text-secondary)' }}>Proxy Port</td>
                      <td>{config.proxy_port}</td>
                    </tr>
                  )}
                  {config.model_override && (
                    <tr>
                      <td style={{ color: 'var(--text-secondary)' }}>Model Override</td>
                      <td>{config.model_override}</td>
                    </tr>
                  )}
                </tbody>
              </table>
            ) : (
              <div class="loading">
                <span class="spinner" />
              </div>
            )}
          </div>

          {/* Quick Actions */}
          <div class="card">
            <div class="card-header">
              <h3 class="card-title">Actions</h3>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
              <button
                class="btn btn-secondary"
                onClick={() => fetch('/api/control-plane/reconnect', { method: 'POST' })}
              >
                ↻ Reconnect Control Plane
              </button>
            </div>
          </div>

          {/* Environment Info */}
          <div class="card">
            <div class="card-header">
              <h3 class="card-title">Environment</h3>
            </div>
            <table class="table">
              <tbody>
                {config?.worker_dir && (
                  <tr>
                    <td style={{ color: 'var(--text-secondary)' }}>Worker Dir</td>
                    <td style={{ fontSize: '0.75rem', wordBreak: 'break-all' }}>{config.worker_dir}</td>
                  </tr>
                )}
                {config?.control_plane_url && (
                  <tr>
                    <td style={{ color: 'var(--text-secondary)' }}>Control Plane</td>
                    <td style={{ fontSize: '0.75rem', wordBreak: 'break-all' }}>{config.control_plane_url}</td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </Fragment>
  );
}
