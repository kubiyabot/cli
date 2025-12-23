import { h, Fragment } from 'preact';
import { useState, useEffect, useRef } from 'preact/hooks';

export interface StreamEvent {
  type: 'text' | 'tool_call' | 'tool_result' | 'reasoning' | 'error' | 'status' | 'plan' | 'done' | 'thinking' | 'output';
  content: string;
  timestamp: string;
  tool_name?: string;
  tool_input?: string;
  tool_output?: string;
  status?: string;
  duration_ms?: number;
  raw?: string; // Raw JSON for debugging
}

interface StreamViewProps {
  events: StreamEvent[];
  isRunning: boolean;
  onClear?: () => void;
}

interface ToolCallState {
  expanded: boolean;
}

// Format JSON with syntax highlighting
const formatJSON = (str: string): h.JSX.Element => {
  try {
    const obj = typeof str === 'string' ? JSON.parse(str) : str;
    const formatted = JSON.stringify(obj, null, 2);
    return <span style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>{formatted}</span>;
  } catch {
    return <span style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>{str}</span>;
  }
};

// Truncate long strings
const truncate = (str: string, maxLen: number = 100): string => {
  if (!str) return '';
  if (str.length <= maxLen) return str;
  return str.slice(0, maxLen) + '...';
};

// Format timestamp
const formatTime = (ts: string): string => {
  try {
    return new Date(ts).toLocaleTimeString('en-US', {
      hour12: false,
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit'
    });
  } catch {
    return '';
  }
};

