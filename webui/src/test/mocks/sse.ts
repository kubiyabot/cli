import { vi } from 'vitest';

type EventHandler = (event: MessageEvent) => void;

/**
 * Mock EventSource for testing SSE connections
 */
export class MockEventSource {
  static instances: MockEventSource[] = [];
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSED = 2;

  readonly CONNECTING = 0;
  readonly OPEN = 1;
  readonly CLOSED = 2;

  url: string;
  readyState: number = 0;
  withCredentials: boolean = false;

  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;

  private handlers: Map<string, Set<EventHandler>> = new Map();
  private closed = false;

  constructor(url: string, eventSourceInitDict?: EventSourceInit) {
    this.url = url;
    this.withCredentials = eventSourceInitDict?.withCredentials ?? false;
    MockEventSource.instances.push(this);

    // Auto-open connection after microtask
    queueMicrotask(() => {
      if (!this.closed) {
        this.readyState = 1;
        this.onopen?.(new Event('open'));
      }
    });
  }

  addEventListener(type: string, handler: EventHandler): void {
    if (!this.handlers.has(type)) {
      this.handlers.set(type, new Set());
    }
    this.handlers.get(type)!.add(handler);
  }

  removeEventListener(type: string, handler: EventHandler): void {
    this.handlers.get(type)?.delete(handler);
  }

  close(): void {
    this.closed = true;
    this.readyState = 2;
    const index = MockEventSource.instances.indexOf(this);
    if (index > -1) {
      MockEventSource.instances.splice(index, 1);
    }
  }

  // Test helpers

  /**
   * Simulate receiving an SSE message
   */
  simulateMessage(data: unknown, type: string = 'message'): void {
    if (this.closed) return;

    const event = new MessageEvent(type, {
      data: typeof data === 'string' ? data : JSON.stringify(data),
    });

    // Call type-specific handlers
    this.handlers.get(type)?.forEach((handler) => handler(event));

    // Call general onmessage for 'message' type events
    if (type === 'message') {
      this.onmessage?.(event);
    }
  }

  /**
   * Simulate an error
   */
  simulateError(): void {
    if (this.closed) return;
    this.readyState = 2;
    this.onerror?.(new Event('error'));
  }

  /**
   * Simulate reconnection
   */
  simulateReconnect(): void {
    if (this.closed) return;
    this.readyState = 1;
    this.onopen?.(new Event('open'));
  }

  /**
   * Get the most recent MockEventSource instance
   */
  static getLatest(): MockEventSource | undefined {
    return MockEventSource.instances[MockEventSource.instances.length - 1];
  }

  /**
   * Clear all instances (call in afterEach)
   */
  static clearInstances(): void {
    MockEventSource.instances.forEach((instance) => instance.close());
    MockEventSource.instances = [];
  }
}

/**
 * Create a mock fetch function that returns predefined responses
 */
export function createMockFetch(responses: Record<string, unknown>) {
  return vi.fn((url: string) => {
    const path = url.replace(/^https?:\/\/[^/]+/, '');
    const data = responses[path];

    if (data === undefined) {
      return Promise.resolve({
        ok: false,
        status: 404,
        statusText: 'Not Found',
        json: () => Promise.resolve({ error: 'Not found' }),
      });
    }

    return Promise.resolve({
      ok: true,
      status: 200,
      statusText: 'OK',
      json: () => Promise.resolve(data),
    });
  });
}
