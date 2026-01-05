import { h } from 'preact';
import { Layout } from './components/Layout';
import { Overview } from './components/Overview';
import { WorkerList } from './components/WorkerList';
import { ControlPlane } from './components/ControlPlane';
import { Diagnostics } from './components/Diagnostics';
import { Sessions } from './components/Sessions';
import { Doctor } from './components/Doctor';
import { ProxyControl } from './components/ProxyControl';
import { Models } from './components/Models';
import { Environment } from './components/Environment';
import { Playground } from './components/Playground';
import { ToastContainer } from './components/Toast';
import { WebUIProvider, ToastProvider, useWebUI } from './state';
import type { Page } from './state';

// Re-export types for backwards compatibility
export type { Page } from './state';
export type {
  AppState,
  Overview,
  Worker,
  WorkerMetrics,
  ControlPlaneStatus,
  Session,
  LogEntry,
  RecentActivity,
  WorkerConfig,
} from './state';

/**
 * Main application component.
 * Wraps AppContent with providers for state management and toast notifications.
 */
export function App() {
  return (
    <ToastProvider>
      <WebUIProvider>
        <AppContent />
        <ToastContainer />
      </WebUIProvider>
    </ToastProvider>
  );
}

/**
 * Application content that consumes the WebUI context.
 */
function AppContent() {
  const {
    currentPage,
    setPage,
    connected,
    controlPlane,
    config,
    overview,
    workers,
    sessions,
    logs,
    activity,
  } = useWebUI();

  const renderPage = () => {
    switch (currentPage) {
      case 'overview':
        return (
          <Overview
            state={{
              overview,
              workers,
              controlPlane,
              sessions,
              logs,
              activity,
              config,
              connected,
            }}
          />
        );
      case 'workers':
        return <WorkerList workers={workers} />;
      case 'proxy':
        return <ProxyControl />;
      case 'models':
        return <Models />;
      case 'playground':
        return <Playground />;
      case 'doctor':
        return <Doctor />;
      case 'environment':
        return <Environment />;
      case 'control-plane':
        return <ControlPlane status={controlPlane} />;
      case 'diagnostics':
        return <Diagnostics logs={logs} config={config} />;
      case 'sessions':
        return <Sessions sessions={sessions} />;
      default:
        return (
          <Overview
            state={{
              overview,
              workers,
              controlPlane,
              sessions,
              logs,
              activity,
              config,
              connected,
            }}
          />
        );
    }
  };

  return (
    <Layout
      currentPage={currentPage}
      onNavigate={setPage}
      connected={connected}
      queueId={config?.queue_id}
      controlPlane={controlPlane}
      config={config}
    >
      {renderPage()}
    </Layout>
  );
}
