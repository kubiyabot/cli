import { http, HttpResponse } from 'msw';
import type { Overview, Worker, ControlPlaneStatus, Session, LogEntry, WorkerConfig } from '../../App';

// Default mock data
export const mockOverview: Overview = {
  total_workers: 2,
  active_workers: 1,
  idle_workers: 1,
  tasks_processed: 150,
  tasks_active: 3,
  tasks_failed: 2,
  error_rate: 1.3,
  uptime: 3600,
  uptime_formatted: '1h 0m',
  control_plane_ok: true,
  litellm_proxy_ok: true,
  start_time: new Date().toISOString(),
};

export const mockWorkers: Worker[] = [
  {
    id: 'worker-1',
    queue_id: 'default',
    status: 'active',
    pid: 12345,
    started_at: new Date().toISOString(),
    last_heartbeat: new Date().toISOString(),
    tasks_active: 2,
    tasks_total: 50,
    version: '1.0.0',
    hostname: 'localhost',
    metrics: {
      cpu_percent: 25.5,
      memory_mb: 256,
      memory_rss: 268435456,
      memory_percent: 3.2,
      open_files: 15,
      threads: 4,
      collected_at: new Date().toISOString(),
    },
  },
  {
    id: 'worker-2',
    queue_id: 'default',
    status: 'idle',
    pid: 12346,
    started_at: new Date().toISOString(),
    last_heartbeat: new Date().toISOString(),
    tasks_active: 0,
    tasks_total: 100,
    version: '1.0.0',
    hostname: 'localhost',
  },
];

export const mockControlPlane: ControlPlaneStatus = {
  connected: true,
  url: 'https://api.kubiya.ai',
  latency: 0.045,
  latency_ms: 45,
  last_check: new Date().toISOString(),
  last_success: new Date().toISOString(),
  auth_status: 'valid',
  config_version: '2.1.0',
  reconnect_count: 0,
};

export const mockSessions: Session[] = [
  {
    id: 'session-1',
    type: 'chat',
    status: 'active',
    worker_id: 'worker-1',
    agent_id: 'agent-1',
    agent_name: 'General Assistant',
    started_at: new Date().toISOString(),
    duration: 120,
    duration_str: '2m 0s',
    messages_count: 5,
    tokens_used: 1500,
  },
];

export const mockLogs: LogEntry[] = [
  {
    timestamp: new Date().toISOString(),
    level: 'info',
    component: 'worker',
    message: 'Worker started successfully',
    worker_id: 'worker-1',
  },
  {
    timestamp: new Date().toISOString(),
    level: 'debug',
    component: 'proxy',
    message: 'LiteLLM proxy health check passed',
  },
];

export const mockConfig: WorkerConfig = {
  queue_id: 'default-queue',
  queue_name: 'Default Queue',
  deployment_type: 'local',
  control_plane_url: 'https://api.kubiya.ai',
  enable_local_proxy: true,
  proxy_port: 8080,
  auto_update: true,
  daemon_mode: false,
  worker_dir: '/tmp/kubiya-worker',
};

export const mockActivity = [
  {
    type: 'task_completed',
    description: 'Task completed successfully',
    timestamp: new Date().toISOString(),
    worker_id: 'worker-1',
  },
];

// MSW handlers
export const handlers = [
  http.get('/api/overview', () => {
    return HttpResponse.json(mockOverview);
  }),

  http.get('/api/workers', () => {
    return HttpResponse.json(mockWorkers);
  }),

  http.get('/api/control-plane', () => {
    return HttpResponse.json(mockControlPlane);
  }),

  http.get('/api/sessions', () => {
    return HttpResponse.json(mockSessions);
  }),

  http.get('/api/logs', () => {
    return HttpResponse.json(mockLogs);
  }),

  http.get('/api/activity', () => {
    return HttpResponse.json(mockActivity);
  }),

  http.get('/api/config', () => {
    return HttpResponse.json(mockConfig);
  }),

  http.get('/api/health', () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.get('/api/doctor', () => {
    return HttpResponse.json({
      overall: 'healthy',
      checks: [
        { name: 'Python Version', category: 'python', status: 'pass', message: 'Python 3.11.4' },
        { name: 'Control Plane', category: 'connectivity', status: 'pass', message: 'Connected' },
      ],
      summary: { total: 2, passed: 2, failed: 0, warnings: 0 },
    });
  }),

  http.get('/api/proxy/status', () => {
    return HttpResponse.json({
      running: true,
      pid: 54321,
      port: 8080,
      base_url: 'http://localhost:8080',
      health_status: 'healthy',
      models: ['gpt-4', 'claude-3-opus'],
    });
  }),

  http.get('/api/llm/models', () => {
    return HttpResponse.json([
      { id: 'gpt-4', label: 'GPT-4', provider: 'openai', enabled: true },
      { id: 'claude-3-opus', label: 'Claude 3 Opus', provider: 'anthropic', enabled: true },
    ]);
  }),

  http.get('/api/agents', () => {
    return HttpResponse.json([
      { id: 'agent-1', name: 'General Assistant', description: 'A helpful assistant' },
    ]);
  }),
];
