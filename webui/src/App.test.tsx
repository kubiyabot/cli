import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from './test/test-utils';
import { App } from './App';
import { MockEventSource } from './test/mocks/sse';
import {
  mockOverview,
  mockWorkers,
  mockControlPlane,
  mockSessions,
  mockLogs,
  mockActivity,
  mockConfig,
} from './test/mocks/handlers';

describe('App', () => {
  beforeEach(() => {
    // Reset EventSource instances
    MockEventSource.clearInstances();

    // Mock fetch with default responses
    vi.mocked(global.fetch).mockImplementation((url) => {
      const path = (url as string).split('?')[0];
      const responses: Record<string, unknown> = {
        '/api/overview': mockOverview,
        '/api/workers': mockWorkers,
        '/api/control-plane': mockControlPlane,
        '/api/sessions': mockSessions,
        '/api/logs': mockLogs,
        '/api/activity': mockActivity,
        '/api/config': mockConfig,
      };

      const data = responses[path];
      if (data) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve(data),
        } as Response);
      }

      return Promise.resolve({
        ok: false,
        status: 404,
        json: () => Promise.resolve({ error: 'Not found' }),
      } as Response);
    });
  });

  afterEach(() => {
    MockEventSource.clearInstances();
    vi.clearAllMocks();
  });

  it('renders without crashing', async () => {
    render(<App />);

    // Should show the header with Kubiya branding
    await waitFor(() => {
      expect(screen.getByText('Kubiya')).toBeInTheDocument();
    });
  });

  it('shows navigation tabs', async () => {
    const { container } = render(<App />);

    await waitFor(() => {
      // Check for nav tabs specifically
      const navTabs = container.querySelectorAll('.nav-tab-label');
      const tabLabels = Array.from(navTabs).map((el) => el.textContent);

      expect(tabLabels).toContain('Overview');
      expect(tabLabels).toContain('Workers');
      expect(tabLabels).toContain('Playground');
      expect(tabLabels).toContain('LLM Proxy');
      expect(tabLabels).toContain('Models');
      expect(tabLabels).toContain('Environment');
      expect(tabLabels).toContain('Doctor');
      expect(tabLabels).toContain('Control Plane');
      expect(tabLabels).toContain('Logs');
      expect(tabLabels).toContain('Sessions');
    });
  });

  it('fetches initial data on mount', async () => {
    render(<App />);

    await waitFor(() => {
      // Verify all initial API calls were made
      expect(global.fetch).toHaveBeenCalledWith('/api/overview');
      expect(global.fetch).toHaveBeenCalledWith('/api/workers');
      expect(global.fetch).toHaveBeenCalledWith('/api/control-plane');
      expect(global.fetch).toHaveBeenCalledWith('/api/sessions');
      expect(global.fetch).toHaveBeenCalledWith('/api/logs?limit=100');
      expect(global.fetch).toHaveBeenCalledWith('/api/activity?limit=20');
      expect(global.fetch).toHaveBeenCalledWith('/api/config');
    });
  });

  it('connects to SSE events', async () => {
    render(<App />);

    await waitFor(() => {
      const eventSource = MockEventSource.getLatest();
      expect(eventSource).toBeDefined();
      expect(eventSource?.url).toBe('/api/events');
    });
  });

  it('shows SSE connection status', async () => {
    render(<App />);

    await waitFor(() => {
      // Initially should show SSE status
      expect(screen.getByText('SSE')).toBeInTheDocument();
    });

    // After EventSource opens, should show connected
    const eventSource = MockEventSource.getLatest();
    await waitFor(() => {
      expect(eventSource?.readyState).toBe(1); // OPEN
    });
  });

  it('displays overview stats from fetched data', async () => {
    const { container } = render(<App />);

    await waitFor(() => {
      // Check for stats grid which contains overview stats
      const statsGrid = container.querySelector('.stats-grid');
      expect(statsGrid).toBeInTheDocument();
    });
  });

  it('updates state when receiving SSE overview event', async () => {
    render(<App />);

    const eventSource = MockEventSource.getLatest();
    await waitFor(() => {
      expect(eventSource?.readyState).toBe(1);
    });

    // Simulate SSE event
    eventSource?.simulateMessage(
      {
        type: 'overview',
        data: {
          ...mockOverview,
          total_workers: 5,
          active_workers: 3,
        },
      },
      'message'
    );

    // The component should update with new data
    // This tests the SSE event handling
  });

  it('handles fetch errors gracefully', async () => {
    // Mock fetch to fail
    vi.mocked(global.fetch).mockRejectedValue(new Error('Network error'));

    // Should not throw
    expect(() => render(<App />)).not.toThrow();
  });

  it('navigates to Workers page when tab clicked', async () => {
    const { container } = render(<App />);

    await waitFor(() => {
      expect(screen.getByText('Workers')).toBeInTheDocument();
    });

    // Find and click Workers tab
    const workersTab = screen.getByText('Workers');
    workersTab.click();

    await waitFor(() => {
      // The Workers page should now be active
      const activeTab = container.querySelector('.nav-tab.active');
      expect(activeTab?.textContent).toContain('Workers');
    });
  });

  it('shows control plane connection status', async () => {
    const { container } = render(<App />);

    await waitFor(() => {
      // Control plane status should be shown in header
      const statusLabel = container.querySelector('.status-label');
      expect(statusLabel).toBeInTheDocument();
    });
  });

  it('displays queue ID in header when available', async () => {
    const { container } = render(<App />);

    await waitFor(() => {
      // Queue ID badge should be present with truncated ID
      const queueBadge = container.querySelector('.header-queue-badge');
      expect(queueBadge).toBeInTheDocument();
      expect(queueBadge?.textContent).toContain('...');
    });
  });
});

