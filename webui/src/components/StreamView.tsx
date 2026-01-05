import { h, Fragment } from 'preact';
import { useState, useEffect, useRef } from 'preact/hooks';

export interface StreamEvent {
  type: 'text' | 'tool_call' | 'tool_result' | 'reasoning' | 'error' | 'status' | 'plan' | 'done' | 'thinking' | 'output';
  content: string;
  timestamp: string;
  tool_name?: string;
  tool_input?: string;
  tool_output?: string;
  tool_call_id?: string;  // ID to match tool_call with tool_result
  message_id?: string;    // ID to group related events
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

  // Smart grouping for cleaner conversation display:
  // 1. Group consecutive text events (CLI noise)
  // 2. Pair tool_call + tool_result into unified tool cards
  // 3. Accumulate output text between tool interactions into single messages
  type GroupedItem = StreamEvent | { type: 'text_group'; events: StreamEvent[] } | { type: 'tool_pair'; call: StreamEvent; result?: StreamEvent };

  const groupedEvents = events.reduce<GroupedItem[]>((acc, event) => {
    if (event.type === 'text') {
      // Group consecutive text events (CLI noise)
      const last = acc[acc.length - 1];
      if (last && 'type' in last && last.type === 'text_group') {
        (last as { type: 'text_group'; events: StreamEvent[] }).events.push(event);
      } else {
        acc.push({ type: 'text_group', events: [event] });
      }
    } else if (event.type === 'output') {
      // Look for the last output event (even if not immediately previous)
      // and merge with it if there's no tool_call between them
      let lastOutputIdx = -1;
      let hasToolCallBetween = false;

      for (let i = acc.length - 1; i >= 0; i--) {
        const item = acc[i];
        if ('type' in item && item.type === 'output') {
          lastOutputIdx = i;
          break;
        }
        // If we hit a tool_pair or tool_call, don't merge (new thought after tool result)
        if ('type' in item && (item.type === 'tool_pair' || item.type === 'tool_call')) {
          hasToolCallBetween = true;
          break;
        }
      }

      if (lastOutputIdx >= 0 && !hasToolCallBetween) {
        // Merge with previous output event
        const lastOutput = acc[lastOutputIdx] as StreamEvent;
        lastOutput.content = (lastOutput.content || '') + (event.content || '');
      } else {
        // Start a new output event (clone to avoid mutating original)
        acc.push({ ...event });
      }
    } else if (event.type === 'tool_call') {
      // Start a new tool pair
      acc.push({ type: 'tool_pair', call: event });
    } else if (event.type === 'tool_result') {
      // Try to pair tool_result with matching tool_call by tool_call_id
      let paired = false;
      for (let i = acc.length - 1; i >= 0; i--) {
        const item = acc[i];
        if ('type' in item && item.type === 'tool_pair') {
          const pair = item as { type: 'tool_pair'; call: StreamEvent; result?: StreamEvent };
          // Match by tool_call_id if available, otherwise match the last unpaired tool_pair
          if (!pair.result) {
            if (event.tool_call_id && pair.call.tool_call_id) {
              // Match by ID
              if (pair.call.tool_call_id === event.tool_call_id) {
                pair.result = event;
                paired = true;
                break;
              }
            } else {
              // Fallback: match last unpaired
              pair.result = event;
              paired = true;
              break;
            }
          }
        }
      }
      if (!paired) {
        // Orphan tool_result - just add it
        acc.push(event);
      }
    } else {
      // All other events (status, error, etc.) are not grouped
      acc.push(event);
    }
    return acc;
  }, []);