export function StreamView({ events, isRunning, onClear }: StreamViewProps) {
  const [toolStates, setToolStates] = useState<Record<number, ToolCallState>>({});
  const [viewMode, setViewMode] = useState<'stream' | 'logs'>('stream');
  const [autoScroll, setAutoScroll] = useState(true);
  const containerRef = useRef<HTMLDivElement>(null);
  const endRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (autoScroll && endRef.current) {
      endRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [events, autoScroll]);

  const toggleToolExpand = (idx: number) => {
    setToolStates(prev => ({
      ...prev,
      [idx]: { expanded: !prev[idx]?.expanded }
    }));
  };

  // Group consecutive text events (but NOT output events - those are agent responses)
  const groupedEvents = events.reduce<Array<StreamEvent | StreamEvent[]>>((acc, event, idx) => {
    if (event.type === 'text') {
      // Only group consecutive text events (CLI noise)
      const last = acc[acc.length - 1];
      if (Array.isArray(last) && last[0].type === 'text') {
        last.push(event);
      } else {
        acc.push([event]);
      }
    } else {
      // All other events (including 'output') are not grouped
      acc.push(event);
    }
    return acc;
  }, []);

  const renderToolCall = (event: StreamEvent, idx: number) => {
    const isExpanded = toolStates[idx]?.expanded ?? false;
    const hasInput = event.tool_input && event.tool_input.length > 0;
    const inputPreview = hasInput ? truncate(event.tool_input!, 80) : '';

    return (
      <div
        key={idx}
        style={{
          background: 'var(--bg-hover)',
          borderRadius: '8px',
          overflow: 'hidden',
          border: '1px solid var(--border)',
        }}
      >
        {/* Tool call header - always visible */}
        <div
          style={{
            padding: '0.625rem 0.875rem',
            display: 'flex',
            alignItems: 'center',
            gap: '0.5rem',
            cursor: hasInput ? 'pointer' : 'default',
            userSelect: 'none',
          }}
          onClick={() => hasInput && toggleToolExpand(idx)}
        >
          <span style={{
            width: '24px',
            height: '24px',
            borderRadius: '6px',
            background: 'linear-gradient(135deg, #6366F1, #8B5CF6)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: '0.75rem',
            flexShrink: 0,
          }}>
            🔧
          </span>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{
              fontWeight: 600,
              fontSize: '0.8125rem',
              color: '#A78BFA',
              display: 'flex',
              alignItems: 'center',
              gap: '0.5rem',
            }}>
              {event.tool_name || 'Tool Call'}
              {isRunning && idx === events.length - 1 && (
                <span class="spinner" style={{ width: 12, height: 12 }} />
              )}
            </div>
            {!isExpanded && inputPreview && (
              <div style={{
                fontSize: '0.6875rem',
                color: 'var(--text-muted)',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}>
                {inputPreview}
              </div>
            )}
          </div>
          <span style={{ fontSize: '0.625rem', color: 'var(--text-muted)', flexShrink: 0 }}>
            {formatTime(event.timestamp)}
          </span>
          {hasInput && (
            <span style={{
              fontSize: '0.75rem',
              color: 'var(--text-muted)',
              transform: isExpanded ? 'rotate(180deg)' : 'rotate(0deg)',
              transition: 'transform 0.2s',
              flexShrink: 0,
            }}>
              ▼
            </span>
          )}
        </div>

        {/* Expanded input */}
        {isExpanded && hasInput && (
          <div style={{
            padding: '0.75rem',
            borderTop: '1px solid var(--border)',
            background: 'var(--bg-primary)',
            maxHeight: '200px',
            overflow: 'auto',
          }}>
            <div style={{ fontSize: '0.625rem', color: 'var(--text-muted)', marginBottom: '0.375rem', textTransform: 'uppercase' }}>
              Input
            </div>
            <pre style={{
              margin: 0,
              fontSize: '0.6875rem',
              fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, monospace',
              color: 'var(--text-secondary)',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
            }}>
              {formatJSON(event.tool_input!)}
            </pre>
          </div>
        )}
      </div>
    );
  };

  const renderToolResult = (event: StreamEvent, idx: number) => {
    const isExpanded = toolStates[idx]?.expanded ?? false;
    const hasOutput = event.tool_output && event.tool_output.length > 0;
    const outputPreview = hasOutput ? truncate(event.tool_output!, 100) : event.content || 'Success';
    const isSuccess = !event.content?.toLowerCase().includes('error');

    return (
      <div
        key={idx}
        style={{
          background: isSuccess ? 'rgba(16, 185, 129, 0.08)' : 'rgba(239, 68, 68, 0.08)',
          borderRadius: '8px',
          overflow: 'hidden',
          borderLeft: `3px solid ${isSuccess ? 'var(--success)' : 'var(--error)'}`,
        }}
      >
        <div
          style={{
            padding: '0.625rem 0.875rem',
            display: 'flex',
            alignItems: 'center',
            gap: '0.5rem',
            cursor: hasOutput ? 'pointer' : 'default',
          }}
          onClick={() => hasOutput && toggleToolExpand(idx)}
        >
          <span style={{ fontSize: '0.875rem', flexShrink: 0 }}>
            {isSuccess ? '✓' : '✗'}
          </span>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{
              fontSize: '0.75rem',
              color: isSuccess ? 'var(--success)' : 'var(--error)',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}>
              {!isExpanded ? outputPreview : 'Result'}
            </div>
          </div>
          {event.duration_ms && (
            <span style={{ fontSize: '0.625rem', color: 'var(--text-muted)', flexShrink: 0 }}>
              {event.duration_ms}ms
            </span>
          )}
          {hasOutput && (
            <span style={{
              fontSize: '0.75rem',
              color: 'var(--text-muted)',
              transform: isExpanded ? 'rotate(180deg)' : 'rotate(0deg)',
              transition: 'transform 0.2s',
              flexShrink: 0,
            }}>
              ▼
            </span>
          )}
        </div>

        {isExpanded && hasOutput && (
          <div style={{
            padding: '0.75rem',
            borderTop: '1px solid var(--border)',
            background: 'var(--bg-primary)',
            maxHeight: '300px',
            overflow: 'auto',
          }}>
            <pre style={{
              margin: 0,
              fontSize: '0.6875rem',
              fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, monospace',
              color: 'var(--text-secondary)',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
            }}>
              {formatJSON(event.tool_output!)}
            </pre>
          </div>
        )}
      </div>
    );
  };

  const renderReasoning = (event: StreamEvent, idx: number) => (
    <div
      key={idx}
      style={{
        padding: '0.625rem 0.875rem',
        background: 'rgba(139, 92, 246, 0.08)',
        borderRadius: '8px',
        borderLeft: '3px solid #8B5CF6',
        display: 'flex',
        gap: '0.5rem',
      }}
    >
      <span style={{ flexShrink: 0 }}>💭</span>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{
          fontSize: '0.8125rem',
          color: 'var(--text-secondary)',
          fontStyle: 'italic',
          lineHeight: 1.5,
        }}>
          {event.content}
        </div>
      </div>
      <span style={{ fontSize: '0.625rem', color: 'var(--text-muted)', flexShrink: 0 }}>
        {formatTime(event.timestamp)}
      </span>
    </div>
  );

  const renderPlan = (event: StreamEvent, idx: number) => {
    const [expanded, setExpanded] = useState(false);
    const lines = event.content.split('\n').filter(l => l.trim());
    const preview = lines.slice(0, 3);
    const hasMore = lines.length > 3;

    return (
      <div
        key={idx}
        style={{
          background: 'rgba(59, 130, 246, 0.08)',
          borderRadius: '8px',
          borderLeft: '3px solid #3B82F6',
          overflow: 'hidden',
        }}
      >
        <div
          style={{
            padding: '0.625rem 0.875rem',
            cursor: hasMore ? 'pointer' : 'default',
          }}
          onClick={() => hasMore && setExpanded(!expanded)}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '0.5rem' }}>
            <span>📋</span>
            <span style={{ fontWeight: 600, fontSize: '0.8125rem', color: '#3B82F6' }}>Execution Plan</span>
            {hasMore && (
              <span style={{
                fontSize: '0.625rem',
                color: 'var(--text-muted)',
                marginLeft: 'auto',
              }}>
                {expanded ? '▲ Collapse' : `▼ +${lines.length - 3} more`}
              </span>
            )}
          </div>
          <div style={{ fontSize: '0.8125rem', color: 'var(--text-secondary)' }}>
            {(expanded ? lines : preview).map((line, i) => (
              <div key={i} style={{ padding: '0.125rem 0' }}>{line}</div>
            ))}
          </div>
        </div>
      </div>
    );
  };

  const renderStatus = (event: StreamEvent, idx: number) => {
    const statusIcons: Record<string, string> = {
      starting: '🚀',
      running: '⏳',
      complete: '✅',
      stopped: '⏹',
      cancelled: '🚫',
    };

    return (
      <div
        key={idx}
        style={{
          padding: '0.5rem 0.75rem',
          display: 'flex',
          alignItems: 'center',
          gap: '0.5rem',
          fontSize: '0.75rem',
          color: 'var(--text-muted)',
        }}
      >
        {event.status === 'starting' && isRunning ? (
          <span class="spinner" style={{ width: 12, height: 12 }} />
        ) : (
          <span>{statusIcons[event.status || ''] || '•'}</span>
        )}
        <span>{event.content}</span>
        <span style={{ marginLeft: 'auto', fontSize: '0.625rem' }}>{formatTime(event.timestamp)}</span>
      </div>
    );
  };

  const renderTextGroup = (group: StreamEvent[], idx: number) => {
    const combined = group.map(e => e.content).join('\n');
    const lines = combined.split('\n');
    const [expanded, setExpanded] = useState(lines.length <= 10);

    return (
      <div
        key={idx}
        style={{
          background: 'var(--bg-hover)',
          borderRadius: '6px',
          overflow: 'hidden',
        }}
      >
        <pre style={{
          margin: 0,
          padding: '0.625rem 0.875rem',
          fontSize: '0.75rem',
          fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, monospace',
          color: 'var(--text-secondary)',
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
          maxHeight: expanded ? 'none' : '120px',
          overflow: expanded ? 'visible' : 'hidden',
          lineHeight: 1.5,
        }}>
          {combined}
        </pre>
        {lines.length > 10 && (
          <div
            style={{
              padding: '0.375rem 0.875rem',
              borderTop: '1px solid var(--border)',
              fontSize: '0.6875rem',
              color: 'var(--text-muted)',
              cursor: 'pointer',
              textAlign: 'center',
            }}
            onClick={() => setExpanded(!expanded)}
          >
            {expanded ? '▲ Collapse' : `▼ Show all ${lines.length} lines`}
          </div>
        )}
      </div>
    );
  };

  // Render agent output - the actual response from the AI
  const renderOutput = (event: StreamEvent, idx: number) => (
    <div
      key={idx}
      style={{
        padding: '0.875rem',
        background: 'linear-gradient(135deg, rgba(99, 102, 241, 0.08), rgba(139, 92, 246, 0.05))',
        borderRadius: '8px',
        borderLeft: '3px solid #6366F1',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '0.5rem' }}>
        <span style={{
          width: '24px',
          height: '24px',
          borderRadius: '6px',
          background: 'linear-gradient(135deg, #6366F1, #8B5CF6)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontSize: '0.75rem',
        }}>
          🤖
        </span>
        <span style={{ fontWeight: 600, color: '#A78BFA', fontSize: '0.8125rem' }}>Agent Response</span>
        <span style={{ marginLeft: 'auto', fontSize: '0.625rem', color: 'var(--text-muted)' }}>
          {formatTime(event.timestamp)}
        </span>
      </div>
      <div style={{
        fontSize: '0.875rem',
        color: 'var(--text-primary)',
        lineHeight: 1.6,
        whiteSpace: 'pre-wrap',
        wordBreak: 'break-word',
      }}>
        {event.content}
      </div>
    </div>
  );

  const renderError = (event: StreamEvent, idx: number) => (
    <div
      key={idx}
      style={{
        padding: '0.75rem',
        background: 'rgba(239, 68, 68, 0.1)',
        borderRadius: '8px',
        borderLeft: '3px solid var(--error)',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '0.25rem' }}>
        <span>❌</span>
        <span style={{ fontWeight: 600, color: 'var(--error)', fontSize: '0.8125rem' }}>Error</span>
      </div>
      <div style={{
        fontSize: '0.8125rem',
        color: 'var(--error)',
        whiteSpace: 'pre-wrap',
        wordBreak: 'break-word',
      }}>
        {event.content}
      </div>
    </div>
  );

  const renderDone = (event: StreamEvent, idx: number) => (
    <div
      key={idx}
      style={{
        padding: '0.875rem',
        background: 'linear-gradient(135deg, rgba(16, 185, 129, 0.1), rgba(16, 185, 129, 0.05))',
        borderRadius: '8px',
        border: '1px solid rgba(16, 185, 129, 0.3)',
        textAlign: 'center',
      }}
    >
      <div style={{ fontSize: '1.5rem', marginBottom: '0.375rem' }}>🎉</div>
      <div style={{ fontWeight: 600, color: 'var(--success)', fontSize: '0.9375rem' }}>Execution Complete</div>
      {event.content && (
        <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: '0.25rem' }}>
          {event.content}
        </div>
      )}
    </div>
  );

  const renderEvent = (event: StreamEvent | StreamEvent[], idx: number) => {
    if (Array.isArray(event)) {
      return renderTextGroup(event, idx);
    }

    switch (event.type) {
      case 'tool_call':
        return renderToolCall(event, idx);
      case 'tool_result':
        return renderToolResult(event, idx);
      case 'reasoning':
      case 'thinking':
        return renderReasoning(event, idx);
      case 'plan':
        return renderPlan(event, idx);
      case 'status':
        return renderStatus(event, idx);
      case 'error':
        return renderError(event, idx);
      case 'done':
        return renderDone(event, idx);
      case 'output':
        // Agent output - render prominently
        return renderOutput(event, idx);
      case 'text':
        // CLI text - render as collapsed group
        return renderTextGroup([event], idx);
      default:
        return null;
    }
  };

  // Log view - shows raw events in a table
  const renderLogView = () => (
    <div style={{
      fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, monospace',
      fontSize: '0.6875rem',
    }}>
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr style={{ borderBottom: '1px solid var(--border)', position: 'sticky', top: 0, background: 'var(--bg-secondary)' }}>
            <th style={{ padding: '0.5rem', textAlign: 'left', width: '70px', color: 'var(--text-muted)' }}>Time</th>
            <th style={{ padding: '0.5rem', textAlign: 'left', width: '80px', color: 'var(--text-muted)' }}>Type</th>
            <th style={{ padding: '0.5rem', textAlign: 'left', color: 'var(--text-muted)' }}>Content</th>
          </tr>
        </thead>
        <tbody>
          {events.map((event, idx) => (
            <tr
              key={idx}
              style={{
                borderBottom: '1px solid var(--border)',
                background: event.type === 'error' ? 'rgba(239, 68, 68, 0.05)' : undefined,
              }}
            >
              <td style={{ padding: '0.375rem 0.5rem', color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
                {formatTime(event.timestamp)}
              </td>
              <td style={{ padding: '0.375rem 0.5rem' }}>
                <span style={{
                  padding: '0.125rem 0.375rem',
                  borderRadius: '4px',
                  fontSize: '0.625rem',
                  background:
                    event.type === 'tool_call' ? 'rgba(139, 92, 246, 0.2)' :
                    event.type === 'tool_result' ? 'rgba(16, 185, 129, 0.2)' :
                    event.type === 'error' ? 'rgba(239, 68, 68, 0.2)' :
                    event.type === 'reasoning' ? 'rgba(139, 92, 246, 0.15)' :
                    'var(--bg-hover)',
                  color:
                    event.type === 'tool_call' ? '#A78BFA' :
                    event.type === 'tool_result' ? 'var(--success)' :
                    event.type === 'error' ? 'var(--error)' :
                    event.type === 'reasoning' ? '#C4B5FD' :
                    'var(--text-secondary)',
                }}>
                  {event.type}
                </span>
              </td>
              <td style={{
                padding: '0.375rem 0.5rem',
                color: 'var(--text-secondary)',
                maxWidth: '400px',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}>
                {event.tool_name ? `[${event.tool_name}] ` : ''}
                {truncate(event.content || event.tool_input || event.tool_output || '', 100)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      {/* Header */}
      <div style={{
        padding: '0.5rem 0.75rem',
        borderBottom: '1px solid var(--border)',
        display: 'flex',
        alignItems: 'center',
        gap: '0.5rem',
        flexShrink: 0,
      }}>
        {/* View mode toggle */}
        <div style={{ display: 'flex', borderRadius: '6px', overflow: 'hidden', border: '1px solid var(--border)' }}>
          <button
            style={{
              padding: '0.25rem 0.625rem',
              fontSize: '0.6875rem',
              border: 'none',
              background: viewMode === 'stream' ? 'var(--accent)' : 'transparent',
              color: viewMode === 'stream' ? 'white' : 'var(--text-secondary)',
              cursor: 'pointer',
            }}
            onClick={() => setViewMode('stream')}
          >
            Stream
          </button>
          <button
            style={{
              padding: '0.25rem 0.625rem',
              fontSize: '0.6875rem',
              border: 'none',
              borderLeft: '1px solid var(--border)',
              background: viewMode === 'logs' ? 'var(--accent)' : 'transparent',
              color: viewMode === 'logs' ? 'white' : 'var(--text-secondary)',
              cursor: 'pointer',
            }}
            onClick={() => setViewMode('logs')}
          >
            Logs
          </button>
        </div>

        <div style={{ flex: 1 }} />

        {/* Auto-scroll toggle */}
        <label style={{
          display: 'flex',
          alignItems: 'center',
          gap: '0.375rem',
          fontSize: '0.6875rem',
          color: 'var(--text-muted)',
          cursor: 'pointer',
        }}>
          <input
            type="checkbox"
            checked={autoScroll}
            onChange={(e) => setAutoScroll((e.target as HTMLInputElement).checked)}
            style={{ width: '12px', height: '12px' }}
          />
          Auto-scroll
        </label>

        {/* Event count */}
        <span style={{ fontSize: '0.6875rem', color: 'var(--text-muted)' }}>
          {events.length} events
        </span>

        {onClear && (
          <button
            style={{
              padding: '0.25rem 0.5rem',
              fontSize: '0.6875rem',
              border: '1px solid var(--border)',
              borderRadius: '4px',
              background: 'transparent',
              color: 'var(--text-secondary)',
              cursor: 'pointer',
            }}
            onClick={onClear}
          >
            Clear
          </button>
        )}
      </div>

      {/* Content */}
      <div
        ref={containerRef}
        style={{
          flex: 1,
          overflow: 'auto',
          padding: viewMode === 'stream' ? '0.75rem' : 0,
        }}
      >
        {events.length === 0 ? (
          <div style={{
            textAlign: 'center',
            padding: '3rem',
            color: 'var(--text-muted)',
          }}>
            <div style={{ fontSize: '2rem', marginBottom: '0.5rem', opacity: 0.5 }}>📡</div>
            <div style={{ fontSize: '0.875rem' }}>Waiting for events...</div>
          </div>
        ) : viewMode === 'stream' ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
            {groupedEvents.map((event, idx) => renderEvent(event, idx))}
            <div ref={endRef} />
          </div>
        ) : (
          renderLogView()
        )}
      </div>

      {/* Running indicator */}
      {isRunning && (
        <div style={{
          padding: '0.5rem 0.75rem',
          borderTop: '1px solid var(--border)',
          display: 'flex',
          alignItems: 'center',
          gap: '0.5rem',
          fontSize: '0.75rem',
          color: 'var(--accent)',
          background: 'rgba(99, 102, 241, 0.05)',
          flexShrink: 0,
        }}>
          <span class="spinner" style={{ width: 12, height: 12 }} />
          <span>Execution in progress...</span>
        </div>
      )}
    </div>
  );
}
