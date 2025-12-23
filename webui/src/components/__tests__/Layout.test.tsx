import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '../../test/test-utils';
import { Layout } from '../Layout';
import type { ControlPlaneStatus } from '../../App';

const defaultProps = {
  currentPage: 'overview' as const,
  onNavigate: vi.fn(),
  connected: true,
  queueId: 'test-queue-123',
  controlPlane: {
    connected: true,
    url: 'https://api.kubiya.ai',
    latency: 0.045,
    latency_ms: 45,
    last_check: new Date().toISOString(),
    last_success: new Date().toISOString(),
    auth_status: 'valid',
    reconnect_count: 0,
  } as ControlPlaneStatus,
};

describe('Layout', () => {
  it('renders without crashing', () => {
    render(
      <Layout {...defaultProps}>
        <div>Test Content</div>
      </Layout>
    );
    expect(screen.getByText('Test Content')).toBeInTheDocument();
  });

  it('displays Kubiya branding', () => {
    render(
      <Layout {...defaultProps}>
        <div>Content</div>
      </Layout>
    );
    expect(screen.getByText('Kubiya')).toBeInTheDocument();
    expect(screen.getByText('Worker Pool')).toBeInTheDocument();
  });

  it('shows all navigation tabs', () => {
    const { container } = render(
      <Layout {...defaultProps}>
        <div>Content</div>
      </Layout>
    );

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

  it('highlights current page tab as active', () => {
    const { container } = render(
      <Layout {...defaultProps} currentPage="workers">
        <div>Content</div>
      </Layout>
    );

    const activeTab = container.querySelector('.nav-tab.active');
    expect(activeTab?.textContent).toContain('Workers');
  });

  it('calls onNavigate when tab is clicked', async () => {
    const onNavigate = vi.fn();
    const { container } = render(
      <Layout {...defaultProps} onNavigate={onNavigate}>
        <div>Content</div>
      </Layout>
    );

    const workersTab = container.querySelector('.nav-tab:nth-child(2)');
    workersTab?.dispatchEvent(new MouseEvent('click', { bubbles: true }));

    expect(onNavigate).toHaveBeenCalledWith('workers');
  });

  it('shows SSE connected status', () => {
    const { container } = render(
      <Layout {...defaultProps} connected={true}>
        <div>Content</div>
      </Layout>
    );

    const statusDot = container.querySelector('.status-dot.connected');
    expect(statusDot).toBeInTheDocument();
  });

  it('shows SSE connecting status when not connected', () => {
    const { container } = render(
      <Layout {...defaultProps} connected={false}>
        <div>Content</div>
      </Layout>
    );

    const statusDot = container.querySelector('.status-dot.connecting');
    expect(statusDot).toBeInTheDocument();
  });

  it('shows control plane connected status', () => {
    const { container } = render(
      <Layout {...defaultProps}>
        <div>Content</div>
      </Layout>
    );

    const cpStatus = container.querySelector('.status-text');
    expect(cpStatus?.textContent).toBe('Connected');
  });

  it('shows control plane latency', () => {
    render(
      <Layout {...defaultProps}>
        <div>Content</div>
      </Layout>
    );

    expect(screen.getByText('45ms')).toBeInTheDocument();
  });

  it('shows control plane disconnected status', () => {
    const disconnectedControlPlane = {
      ...defaultProps.controlPlane,
      connected: false,
    };

    const { container } = render(
      <Layout {...defaultProps} controlPlane={disconnectedControlPlane}>
        <div>Content</div>
      </Layout>
    );

    const cpStatus = container.querySelector('.status-text');
    expect(cpStatus?.textContent).toBe('Disconnected');
  });

  it('shows auth error status when auth expired', () => {
    const authExpiredControlPlane = {
      ...defaultProps.controlPlane,
      connected: false,
      auth_status: 'expired',
    };

    const { container } = render(
      <Layout {...defaultProps} controlPlane={authExpiredControlPlane}>
        <div>Content</div>
      </Layout>
    );

    const cpStatus = container.querySelector('.status-text');
    expect(cpStatus?.textContent).toBe('Auth Error');
  });

  it('shows checking status when control plane is null', () => {
    const { container } = render(
      <Layout {...defaultProps} controlPlane={null}>
        <div>Content</div>
      </Layout>
    );

    const cpStatus = container.querySelector('.status-text');
    expect(cpStatus?.textContent).toBe('Checking...');
  });

  it('shows truncated queue ID in header', () => {
    render(
      <Layout {...defaultProps} queueId="test-queue-12345678">
        <div>Content</div>
      </Layout>
    );

    // Queue ID is truncated to first 8 chars + "..."
    expect(screen.getByText('test-que...')).toBeInTheDocument();
  });

  it('does not show queue badge when no queueId', () => {
    const { container } = render(
      <Layout {...defaultProps} queueId={undefined}>
        <div>Content</div>
      </Layout>
    );

    const queueBadge = container.querySelector('.header-queue-badge');
    expect(queueBadge).not.toBeInTheDocument();
  });

  it('renders children in main content area', () => {
    const { container } = render(
      <Layout {...defaultProps}>
        <div data-testid="child-content">Child Content</div>
      </Layout>
    );

    const mainContent = container.querySelector('.content');
    expect(mainContent).toContainElement(screen.getByTestId('child-content'));
  });
});
