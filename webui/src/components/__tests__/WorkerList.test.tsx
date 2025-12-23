import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '../../test/test-utils';
import { WorkerList } from '../WorkerList';
import type { Worker } from '../../App';

const mockWorker = (id: string, overrides: Partial<Worker> = {}): Worker => ({
  id,
  queue_id: 'default-queue',
  status: 'running',
  pid: 12345,
  started_at: new Date().toISOString(),
  last_heartbeat: new Date().toISOString(),
  tasks_active: 2,
  tasks_total: 50,
  version: '1.0.0',
  hostname: 'localhost',
  ...overrides,
});

describe('WorkerList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders empty state when no workers', () => {
    render(<WorkerList workers={[]} />);
    expect(screen.getByText('No workers connected')).toBeInTheDocument();
  });

  it('renders worker count in heading', () => {
    const workers = [mockWorker('worker-1'), mockWorker('worker-2')];
    render(<WorkerList workers={workers} />);
    expect(screen.getByText('Workers (2)')).toBeInTheDocument();
  });

  it('displays worker ID', () => {
    const workers = [mockWorker('my-worker-id')];
    render(<WorkerList workers={workers} />);
    expect(screen.getByText('my-worker-id')).toBeInTheDocument();
  });

  it('displays worker status badge for running', () => {
    const workers = [mockWorker('worker-1', { status: 'running' })];
    const { container } = render(<WorkerList workers={workers} />);
    const badge = container.querySelector('.badge-success');
    expect(badge).toBeInTheDocument();
    expect(badge?.textContent).toBe('Running');
  });

  it('displays worker status badge for busy', () => {
    const workers = [mockWorker('worker-1', { status: 'busy' })];
    const { container } = render(<WorkerList workers={workers} />);
    const badge = container.querySelector('.badge-warning');
    expect(badge).toBeInTheDocument();
    expect(badge?.textContent).toBe('Busy');
  });

  it('displays worker status badge for idle', () => {
    const workers = [mockWorker('worker-1', { status: 'idle' })];
    const { container } = render(<WorkerList workers={workers} />);
    const badge = container.querySelector('.badge-info');
    expect(badge).toBeInTheDocument();
    expect(badge?.textContent).toBe('Idle');
  });

  it('displays worker status badge for error', () => {
    const workers = [mockWorker('worker-1', { status: 'error' })];
    const { container } = render(<WorkerList workers={workers} />);
    const badge = container.querySelector('.badge-error');
    expect(badge).toBeInTheDocument();
    expect(badge?.textContent).toBe('Error');
  });

  it('displays worker metadata (PID, hostname)', () => {
    const workers = [mockWorker('worker-1', { pid: 54321, hostname: 'server-01' })];
    render(<WorkerList workers={workers} />);
    expect(screen.getByText(/PID: 54321/)).toBeInTheDocument();
    expect(screen.getByText(/Host: server-01/)).toBeInTheDocument();
  });

  it('displays task counts', () => {
    const workers = [mockWorker('worker-1', { tasks_active: 5, tasks_total: 100 })];
    render(<WorkerList workers={workers} />);
    expect(screen.getByText('5')).toBeInTheDocument();
    expect(screen.getByText('100')).toBeInTheDocument();
    expect(screen.getByText('Active')).toBeInTheDocument();
    expect(screen.getByText('Total')).toBeInTheDocument();
  });

  it('displays worker metrics when available', () => {
    const workers = [
      mockWorker('worker-1', {
        metrics: {
          cpu_percent: 45.5,
          memory_mb: 256.7,
          memory_rss: 268435456,
          memory_percent: 12.5,
          open_files: 50,
          threads: 10,
          collected_at: new Date().toISOString(),
        },
      }),
    ];
    render(<WorkerList workers={workers} />);
    expect(screen.getByText('45.5%')).toBeInTheDocument();
    expect(screen.getByText('257 MB')).toBeInTheDocument();
    expect(screen.getByText('CPU')).toBeInTheDocument();
    expect(screen.getByText('Memory')).toBeInTheDocument();
  });

  it('does not display metrics section when no metrics', () => {
    const workers = [mockWorker('worker-1')];
    render(<WorkerList workers={workers} />);
    expect(screen.queryByText('CPU')).not.toBeInTheDocument();
  });

  it('renders multiple workers', () => {
    const workers = [
      mockWorker('worker-1'),
      mockWorker('worker-2'),
      mockWorker('worker-3'),
    ];
    render(<WorkerList workers={workers} />);
    expect(screen.getByText('worker-1')).toBeInTheDocument();
    expect(screen.getByText('worker-2')).toBeInTheDocument();
    expect(screen.getByText('worker-3')).toBeInTheDocument();
  });

  it('has restart button for each worker', () => {
    const workers = [mockWorker('worker-1')];
    render(<WorkerList workers={workers} />);
    expect(screen.getByText('↻ Restart')).toBeInTheDocument();
  });

  it('shows confirm dialog when restart is clicked', async () => {
    const workers = [mockWorker('worker-1')];
    render(<WorkerList workers={workers} />);

    const restartBtn = screen.getByText('↻ Restart');
    restartBtn.click();

    // Should show ConfirmDialog with proper content
    await waitFor(() => {
      expect(screen.getByText('Restart Worker?')).toBeInTheDocument();
    });

    // Check dialog content
    expect(screen.getByText(/Are you sure you want to restart worker/)).toBeInTheDocument();
  });

  it('calls restart API when confirmed', async () => {
    vi.mocked(global.fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ success: true }),
    } as Response);

    const workers = [mockWorker('worker-1')];
    render(<WorkerList workers={workers} />);

    // Click restart button to open dialog
    const restartBtn = screen.getByText('↻ Restart');
    restartBtn.click();

    // Wait for dialog to appear
    await waitFor(() => {
      expect(screen.getByText('Restart Worker?')).toBeInTheDocument();
    });

    // Find and click the confirm button in the dialog
    const dialogButtons = screen.getAllByRole('button');
    const confirmBtn = dialogButtons.find(btn => btn.textContent === 'Restart');
    expect(confirmBtn).toBeTruthy();
    confirmBtn!.click();

    // Wait for the fetch to be called
    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith('/api/workers/worker-1/restart', {
        method: 'POST',
      });
    });
  });

  it('does not call restart API when cancelled', async () => {
    const workers = [mockWorker('worker-1')];
    render(<WorkerList workers={workers} />);

    // Click restart button to open dialog
    const restartBtn = screen.getByText('↻ Restart');
    restartBtn.click();

    // Wait for dialog to appear
    await waitFor(() => {
      expect(screen.getByText('Restart Worker?')).toBeInTheDocument();
    });

    // Click cancel button
    const cancelBtn = screen.getByText('Cancel');
    cancelBtn.click();

    // Dialog should close
    await waitFor(() => {
      expect(screen.queryByText('Restart Worker?')).not.toBeInTheDocument();
    });
    expect(global.fetch).not.toHaveBeenCalled();
  });

  it('handles different status casing', () => {
    const workers = [mockWorker('worker-1', { status: 'RUNNING' })];
    const { container } = render(<WorkerList workers={workers} />);
    const badge = container.querySelector('.badge-success');
    expect(badge).toBeInTheDocument();
    expect(badge?.textContent).toBe('Running');
  });

  it('shows default badge for unknown status', () => {
    const workers = [mockWorker('worker-1', { status: 'unknown-status' })];
    const { container } = render(<WorkerList workers={workers} />);
    const badge = container.querySelector('.badge');
    expect(badge?.textContent).toBe('unknown-status');
  });
});
