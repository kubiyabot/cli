import { useState, useEffect, useRef, useCallback } from 'preact/hooks';

export interface SSEEvent {
  type: string;
  data: unknown;
}

export function useSSE(onEvent: (event: SSEEvent) => void) {
  const [connected, setConnected] = useState(false);
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<number | null>(null);

  const connect = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }

    const eventSource = new EventSource('/api/events');
    eventSourceRef.current = eventSource;

    eventSource.onopen = () => {
      setConnected(true);
      console.log('SSE connected');
    };

    eventSource.onerror = () => {
      setConnected(false);
      eventSource.close();

      // Reconnect after delay
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      reconnectTimeoutRef.current = window.setTimeout(() => {
        console.log('SSE reconnecting...');
        connect();
      }, 3000);
    };

    // Handle named events
    const eventTypes = [
      'overview',
      'worker_update',
      'metrics',
      'log',
      'session',
      'control_plane',
      'heartbeat',
    ];

    eventTypes.forEach((type) => {
      eventSource.addEventListener(type, (e: MessageEvent) => {
        try {
          const eventData = JSON.parse(e.data);
          onEvent({ type, data: eventData.data });
        } catch (err) {
          console.error('Failed to parse SSE event:', err);
        }
      });
    });
  }, [onEvent]);

  useEffect(() => {
    connect();

    return () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
    };
  }, [connect]);

  return { connected };
}
