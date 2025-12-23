import { h, Fragment } from 'preact';
import { useState, useCallback, memo } from 'preact/compat';
import type { Worker } from '../App';
import { Skeleton } from './Skeleton';
import { ConfirmDialog } from './ConfirmDialog';
import { useToastActions } from '../state';

interface WorkerListProps {
  workers: Worker[];
  isLoading?: boolean;
}

interface WorkerCardProps {
  worker: Worker;
  isRestarting: boolean;
  onRestart: (id: string) => void;
}

// Memoized worker card to prevent re-renders when other workers change
const WorkerCard = memo(function WorkerCard({ worker, isRestarting, onRestart }: WorkerCardProps) {
  const getStatusBadge = (status: string) => {
    switch (status.toLowerCase()) {
      case 'running':
        return <span class="badge badge-success">Running</span>;
      case 'busy':
        return <span class="badge badge-warning">Busy</span>;
      case 'idle':
        return <span class="badge badge-info">Idle</span>;
      case 'error':
        return <span class="badge badge-error">Error</span>;
      case 'stopping':
        return <span class="badge badge-warning">Stopping</span>;
      default:
        return <span class="badge">{status}</span>;
    }
  };

  const formatTime = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleString();
  };

  return (
    <div class="worker-card">
      <div class="worker-info">
        <div class="worker-icon">🤖</div>
        <div class="worker-details">
          <h3>
            {worker.id}
            <span style={{ marginLeft: '0.5rem' }}>
              {getStatusBadge(worker.status)}
            </span>
          </h3>
          <div class="worker-meta">
            PID: {worker.pid} • Host: {worker.hostname} • Started:{' '}
            {formatTime(worker.started_at)}
          </div>
        </div>
      </div>

      <div class="worker-metrics">
        {worker.metrics && (
          <Fragment>
            <div class="metric">
              <div class="metric-value">
                {worker.metrics.cpu_percent.toFixed(1)}%
              </div>
              <div class="metric-label">CPU</div>
            </div>
            <div class="metric">
              <div class="metric-value">
                {worker.metrics.memory_mb.toFixed(0)} MB
              </div>
              <div class="metric-label">Memory</div>
            </div>
          </Fragment>
        )}
        <div class="metric">
          <div class="metric-value">{worker.tasks_active}</div>
          <div class="metric-label">Active</div>
        </div>
        <div class="metric">
          <div class="metric-value">{worker.tasks_total}</div>
          <div class="metric-label">Total</div>
        </div>
        <button
          class="btn btn-secondary"
          onClick={() => onRestart(worker.id)}
          disabled={isRestarting}
        >
          {isRestarting ? (
            <span class="spinner" style={{ width: 14, height: 14 }} />
          ) : (
            '↻ Restart'
          )}
        </button>
      </div>
    </div>
  );
});

// Skeleton loader for worker cards
function WorkerCardSkeleton() {
  return (
    <div class="worker-card">
      <div class="worker-info">
        <Skeleton variant="circular" width={40} height={40} />
        <div class="worker-details" style={{ flex: 1 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '0.25rem' }}>
            <Skeleton variant="text" width={120} height="1.125rem" />
            <Skeleton variant="rectangular" width={60} height={20} />
          </div>
          <Skeleton variant="text" width="80%" height="0.75rem" />
        </div>
      </div>
      <div class="worker-metrics">
        <div class="metric">
          <Skeleton variant="text" width={40} height="1.25rem" />
          <Skeleton variant="text" width={30} height="0.75rem" />
        </div>
        <div class="metric">
          <Skeleton variant="text" width={50} height="1.25rem" />
          <Skeleton variant="text" width={45} height="0.75rem" />
        </div>
        <div class="metric">
          <Skeleton variant="text" width={30} height="1.25rem" />
          <Skeleton variant="text" width={35} height="0.75rem" />
        </div>
        <div class="metric">
          <Skeleton variant="text" width={30} height="1.25rem" />
          <Skeleton variant="text" width={30} height="0.75rem" />
        </div>
        <Skeleton variant="rectangular" width={80} height={32} />
      </div>
    </div>
  );
}

export function WorkerList({ workers, isLoading }: WorkerListProps) {
  const [restartingId, setRestartingId] = useState<string | null>(null);
  const [confirmDialog, setConfirmDialog] = useState<{ workerId: string } | null>(null);
  const toast = useToastActions();

  const handleRestartClick = useCallback((workerId: string) => {
    setConfirmDialog({ workerId });
  }, []);

  const handleConfirmRestart = useCallback(async () => {
    if (!confirmDialog) return;

    const workerId = confirmDialog.workerId;
    setConfirmDialog(null);
    setRestartingId(workerId);

    try {
      const res = await fetch(`/api/workers/${workerId}/restart`, {
        method: 'POST',
      });
      const data = await res.json();
      if (data.success) {
        toast.success('Worker restarting', `Worker ${workerId} is being restarted`);
      } else {
        toast.error('Restart failed', data.error || 'Unknown error');
      }
    } catch (err) {
      toast.error('Restart failed', String(err));
    } finally {
      setRestartingId(null);
    }
  }, [confirmDialog, toast]);

  const handleCancelRestart = useCallback(() => {
    setConfirmDialog(null);
  }, []);

  // Loading state
  if (isLoading) {
    return (
      <Fragment>
        <div style={{ marginBottom: '1rem' }}>
          <Skeleton variant="text" width={150} height="1.5rem" />
        </div>
        <div class="worker-list">
          {[1, 2, 3].map((i) => (
            <WorkerCardSkeleton key={i} />
          ))}
        </div>
      </Fragment>
    );
  }

  if (workers.length === 0) {
    return (
      <div class="card">
        <div class="empty-state">
          <div class="empty-state-icon">🔧</div>
          <div>No workers connected</div>
        </div>
      </div>
    );
  }

  return (
    <Fragment>
      <ConfirmDialog
        isOpen={!!confirmDialog}
        title="Restart Worker?"
        message={`Are you sure you want to restart worker "${confirmDialog?.workerId}"? This will interrupt any active tasks.`}
        confirmLabel="Restart"
        cancelLabel="Cancel"
        variant="warning"
        onConfirm={handleConfirmRestart}
        onCancel={handleCancelRestart}
      />

      <div style={{ marginBottom: '1rem' }}>
        <h2 style={{ fontSize: '1.25rem', fontWeight: 600 }}>
          Workers ({workers.length})
        </h2>
      </div>

      <div class="worker-list">
        {workers.map((worker) => (
          <WorkerCard
            key={worker.id}
            worker={worker}
            isRestarting={restartingId === worker.id}
            onRestart={handleRestartClick}
          />
        ))}
      </div>
    </Fragment>
  );
}
