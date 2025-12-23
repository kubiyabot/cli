import { h, Fragment } from 'preact';
import { useState } from 'preact/hooks';
import type { ControlPlaneStatus } from '../App';

interface ControlPlaneProps {
  status: ControlPlaneStatus | null;
}

export function ControlPlane({ status }: ControlPlaneProps) {
  const [reconnecting, setReconnecting] = useState(false);

  const handleReconnect = async () => {
    setReconnecting(true);
    try {
      const res = await fetch('/api/control-plane/reconnect', {
        method: 'POST',
      });
      const data = await res.json();
      if (!data.success) {
        alert('Reconnect failed: ' + (data.error || 'Unknown error'));
      }
    } catch (err) {
      alert('Reconnect failed: ' + err);
    } finally {
      setReconnecting(false);
    }
  };

  const formatTime = (timestamp: string) => {
    if (!timestamp) return 'Never';
    const date = new Date(timestamp);
    return date.toLocaleString();
  };

  if (!status) {
    return (
      <div class="card">
        <div class="loading">
          <span class="spinner" />
        </div>
      </div>
    );
  }

  return (
    <Fragment>
      <div style={{ marginBottom: '1rem', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2 style={{ fontSize: '1.25rem', fontWeight: 600 }}>Control Plane Connection</h2>
        <button
          class="btn btn-secondary"
          onClick={handleReconnect}
          disabled={reconnecting}
        >
          {reconnecting ? (
            <Fragment>
              <span class="spinner" style={{ width: 14, height: 14 }} />
              Reconnecting...
            </Fragment>
          ) : (
            '↻ Reconnect'
          )}
        </button>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
        {/* Connection Status */}
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">Connection Status</h3>
          </div>
          <table class="table">
            <tbody>
              <tr>
                <td style={{ color: 'var(--text-secondary)' }}>Status</td>
                <td>
                  <span class={`badge ${status.connected ? 'badge-success' : 'badge-error'}`}>
                    {status.connected ? 'Connected' : 'Disconnected'}
                  </span>
                </td>
              </tr>
              <tr>
                <td style={{ color: 'var(--text-secondary)' }}>URL</td>
                <td style={{ wordBreak: 'break-all' }}>{status.url}</td>
              </tr>
              <tr>
                <td style={{ color: 'var(--text-secondary)' }}>Latency</td>
                <td>{status.latency_ms ? `${status.latency_ms}ms` : 'N/A'}</td>
              </tr>
              <tr>
                <td style={{ color: 'var(--text-secondary)' }}>Last Check</td>
                <td>{formatTime(status.last_check)}</td>
              </tr>
              <tr>
                <td style={{ color: 'var(--text-secondary)' }}>Last Success</td>
                <td>{formatTime(status.last_success)}</td>
              </tr>
            </tbody>
          </table>
        </div>

        {/* Authentication */}
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">Authentication</h3>
          </div>
          <table class="table">
            <tbody>
              <tr>
                <td style={{ color: 'var(--text-secondary)' }}>Auth Status</td>
                <td>
                  <span
                    class={`badge ${
                      status.auth_status === 'valid'
                        ? 'badge-success'
                        : status.auth_status === 'expired'
                        ? 'badge-warning'
                        : 'badge-error'
                    }`}
                  >
                    {status.auth_status}
                  </span>
                </td>
              </tr>
              {status.config_version && (
                <tr>
                  <td style={{ color: 'var(--text-secondary)' }}>Config Version</td>
                  <td>{status.config_version}</td>
                </tr>
              )}
              <tr>
                <td style={{ color: 'var(--text-secondary)' }}>Reconnect Count</td>
                <td>{status.reconnect_count}</td>
              </tr>
              {status.error_message && (
                <tr>
                  <td style={{ color: 'var(--text-secondary)' }}>Error</td>
                  <td style={{ color: 'var(--error)' }}>{status.error_message}</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Error Alert */}
      {!status.connected && status.error_message && (
        <div
          class="card"
          style={{
            marginTop: '1rem',
            borderColor: 'var(--error)',
            background: 'rgba(255, 107, 107, 0.1)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
            <span style={{ fontSize: '1.5rem' }}>⚠️</span>
            <div>
              <div style={{ fontWeight: 600, marginBottom: '0.25rem' }}>
                Connection Error
              </div>
              <div style={{ color: 'var(--text-secondary)' }}>
                {status.error_message}
              </div>
            </div>
          </div>
        </div>
      )}
    </Fragment>
  );
}
