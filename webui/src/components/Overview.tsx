import { h, Fragment } from 'preact';
import type { AppState, RecentActivity } from '../App';
import { Skeleton } from './Skeleton';

interface OverviewProps {
  state: AppState;
  isLoading?: boolean;
}

export function Overview({ state, isLoading }: OverviewProps) {
  const { overview, activity, controlPlane } = state;

  // Show skeleton loading state
  if (isLoading || (!overview && !controlPlane)) {
    return <OverviewSkeleton />;
  }

  return (
    <Fragment>
      {/* Stats Grid */}
      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-label">Active Workers</div>
          <div class="stat-value success">
            {overview?.active_workers ?? 0}
          </div>
        </div>

        <div class="stat-card">
          <div class="stat-label">Tasks Processed</div>
          <div class="stat-value">{overview?.tasks_processed ?? 0}</div>
        </div>

        <div class="stat-card">
          <div class="stat-label">Tasks Active</div>
          <div class="stat-value">{overview?.tasks_active ?? 0}</div>
        </div>

        <div class="stat-card">
          <div class="stat-label">Error Rate</div>
          <div class={`stat-value ${(overview?.error_rate ?? 0) > 5 ? 'error' : ''}`}>
            {(overview?.error_rate ?? 0).toFixed(1)}%
          </div>
        </div>

        <div class="stat-card">
          <div class="stat-label">Uptime</div>
          <div class="stat-value">{overview?.uptime_formatted ?? '0s'}</div>
        </div>

        <div class="stat-card">
          <div class="stat-label">Control Plane</div>
          <div class={`stat-value ${controlPlane?.connected ? 'success' : 'error'}`}>
            {controlPlane?.connected ? 'Connected' : 'Disconnected'}
          </div>
        </div>
      </div>

      {/* Quick Status */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginBottom: '1.5rem' }}>
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">Connection Status</h3>
          </div>
          <div style={{ display: 'flex', gap: '2rem' }}>
            <div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginBottom: '0.25rem' }}>
                Control Plane
              </div>
              <span class={`badge ${controlPlane?.connected ? 'badge-success' : 'badge-error'}`}>
                {controlPlane?.connected ? 'Connected' : 'Disconnected'}
              </span>
              {controlPlane?.latency_ms && (
                <span style={{ marginLeft: '0.5rem', fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                  {controlPlane.latency_ms}ms
                </span>
              )}
            </div>
            <div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginBottom: '0.25rem' }}>
                Auth Status
              </div>
              <span class={`badge ${controlPlane?.auth_status === 'valid' ? 'badge-success' : 'badge-warning'}`}>
                {controlPlane?.auth_status ?? 'Unknown'}
              </span>
            </div>
            {overview?.litellm_proxy_ok !== undefined && (
              <div>
                <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginBottom: '0.25rem' }}>
                  LiteLLM Proxy
                </div>
                <span class={`badge ${overview.litellm_proxy_ok ? 'badge-success' : 'badge-error'}`}>
                  {overview.litellm_proxy_ok ? 'Running' : 'Stopped'}
                </span>
              </div>
            )}
          </div>
        </div>

        <div class="card">
          <div class="card-header">
            <h3 class="card-title">Workers Summary</h3>
          </div>
          <div style={{ display: 'flex', gap: '2rem' }}>
            <div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginBottom: '0.25rem' }}>
                Total
              </div>
              <div style={{ fontSize: '1.25rem', fontWeight: 600 }}>
                {overview?.total_workers ?? 0}
              </div>
            </div>
            <div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginBottom: '0.25rem' }}>
                Active
              </div>
              <div style={{ fontSize: '1.25rem', fontWeight: 600, color: 'var(--success)' }}>
                {overview?.active_workers ?? 0}
              </div>
            </div>
            <div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginBottom: '0.25rem' }}>
                Idle
              </div>
              <div style={{ fontSize: '1.25rem', fontWeight: 600, color: 'var(--text-muted)' }}>
                {overview?.idle_workers ?? 0}
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Recent Activity */}
      <div class="card">
        <div class="card-header">
          <h3 class="card-title">Recent Activity</h3>
        </div>
        <div class="activity-feed">
          {activity.length === 0 ? (
            <div class="empty-state">
              <div style={{ color: 'var(--text-muted)' }}>No recent activity</div>
            </div>
          ) : (
            activity.slice(0, 10).map((item, idx) => (
              <ActivityItem key={idx} activity={item} />
            ))
          )}
        </div>
      </div>
    </Fragment>
  );
}

function ActivityItem({ activity }: { activity: RecentActivity }) {
  const getIconClass = () => {
    switch (activity.type) {
      case 'task_completed':
      case 'worker_started':
        return 'success';
      case 'task_failed':
        return 'error';
      default:
        return 'info';
    }
  };

  const getIcon = () => {
    switch (activity.type) {
      case 'task_completed':
        return '✓';
      case 'task_failed':
        return '✗';
      case 'worker_started':
        return '▶';
      case 'session_started':
        return '💬';
      default:
        return '•';
    }
  };

  const formatTime = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString();
  };

  return (
    <div class="activity-item">
      <div class={`activity-icon ${getIconClass()}`}>{getIcon()}</div>
      <div class="activity-content">
        <div class="activity-text">{activity.description}</div>
        <div class="activity-time">{formatTime(activity.timestamp)}</div>
      </div>
    </div>
  );
}

/**
 * Skeleton loader for the Overview page
 */
function OverviewSkeleton() {
  return (
    <Fragment>
      {/* Stats Grid skeleton */}
      <div class="stats-grid">
        {[1, 2, 3, 4, 5, 6].map((i) => (
          <div key={i} class="stat-card">
            <Skeleton variant="text" width="60%" height="0.75rem" />
            <Skeleton variant="text" width="40%" height="2rem" />
          </div>
        ))}
      </div>

      {/* Quick Status skeleton */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginBottom: '1.5rem' }}>
        <div class="card">
          <div class="card-header">
            <Skeleton variant="text" width={140} height="1rem" />
          </div>
          <div style={{ display: 'flex', gap: '2rem' }}>
            {[1, 2, 3].map((i) => (
              <div key={i}>
                <Skeleton variant="text" width={80} height="0.75rem" />
                <Skeleton variant="rectangular" width={70} height={22} />
              </div>
            ))}
          </div>
        </div>

        <div class="card">
          <div class="card-header">
            <Skeleton variant="text" width={140} height="1rem" />
          </div>
          <div style={{ display: 'flex', gap: '2rem' }}>
            {[1, 2, 3].map((i) => (
              <div key={i}>
                <Skeleton variant="text" width={50} height="0.75rem" />
                <Skeleton variant="text" width={30} height="1.5rem" />
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Recent Activity skeleton */}
      <div class="card">
        <div class="card-header">
          <Skeleton variant="text" width={130} height="1rem" />
        </div>
        <div class="activity-feed">
          {[1, 2, 3, 4, 5].map((i) => (
            <div key={i} class="activity-item">
              <Skeleton variant="circular" width={28} height={28} />
              <div class="activity-content" style={{ flex: 1 }}>
                <Skeleton variant="text" width="70%" height="0.875rem" />
                <Skeleton variant="text" width={60} height="0.75rem" />
              </div>
            </div>
          ))}
        </div>
      </div>
    </Fragment>
  );
}
