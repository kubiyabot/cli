import { h, Fragment } from 'preact';
import { useState, useMemo, useCallback, memo } from 'preact/compat';
import type { Session } from '../App';
import { ChatSession } from './ChatSession';

interface SessionsProps {
  sessions: Session[];
}

interface SessionRowProps {
  session: Session;
  onContinue: (agentId?: string) => void;
}

// Memoized session row to prevent unnecessary re-renders
const SessionRow = memo(function SessionRow({ session, onContinue }: SessionRowProps) {
  const formatTime = (timestamp: string) => {
    if (!timestamp) return 'N/A';
    const date = new Date(timestamp);
    return date.toLocaleString();
  };

  const getStatusBadge = (status: string) => {
    switch (status.toLowerCase()) {
      case 'active':
        return <span class="badge badge-success">Active</span>;
      case 'completed':
        return <span class="badge badge-info">Completed</span>;
      case 'failed':
        return <span class="badge badge-error">Failed</span>;
      case 'cancelled':
        return <span class="badge badge-warning">Cancelled</span>;
      default:
        return <span class="badge">{status}</span>;
    }
  };

  const getTypeBadge = (type: string) => {
    switch (type.toLowerCase()) {
      case 'chat':
        return <span class="badge badge-info">💬 Chat</span>;
      case 'streaming':
        return <span class="badge badge-warning">📡 Streaming</span>;
      case 'execution':
        return <span class="badge badge-success">⚡ Execution</span>;
      default:
        return <span class="badge">{type}</span>;
    }
  };

  return (
    <tr>
      <td style={{ fontFamily: 'monospace', fontSize: '0.8125rem' }}>
        {session.id.slice(0, 8)}...
      </td>
      <td>{getTypeBadge(session.type)}</td>
      <td>{getStatusBadge(session.status)}</td>
      <td>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.375rem' }}>
          <span style={{ fontSize: '0.875rem' }}>🤖</span>
          <span style={{ fontSize: '0.8125rem' }}>
            {session.agent_name || session.agent_id?.slice(0, 8) || '-'}
          </span>
        </div>
      </td>
      <td style={{ fontSize: '0.8125rem' }}>{formatTime(session.started_at)}</td>
      <td>{session.duration_str || '-'}</td>
      <td>
        <span style={{
          background: 'var(--bg-hover)',
          padding: '0.125rem 0.5rem',
          borderRadius: '12px',
          fontSize: '0.75rem',
        }}>
          {session.messages_count}
        </span>
      </td>
      <td>
        <div style={{ display: 'flex', gap: '0.375rem' }}>
          {session.status === 'active' && session.agent_id && (
            <button
              class="btn btn-secondary"
              style={{ padding: '0.25rem 0.5rem', fontSize: '0.6875rem' }}
              onClick={() => onContinue(session.agent_id)}
              title="Continue conversation"
            >
              💬
            </button>
          )}
          <button
            class="btn btn-secondary"
            style={{ padding: '0.25rem 0.5rem', fontSize: '0.6875rem' }}
            title="View details"
          >
            👁
          </button>
        </div>
      </td>
    </tr>
  );
});