  // Unified tool call card that shows tool name, input (collapsible), and output (collapsible)
  const renderToolPair = (call: StreamEvent, result: StreamEvent | undefined, idx: number) => {
    const isExpanded = toolStates[idx]?.expanded ?? false;
    const hasInput = call.tool_input && call.tool_input.length > 0;
    const hasOutput = result?.tool_output && result.tool_output.length > 0;
    const isSuccess = result ? !result.content?.toLowerCase().includes('error') : true;
    const isWaiting = !result;

    // Parse input to show a nice preview
    let inputPreview = '';
    if (hasInput) {
      try {
        const parsed = JSON.parse(call.tool_input!);
        // For Bash, show the command
        if (parsed.command) {
          inputPreview = parsed.command.length > 60 ? parsed.command.slice(0, 60) + '...' : parsed.command;
        } else if (parsed.file_path) {
          inputPreview = parsed.file_path;
        } else if (parsed.pattern) {
          inputPreview = parsed.pattern;
        } else {
          inputPreview = truncate(call.tool_input!, 60);
        }
      } catch {
        inputPreview = truncate(call.tool_input!, 60);
      }
    }

    // Parse output for preview
    let outputPreview = '';
    if (hasOutput) {
      outputPreview = truncate(result!.tool_output!, 80);
    }

    return (
      <div
        key={idx}
        style={{
          background: 'var(--bg-hover)',
          borderRadius: '8px',
          overflow: 'hidden',
          border: '1px solid var(--border)',
          borderLeft: result ? `3px solid ${isSuccess ? 'var(--success)' : 'var(--error)'}` : '3px solid var(--accent)',
        }}
      >
        {/* Tool header - always visible */}
        <div
          style={{
            padding: '0.625rem 0.875rem',
            display: 'flex',
            alignItems: 'center',
            gap: '0.5rem',
            cursor: (hasInput || hasOutput) ? 'pointer' : 'default',
            userSelect: 'none',
          }}
          onClick={() => (hasInput || hasOutput) && toggleToolExpand(idx)}
        >
          <span style={{
            width: '24px',
            height: '24px',
            borderRadius: '6px',
            background: isWaiting ? 'var(--accent)' : isSuccess ? 'var(--success)' : 'var(--error)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: '0.75rem',
            flexShrink: 0,
            color: 'white',
          }}>
            {isWaiting ? (
              <span class="spinner spinner-sm" style={{ width: 12, height: 12 }} />
            ) : isSuccess ? '✓' : '✗'}
          </span>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{
              fontWeight: 600,
              fontSize: '0.8125rem',
              color: 'var(--text-primary)',
              display: 'flex',
              alignItems: 'center',
              gap: '0.5rem',
            }}>
              {call.tool_name || 'Tool'}
            </div>
            {!isExpanded && inputPreview && (
              <div style={{
                fontSize: '0.6875rem',
                color: 'var(--text-muted)',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
                fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, monospace',
              }}>
                {inputPreview}
              </div>
            )}
          </div>
          {result?.duration_ms && (
            <span style={{
              fontSize: '0.625rem',
              color: 'var(--text-muted)',
              flexShrink: 0,
              padding: '0.125rem 0.375rem',
              background: 'var(--bg-primary)',
              borderRadius: '4px',
            }}>
              {result.duration_ms}ms
            </span>
          )}
          <span style={{ fontSize: '0.625rem', color: 'var(--text-muted)', flexShrink: 0 }}>
            {formatTime(call.timestamp)}
          </span>
          {(hasInput || hasOutput) && (
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

        {/* Expanded details */}
        {isExpanded && (
          <div style={{ borderTop: '1px solid var(--border)' }}>
            {/* Input section */}
            {hasInput && (
              <div style={{
                padding: '0.625rem 0.875rem',
                background: 'var(--bg-primary)',
              }}>
                <div style={{
                  fontSize: '0.625rem',
                  color: 'var(--text-muted)',
                  marginBottom: '0.375rem',
                  textTransform: 'uppercase',
                  fontWeight: 600,
                }}>
                  Input
                </div>
                <pre style={{
                  margin: 0,
                  fontSize: '0.6875rem',
                  fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, monospace',
                  color: 'var(--text-secondary)',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-word',
                  maxHeight: '150px',
                  overflow: 'auto',
                }}>
                  {formatJSON(call.tool_input!)}
                </pre>
              </div>
            )}

            {/* Output section */}
            {hasOutput && (
              <div style={{
                padding: '0.625rem 0.875rem',
                background: 'var(--bg-secondary)',
                borderTop: hasInput ? '1px solid var(--border)' : 'none',
              }}>
                <div style={{
                  fontSize: '0.625rem',
                  color: 'var(--text-muted)',
                  marginBottom: '0.375rem',
                  textTransform: 'uppercase',
                  fontWeight: 600,
                }}>
                  Output
                </div>
                <pre style={{
                  margin: 0,
                  fontSize: '0.6875rem',
                  fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, monospace',
                  color: 'var(--text-secondary)',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-word',
                  maxHeight: '200px',
                  overflow: 'auto',
                }}>
                  {formatJSON(result!.tool_output!)}
                </pre>
              </div>
            )}
          </div>
        )}
      </div>
    );
  };

  // Keep legacy renderers for orphaned events
  const renderToolCall = (event: StreamEvent, idx: number) => {
    return renderToolPair(event, undefined, idx);
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

  const renderEvent = (item: GroupedItem, idx: number) => {
    // Check for our special grouped types first
    if ('type' in item) {
      if (item.type === 'text_group') {
        return renderTextGroup((item as { type: 'text_group'; events: StreamEvent[] }).events, idx);
      }
      if (item.type === 'tool_pair') {
        const pair = item as { type: 'tool_pair'; call: StreamEvent; result?: StreamEvent };
        return renderToolPair(pair.call, pair.result, idx);
      }
    }

    // It's a regular StreamEvent
    const event = item as StreamEvent;
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
