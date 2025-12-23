import { h, Fragment, ComponentChildren } from 'preact';
import type { Page, ControlPlaneStatus } from '../App';
import { KubiyaLogo } from './Logo';

interface LayoutProps {
  children: ComponentChildren;
  currentPage: Page;
  onNavigate: (page: Page) => void;
  connected: boolean;
  queueId?: string;
  controlPlane?: ControlPlaneStatus | null;
}

const PAGES: { id: Page; label: string; icon: string }[] = [
  { id: 'overview', label: 'Overview', icon: 'chart' },
  { id: 'workers', label: 'Workers', icon: 'cpu' },
  { id: 'playground', label: 'Playground', icon: 'play' },
  { id: 'proxy', label: 'LLM Proxy', icon: 'route' },
  { id: 'models', label: 'Models', icon: 'brain' },
  { id: 'environment', label: 'Environment', icon: 'settings' },
  { id: 'doctor', label: 'Doctor', icon: 'health' },
  { id: 'control-plane', label: 'Control Plane', icon: 'cloud' },
  { id: 'diagnostics', label: 'Logs', icon: 'terminal' },
  { id: 'sessions', label: 'Sessions', icon: 'chat' },
];

// Icon component for navigation
function NavIcon({ type }: { type: string }) {
  const icons: Record<string, h.JSX.Element> = {
    chart: <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M3 3v18h18"/><path d="M18 17V9"/><path d="M13 17V5"/><path d="M8 17v-3"/></svg>,
    cpu: <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="4" y="4" width="16" height="16" rx="2"/><rect x="9" y="9" width="6" height="6"/><path d="M9 1v3M15 1v3M9 20v3M15 20v3M20 9h3M20 14h3M1 9h3M1 14h3"/></svg>,
    play: <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polygon points="5 3 19 12 5 21 5 3"/></svg>,
    route: <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><circle cx="6" cy="19" r="3"/><path d="M9 19h8.5a3.5 3.5 0 0 0 0-7h-11a3.5 3.5 0 0 1 0-7H15"/><circle cx="18" cy="5" r="3"/></svg>,
    brain: <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M12 5a3 3 0 1 0-5.997.125 4 4 0 0 0-2.526 5.77 4 4 0 0 0 .556 6.588A4 4 0 1 0 12 18Z"/><path d="M12 5a3 3 0 1 1 5.997.125 4 4 0 0 1 2.526 5.77 4 4 0 0 1-.556 6.588A4 4 0 1 1 12 18Z"/><path d="M12 5v13"/></svg>,
    settings: <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>,
    health: <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>,
    cloud: <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M18 10h-1.26A8 8 0 1 0 9 20h9a5 5 0 0 0 0-10z"/></svg>,
    terminal: <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>,
    chat: <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>,
  };
  return icons[type] || null;
}

export function Layout({ children, currentPage, onNavigate, connected, queueId, controlPlane }: LayoutProps) {
  const getControlPlaneStatus = () => {
    if (!controlPlane) return { class: 'connecting', text: 'Checking...' };
    if (controlPlane.connected) return { class: 'connected', text: 'Connected' };
    if (controlPlane.auth_status === 'expired') return { class: 'disconnected', text: 'Auth Error' };
    return { class: 'disconnected', text: 'Disconnected' };
  };

  const cpStatus = getControlPlaneStatus();

  return (
    <div class="layout">
      <header class="header">
        <div class="header-title">
          <div class="header-brand">
            <div class="header-logo">
              <KubiyaLogo size={28} />
            </div>
            <div class="header-brand-text">
              <h1>Kubiya</h1>
              <span class="header-brand-subtitle">Worker Pool</span>
            </div>
          </div>
          {queueId && (
            <div class="header-queue-badge">
              <span class="header-queue-icon">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M22 12h-4l-3 9L9 3l-3 9H2"/>
                </svg>
              </span>
              <span class="header-queue-text">{queueId.slice(0, 8)}...</span>
            </div>
          )}
        </div>
        <div class="header-status">
          <div class="status-group">
            <div class="status-item">
              <span class="status-label">SSE</span>
              <span class={`status-dot ${connected ? 'connected' : 'connecting'}`} />
            </div>
            <div class="status-divider" />
            <div class="status-item">
              <span class="status-label">Control Plane</span>
              <span class={`status-dot ${cpStatus.class}`} />
              <span class="status-text">{cpStatus.text}</span>
              {controlPlane?.latency_ms && controlPlane.connected && (
                <span class="status-latency">{controlPlane.latency_ms}ms</span>
              )}
            </div>
          </div>
        </div>
      </header>

      <nav class="nav">
        <ul class="nav-tabs">
          {PAGES.map((page) => (
            <li
              key={page.id}
              class={`nav-tab ${currentPage === page.id ? 'active' : ''}`}
              onClick={() => onNavigate(page.id)}
            >
              <span class="nav-tab-icon">
                <NavIcon type={page.icon} />
              </span>
              <span class="nav-tab-label">{page.label}</span>
            </li>
          ))}
        </ul>
      </nav>

      <main class="content">{children}</main>
    </div>
  );
}
