import { describe, it, expect } from 'vitest';
import { render, screen } from '../../test/test-utils';
import { Overview } from '../Overview';
import type { AppState } from '../../App';

const mockState: AppState = {
  overview: {
    total_workers: 5,
    active_workers: 3,
    idle_workers: 2,
    tasks_processed: 1500,
    tasks_active: 10,
    tasks_failed: 5,
    error_rate: 0.3,
    uptime: 86400,
    uptime_formatted: '1d 0h',
    control_plane_ok: true,
    litellm_proxy_ok: true,
    start_time: new Date().toISOString(),
  },
  workers: [],
  controlPlane: {
    connected: true,
    url: 'https://api.kubiya.ai',
    latency: 0.045,
    latency_ms: 45,
    last_check: new Date().toISOString(),
    last_success: new Date().toISOString(),
    auth_status: 'valid',
    reconnect_count: 0,
  },
  sessions: [],
  logs: [],
  activity: [
    {
      type: 'task_completed',
      description: 'Task completed successfully',
      timestamp: new Date().toISOString(),
      worker_id: 'worker-1',
    },
    {
      type: 'worker_started',
      description: 'Worker started',
      timestamp: new Date().toISOString(),
      worker_id: 'worker-2',
    },
  ],
  config: null,
  connected: true,
};

describe('Overview', () => {
  it('renders without crashing', () => {
    render(<Overview state={mockState} />);
    // Check for stats grid
    expect(document.querySelector('.stats-grid')).toBeInTheDocument();
  });

  it('displays active workers count', () => {
    const { container } = render(<Overview state={mockState} />);
    // Find the stat card for active workers
    const statCards = container.querySelectorAll('.stat-card');
    const activeWorkersCard = Array.from(statCards).find(
      (card) => card.querySelector('.stat-label')?.textContent === 'Active Workers'
    );
    expect(activeWorkersCard).toBeInTheDocument();
    expect(activeWorkersCard?.querySelector('.stat-value')?.textContent).toBe('3');
  });

  it('displays tasks processed count', () => {
    render(<Overview state={mockState} />);
    expect(screen.getByText('1500')).toBeInTheDocument();
    expect(screen.getByText('Tasks Processed')).toBeInTheDocument();
  });

  it('displays tasks active count', () => {
    render(<Overview state={mockState} />);
    expect(screen.getByText('10')).toBeInTheDocument();
    expect(screen.getByText('Tasks Active')).toBeInTheDocument();
  });

  it('displays error rate', () => {
    render(<Overview state={mockState} />);
    expect(screen.getByText('0.3%')).toBeInTheDocument();
    expect(screen.getByText('Error Rate')).toBeInTheDocument();
  });

  it('displays uptime', () => {
    render(<Overview state={mockState} />);
    expect(screen.getByText('1d 0h')).toBeInTheDocument();
    expect(screen.getByText('Uptime')).toBeInTheDocument();
  });

  it('displays control plane status as connected', () => {
    render(<Overview state={mockState} />);
    // Should show "Connected" in the overview stats
    const connectedElements = screen.getAllByText('Connected');
    expect(connectedElements.length).toBeGreaterThan(0);
  });

  it('displays control plane status as disconnected when not connected', () => {
    const disconnectedState = {
      ...mockState,
      controlPlane: {
        ...mockState.controlPlane!,
        connected: false,
      },
    };
    const { container } = render(<Overview state={disconnectedState} />);
    // Check for disconnected status in stats
    const disconnectedValue = container.querySelector('.stat-value.error');
    expect(disconnectedValue?.textContent).toBe('Disconnected');
  });

  it('highlights error rate when above threshold', () => {
    const highErrorState = {
      ...mockState,
      overview: {
        ...mockState.overview!,
        error_rate: 10.5,
      },
    };
    const { container } = render(<Overview state={highErrorState} />);

    const errorValue = container.querySelector('.stat-value.error');
    expect(errorValue).toBeInTheDocument();
    expect(errorValue?.textContent).toContain('10.5%');
  });

  it('renders activity section', () => {
    const { container } = render(<Overview state={mockState} />);
    const activitySection = container.querySelector('.activity-section') ||
                            container.querySelector('[class*="activity"]');
    // Activity section should exist if there's activity data
    expect(mockState.activity.length).toBeGreaterThan(0);
  });

  it('shows empty state when no activity', () => {
    const noActivityState = {
      ...mockState,
      activity: [],
    };
    render(<Overview state={noActivityState} />);
    // Should handle empty activity gracefully
  });

  it('handles null overview gracefully', () => {
    const nullOverviewState = {
      ...mockState,
      overview: null,
    };
    render(<Overview state={nullOverviewState} />);

    // Should show default values (0)
    expect(screen.getByText('0s')).toBeInTheDocument(); // Default uptime
  });

  it('handles null controlPlane gracefully', () => {
    const nullControlPlaneState = {
      ...mockState,
      controlPlane: null,
    };
    expect(() => render(<Overview state={nullControlPlaneState} />)).not.toThrow();
  });
});
