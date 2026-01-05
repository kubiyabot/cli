/**
 * Shared types for the WebUI application state.
 * All interfaces are centralized here to avoid prop drilling
 * and ensure type consistency across components.
 */

// Page navigation types
export type Page =
  | 'overview'
  | 'workers'
  | 'proxy'
  | 'models'
  | 'playground'
  | 'environment'
  | 'doctor'
  | 'control-plane'
  | 'diagnostics'
  | 'sessions';

// Overview statistics
export interface Overview {
  total_workers: number;
  active_workers: number;
  idle_workers: number;
  tasks_processed: number;
  tasks_active: number;
  tasks_failed: number;
  error_rate: number;
  uptime: number;
  uptime_formatted: string;
  control_plane_ok: boolean;
  litellm_proxy_ok?: boolean;
  start_time: string;
}

// Worker types
export interface Worker {
  id: string;
  queue_id: string;
  status: string;
  pid: number;
  started_at: string;
  last_heartbeat: string;
  tasks_active: number;
  tasks_total: number;
  version: string;
  hostname: string;
  metrics?: WorkerMetrics;
}

export interface WorkerMetrics {
  cpu_percent: number;
  memory_mb: number;
  memory_rss: number;
  memory_percent: number;
  open_files: number;
  threads: number;
  collected_at: string;
}

// Control plane types
export interface ControlPlaneStatus {
  connected: boolean;
  url: string;
  latency: number;
  latency_ms: number;
  last_check: string;
  last_success: string;
  auth_status: string;
  config_version?: string;
  error_message?: string;
  reconnect_count: number;
}

// Session types
export interface Session {
  id: string;
  type: string;
  status: string;
  worker_id: string;
  agent_id?: string;
  agent_name?: string;
  started_at: string;
  ended_at?: string;
  duration: number;
  duration_str: string;
  messages_count: number;
  tokens_used?: number;
}

// Logging types
export interface LogEntry {
  timestamp: string;
  level: string;
  component: string;
  message: string;
  worker_id?: string;
  session_id?: string;
}

export interface RecentActivity {
  type: string;
  description: string;
  timestamp: string;
  worker_id?: string;
  session_id?: string;
}

// Configuration types
export interface WorkerConfig {
  queue_id: string;
  queue_name?: string;
  deployment_type: string;
  control_plane_url: string;
  enable_local_proxy: boolean;
  proxy_port?: number;
  model_override?: string;
  auto_update: boolean;
  daemon_mode: boolean;
  worker_dir: string;
  // System info for UI display
  version?: string;
  build_commit?: string;
  build_date?: string;
  go_version?: string;
  os?: string;
  arch?: string;
}

// Combined application state
export interface AppState {
  overview: Overview | null;
  workers: Worker[];
  controlPlane: ControlPlaneStatus | null;
  sessions: Session[];
  logs: LogEntry[];
  activity: RecentActivity[];
  config: WorkerConfig | null;
  connected: boolean;
}

// SSE event types
export interface SSEEvent {
  type: string;
  data: unknown;
}

// API response types for type-safe fetching
export interface ApiError {
  error: string;
  message?: string;
  code?: string;
}

// Proxy types
export interface ProxyStatus {
  running: boolean;
  pid?: number;
  port?: number;
  started_at?: string;
  uptime?: number;
  uptime_formatted?: string;
  models_available?: number;
  requests_total?: number;
  error?: string;
}

export interface ProxyConfig {
  enabled: boolean;
  port: number;
  config_path?: string;
  models?: ProxyModel[];
}

export interface ProxyModel {
  model_name: string;
  litellm_params: {
    model: string;
    api_key?: string;
    api_base?: string;
  };
}

// Doctor diagnostic types
export interface DiagnosticCheck {
  name: string;
  status: 'pass' | 'fail' | 'warn' | 'running';
  message: string;
  details?: string;
  duration_ms?: number;
}

export interface DiagnosticResult {
  overall_status: 'healthy' | 'degraded' | 'unhealthy';
  checks: DiagnosticCheck[];
  timestamp: string;
}

// Model types
export interface Model {
  id: string;
  name: string;
  provider: string;
  available: boolean;
  context_length?: number;
  pricing?: {
    input_per_1k?: number;
    output_per_1k?: number;
  };
}

// Environment variable types
export interface EnvironmentVariable {
  name: string;
  value: string;
  source: 'config' | 'env' | 'default';
  sensitive?: boolean;
}

// Agent types (for Playground)
export interface Agent {
  id: string;
  name: string;
  description?: string;
  model?: string;
  tools?: string[];
  created_at?: string;
}

export interface Execution {
  id: string;
  agent_id: string;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';
  prompt: string;
  started_at: string;
  ended_at?: string;
  result?: string;
  error?: string;
}

// Stream event types (for Playground streaming)
export interface StreamEvent {
  type: 'text' | 'tool_call' | 'tool_result' | 'error' | 'done' | 'status';
  content?: string;
  tool_name?: string;
  tool_input?: Record<string, unknown>;
  tool_output?: string;
  error?: string;
  timestamp?: string;
}
