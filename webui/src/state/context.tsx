import { h, createContext, ComponentChildren } from 'preact';
import { useContext, useState, useEffect, useCallback, useMemo } from 'preact/hooks';
import { useSSE } from '../hooks/useSSE';
import type {
  Page,
  AppState,
  Overview,
  Worker,
  WorkerMetrics,
  ControlPlaneStatus,
  Session,
  LogEntry,
  RecentActivity,
  WorkerConfig,
} from './types';

// Constants
const MAX_LOGS = 500;
const MAX_ACTIVITY = 100;

// Context value interface
export interface WebUIContextValue {
  // Navigation
  currentPage: Page;
  setPage: (page: Page) => void;

  // Connection status
  connected: boolean;

  // State data
  overview: Overview | null;
  workers: Worker[];
  controlPlane: ControlPlaneStatus | null;
  sessions: Session[];
  logs: LogEntry[];
  activity: RecentActivity[];
  config: WorkerConfig | null;

  // Refresh functions
  refreshAll: () => Promise<void>;
  refreshWorkers: () => Promise<void>;
  refreshSessions: () => Promise<void>;
  refreshLogs: () => Promise<void>;
}

// Create context with undefined default
const WebUIContext = createContext<WebUIContextValue | undefined>(undefined);

// Initial state
const initialState: AppState = {
  overview: null,
  workers: [],
  controlPlane: null,
  sessions: [],
  logs: [],
  activity: [],
  config: null,
  connected: false,
};

// Provider props
interface WebUIProviderProps {
  children: ComponentChildren;
}

/**
 * WebUI Context Provider
 * Centralizes all application state and SSE event handling.
 */