export function Sessions({ sessions }: SessionsProps) {
  const [showChat, setShowChat] = useState(false);
  const [selectedAgentId, setSelectedAgentId] = useState<string | undefined>(undefined);
  const [viewMode, setViewMode] = useState<'list' | 'chat'>('list');

  // Memoize computed stats
  const stats = useMemo(() => ({
    active: sessions.filter((s) => s.status === 'active').length,
    completed: sessions.filter((s) => s.status === 'completed').length,
    messages: sessions.reduce((sum, s) => sum + s.messages_count, 0),
    tokens: sessions.reduce((sum, s) => sum + (s.tokens_used || 0), 0),
  }), [sessions]);

  const startNewChat = useCallback((agentId?: string) => {
    setSelectedAgentId(agentId);
    setShowChat(true);
    setViewMode('chat');
  }, []);

  const closeChat = useCallback(() => {
    setShowChat(false);
    setSelectedAgentId(undefined);
    setViewMode('list');
  }, []);

  // Full screen chat view
  if (viewMode === 'chat' && showChat) {
    return (
      <div style={{ height: 'calc(100vh - 140px)' }}>
        <ChatSession onClose={closeChat} initialAgentId={selectedAgentId} />
      </div>
    );
  }

  return (
    <Fragment>
      {/* Header */}
      <div style={{
        marginBottom: '1rem',
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
      }}>
        <div>
          <h2 style={{ fontSize: '1.25rem', fontWeight: 600, marginBottom: '0.125rem' }}>
            Sessions
          </h2>
          <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
            {sessions.length} total sessions
          </div>
        </div>
        <button
          class="btn btn-primary"
          onClick={() => startNewChat()}
          style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}
        >
          <span style={{ fontSize: '1.125rem' }}>💬</span>
          New Conversation
        </button>
      </div>

      {/* Quick Actions */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
        gap: '0.75rem',
        marginBottom: '1.5rem',
      }}>
        <div
          style={{
            padding: '1rem',
            background: 'linear-gradient(135deg, rgba(99, 102, 241, 0.15), rgba(139, 92, 246, 0.1))',
            borderRadius: '12px',
            border: '1px solid rgba(99, 102, 241, 0.3)',
            cursor: 'pointer',
            transition: 'all 0.2s',
          }}
          onClick={() => startNewChat()}
          onMouseOver={(e) => {
            (e.currentTarget as HTMLElement).style.transform = 'translateY(-2px)';
            (e.currentTarget as HTMLElement).style.boxShadow = '0 4px 12px rgba(99, 102, 241, 0.2)';
          }}
          onMouseOut={(e) => {
            (e.currentTarget as HTMLElement).style.transform = 'translateY(0)';
            (e.currentTarget as HTMLElement).style.boxShadow = 'none';
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
            <div style={{
              width: '48px',
              height: '48px',
              borderRadius: '12px',
              background: 'linear-gradient(135deg, #6366F1, #8B5CF6)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: '1.5rem',
            }}>
              💬
            </div>
            <div>
              <div style={{ fontWeight: 600, fontSize: '0.9375rem' }}>Start Chat</div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                Talk to an AI agent
              </div>
            </div>
          </div>
        </div>

        <div
          style={{
            padding: '1rem',
            background: 'linear-gradient(135deg, rgba(16, 185, 129, 0.15), rgba(52, 211, 153, 0.1))',
            borderRadius: '12px',
            border: '1px solid rgba(16, 185, 129, 0.3)',
            cursor: 'pointer',
            transition: 'all 0.2s',
          }}
          onClick={() => window.location.hash = '#playground'}
          onMouseOver={(e) => {
            (e.currentTarget as HTMLElement).style.transform = 'translateY(-2px)';
            (e.currentTarget as HTMLElement).style.boxShadow = '0 4px 12px rgba(16, 185, 129, 0.2)';
          }}
          onMouseOut={(e) => {
            (e.currentTarget as HTMLElement).style.transform = 'translateY(0)';
            (e.currentTarget as HTMLElement).style.boxShadow = 'none';
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
            <div style={{
              width: '48px',
              height: '48px',
              borderRadius: '12px',
              background: 'linear-gradient(135deg, #10B981, #34D399)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: '1.5rem',
            }}>
              🚀
            </div>
            <div>
              <div style={{ fontWeight: 600, fontSize: '0.9375rem' }}>Execute Task</div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                Run a one-shot execution
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Session Stats */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(4, 1fr)',
        gap: '0.75rem',
        marginBottom: '1.5rem',
      }}>
        <div class="stat-card">
          <div class="stat-label">Active</div>
          <div class="stat-value success">{stats.active}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Completed</div>
          <div class="stat-value">{stats.completed}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Messages</div>
          <div class="stat-value">{stats.messages}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Tokens</div>
          <div class="stat-value">{stats.tokens.toLocaleString()}</div>
        </div>
      </div>

      {/* Sessions List */}
      {sessions.length === 0 ? (
        <div class="card">
          <div class="empty-state">
            <div class="empty-state-icon">💬</div>
            <div>No sessions yet</div>
            <div style={{ fontSize: '0.875rem', color: 'var(--text-muted)', marginTop: '0.5rem' }}>
              Start a new conversation to begin
            </div>
            <button
              class="btn btn-primary"
              style={{ marginTop: '1rem' }}
              onClick={() => startNewChat()}
            >
              Start Conversation
            </button>
          </div>
        </div>
      ) : (
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">Recent Sessions</h3>
          </div>
          <table class="table">
            <thead>
              <tr>
                <th>Session</th>
                <th>Type</th>
                <th>Status</th>
                <th>Agent</th>
                <th>Started</th>
                <th>Duration</th>
                <th>Messages</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {sessions.map((session) => (
                <SessionRow
                  key={session.id}
                  session={session}
                  onContinue={startNewChat}
                />
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Chat Modal/Overlay */}
      {showChat && viewMode === 'list' && (
        <div style={{
          position: 'fixed',
          inset: 0,
          background: 'rgba(0, 0, 0, 0.5)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          zIndex: 1000,
          padding: '2rem',
        }}>
          <div style={{
            width: '100%',
            maxWidth: '700px',
            height: '80vh',
            maxHeight: '700px',
          }}>
            <ChatSession onClose={closeChat} initialAgentId={selectedAgentId} />
          </div>
        </div>
      )}
    </Fragment>
  );
}