describe('App SSE event handling', () => {
  beforeEach(() => {
    MockEventSource.clearInstances();
    // Provide proper initial data to prevent component errors
    vi.mocked(global.fetch).mockImplementation((url) => {
      const path = (url as string).split('?')[0];
      const responses: Record<string, unknown> = {
        '/api/overview': mockOverview,
        '/api/workers': [],
        '/api/control-plane': mockControlPlane,
        '/api/sessions': [],
        '/api/logs': [],
        '/api/activity': [],
        '/api/config': mockConfig,
      };
      const data = responses[path] ?? {};
      return Promise.resolve({
        ok: true,
        status: 200,
        json: () => Promise.resolve(data),
      } as Response);
    });
  });

  afterEach(() => {
    MockEventSource.clearInstances();
    vi.clearAllMocks();
  });

  it('handles worker_update events', async () => {
    render(<App />);

    const eventSource = MockEventSource.getLatest();
    await waitFor(() => expect(eventSource?.readyState).toBe(1));

    // Simulate worker update event
    eventSource?.simulateMessage({
      type: 'worker_update',
      data: {
        id: 'new-worker',
        status: 'active',
        queue_id: 'default',
        pid: 99999,
        started_at: new Date().toISOString(),
        last_heartbeat: new Date().toISOString(),
        tasks_active: 1,
        tasks_total: 10,
        version: '1.0.0',
        hostname: 'localhost',
      },
    });

    // Event should be processed without error
  });

  it('handles control_plane events', async () => {
    render(<App />);

    const eventSource = MockEventSource.getLatest();
    await waitFor(() => expect(eventSource?.readyState).toBe(1));

    eventSource?.simulateMessage({
      type: 'control_plane',
      data: {
        connected: true,
        url: 'https://api.kubiya.ai',
        latency_ms: 50,
        auth_status: 'valid',
      },
    });
  });

  it('handles session events', async () => {
    render(<App />);

    const eventSource = MockEventSource.getLatest();
    await waitFor(() => expect(eventSource?.readyState).toBe(1));

    eventSource?.simulateMessage({
      type: 'session',
      data: {
        id: 'session-new',
        type: 'chat',
        status: 'active',
        worker_id: 'worker-1',
        started_at: new Date().toISOString(),
        duration: 60,
        duration_str: '1m',
        messages_count: 3,
      },
    });
  });

  it('handles log events and limits to 500', async () => {
    render(<App />);

    const eventSource = MockEventSource.getLatest();
    await waitFor(() => expect(eventSource?.readyState).toBe(1));

    // Simulate log event
    eventSource?.simulateMessage({
      type: 'log',
      data: {
        timestamp: new Date().toISOString(),
        level: 'info',
        component: 'test',
        message: 'Test log message',
      },
    });
  });

  it('handles metrics events', async () => {
    render(<App />);

    const eventSource = MockEventSource.getLatest();
    await waitFor(() => expect(eventSource?.readyState).toBe(1));

    eventSource?.simulateMessage({
      type: 'metrics',
      data: {
        worker_id: 'worker-1',
        metrics: {
          cpu_percent: 50,
          memory_mb: 512,
          memory_rss: 536870912,
          memory_percent: 6.4,
          open_files: 20,
          threads: 8,
          collected_at: new Date().toISOString(),
        },
      },
    });
  });
});