export function WebUIProvider({ children }: WebUIProviderProps) {
  const [currentPage, setCurrentPage] = useState<Page>('overview');
  const [state, setState] = useState<AppState>(initialState);

  // SSE event handler
  const handleSSEEvent = useCallback((event: { type: string; data: unknown }) => {
    const { type, data } = event;

    switch (type) {
      case 'overview':
        setState((s) => ({ ...s, overview: data as Overview }));
        break;

      case 'worker_update':
        setState((s) => {
          const worker = data as Worker;
          const existingIndex = s.workers.findIndex((w) => w.id === worker.id);
          if (existingIndex >= 0) {
            const workers = [...s.workers];
            workers[existingIndex] = worker;
            return { ...s, workers };
          }
          return { ...s, workers: [...s.workers, worker] };
        });
        break;

      case 'worker_remove':
        setState((s) => {
          const { id } = data as { id: string };
          return { ...s, workers: s.workers.filter((w) => w.id !== id) };
        });
        break;

      case 'control_plane':
        setState((s) => ({ ...s, controlPlane: data as ControlPlaneStatus }));
        break;

      case 'session':
        setState((s) => {
          const session = data as Session;
          const existingIndex = s.sessions.findIndex((sess) => sess.id === session.id);
          if (existingIndex >= 0) {
            const sessions = [...s.sessions];
            sessions[existingIndex] = session;
            return { ...s, sessions };
          }
          return { ...s, sessions: [...s.sessions, session] };
        });
        break;

      case 'session_end':
        setState((s) => {
          const { id } = data as { id: string };
          return { ...s, sessions: s.sessions.filter((sess) => sess.id !== id) };
        });
        break;

      case 'log':
        setState((s) => ({
          ...s,
          logs: [data as LogEntry, ...s.logs].slice(0, MAX_LOGS),
        }));
        break;

      case 'activity':
        setState((s) => ({
          ...s,
          activity: [data as RecentActivity, ...s.activity].slice(0, MAX_ACTIVITY),
        }));
        break;

      case 'metrics':
        setState((s) => {
          const { worker_id, metrics } = data as { worker_id: string; metrics: WorkerMetrics };
          const workers = s.workers.map((w) =>
            w.id === worker_id ? { ...w, metrics } : w
          );
          return { ...s, workers };
        });
        break;

      case 'config':
        setState((s) => ({ ...s, config: data as WorkerConfig }));
        break;
    }
  }, []);

  // Subscribe to SSE events
  const { connected } = useSSE(handleSSEEvent);

  // Update connection state
  useEffect(() => {
    setState((s) => ({ ...s, connected }));
  }, [connected]);

  // Fetch helper with error handling
  const fetchJSON = useCallback(async <T,>(url: string): Promise<T | null> => {
    try {
      const response = await fetch(url);
      if (!response.ok) {
        console.error(`Failed to fetch ${url}: ${response.status}`);
        return null;
      }
      return await response.json();
    } catch (err) {
      console.error(`Error fetching ${url}:`, err);
      return null;
    }
  }, []);

  // Refresh functions
  const refreshWorkers = useCallback(async () => {
    const workers = await fetchJSON<Worker[]>('/api/workers');
    if (workers) {
      setState((s) => ({ ...s, workers }));
    }
  }, [fetchJSON]);

  const refreshSessions = useCallback(async () => {
    const sessions = await fetchJSON<Session[]>('/api/sessions');
    if (sessions) {
      setState((s) => ({ ...s, sessions }));
    }
  }, [fetchJSON]);

  const refreshLogs = useCallback(async () => {
    const logs = await fetchJSON<LogEntry[]>('/api/logs?limit=100');
    if (logs) {
      setState((s) => ({ ...s, logs }));
    }
  }, [fetchJSON]);

  const refreshAll = useCallback(async () => {
    const [overview, workers, controlPlane, sessions, logs, activity, config] = await Promise.all([
      fetchJSON<Overview>('/api/overview'),
      fetchJSON<Worker[]>('/api/workers'),
      fetchJSON<ControlPlaneStatus>('/api/control-plane'),
      fetchJSON<Session[]>('/api/sessions'),
      fetchJSON<LogEntry[]>('/api/logs?limit=100'),
      fetchJSON<RecentActivity[]>('/api/activity?limit=20'),
      fetchJSON<WorkerConfig>('/api/config'),
    ]);

    setState((s) => ({
      ...s,
      overview: overview ?? s.overview,
      workers: workers ?? s.workers,
      controlPlane: controlPlane ?? s.controlPlane,
      sessions: sessions ?? s.sessions,
      logs: logs ?? s.logs,
      activity: activity ?? s.activity,
      config: config ?? s.config,
    }));
  }, [fetchJSON]);

  // Initial data fetch
  useEffect(() => {
    refreshAll();
  }, [refreshAll]);

  // Memoize page setter to prevent unnecessary re-renders
  const setPage = useCallback((page: Page) => {
    setCurrentPage(page);
  }, []);

  // Memoize context value
  const contextValue = useMemo<WebUIContextValue>(
    () => ({
      currentPage,
      setPage,
      connected: state.connected,
      overview: state.overview,
      workers: state.workers,
      controlPlane: state.controlPlane,
      sessions: state.sessions,
      logs: state.logs,
      activity: state.activity,
      config: state.config,
      refreshAll,
      refreshWorkers,
      refreshSessions,
      refreshLogs,
    }),
    [
      currentPage,
      setPage,
      state.connected,
      state.overview,
      state.workers,
      state.controlPlane,
      state.sessions,
      state.logs,
      state.activity,
      state.config,
      refreshAll,
      refreshWorkers,
      refreshSessions,
      refreshLogs,
    ]
  );

  return (
    <WebUIContext.Provider value={contextValue}>
      {children}
    </WebUIContext.Provider>
  );
}

/**
 * Hook to access the WebUI context.
 * Must be used within a WebUIProvider.
 */
export function useWebUI(): WebUIContextValue {
  const context = useContext(WebUIContext);
  if (context === undefined) {
    throw new Error('useWebUI must be used within a WebUIProvider');
  }
  return context;
}

/**
 * Hook to access only navigation state (for performance).
 */
export function useNavigation() {
  const { currentPage, setPage } = useWebUI();
  return { currentPage, setPage };
}

/**
 * Hook to access only connection status.
 */
export function useConnection() {
  const { connected, controlPlane } = useWebUI();
  return { connected, controlPlane };
}

/**
 * Hook to access workers state.
 */
export function useWorkers() {
  const { workers, refreshWorkers } = useWebUI();
  return { workers, refreshWorkers };
}

/**
 * Hook to access sessions state.
 */
export function useSessions() {
  const { sessions, refreshSessions } = useWebUI();
  return { sessions, refreshSessions };
}

/**
 * Hook to access logs state.
 */
export function useLogs() {
  const { logs, refreshLogs } = useWebUI();
  return { logs, refreshLogs };
}
