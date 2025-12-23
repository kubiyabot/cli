import { h, Fragment } from 'preact';
import { useState, useEffect } from 'preact/hooks';

interface DiagnosticCheck {
  name: string;
  category: string;
  status: 'pass' | 'fail' | 'warning' | 'skip';
  message: string;
  details?: unknown;
  duration_ms: number;
  remediation?: string;
}

interface DiagnosticSummary {
  total: number;
  passed: number;
  failed: number;
  warnings: number;
  skipped: number;
}

interface DiagnosticsReport {
  timestamp: string;
  overall: 'healthy' | 'degraded' | 'unhealthy';
  checks: DiagnosticCheck[];
  summary: DiagnosticSummary;
}

export function Doctor() {
  const [report, setReport] = useState<DiagnosticsReport | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [expandedChecks, setExpandedChecks] = useState<Set<string>>(new Set());

  const runDiagnostics = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch('/api/doctor');
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}`);
      }
      const data = await res.json();
      setReport(data);
    } catch (err) {
      setError(`Failed to run diagnostics: ${err}`);
    } finally {
      setLoading(false);
    }
  };

  // Run diagnostics on mount
  useEffect(() => {
    runDiagnostics();
  }, []);

  const toggleExpanded = (name: string) => {
    const newSet = new Set(expandedChecks);
    if (newSet.has(name)) {
      newSet.delete(name);
    } else {
      newSet.add(name);
    }
    setExpandedChecks(newSet);
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'pass':
        return '✓';
      case 'fail':
        return '✗';
      case 'warning':
        return '⚠';
      case 'skip':
        return '○';
      default:
        return '?';
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'pass':
        return 'var(--success)';
      case 'fail':
        return 'var(--error)';
      case 'warning':
        return 'var(--warning)';
      case 'skip':
        return 'var(--text-muted)';
      default:
        return 'var(--text-secondary)';
    }
  };

  const getOverallColor = (overall: string) => {
    switch (overall) {
      case 'healthy':
        return 'var(--success)';
      case 'degraded':
        return 'var(--warning)';
      case 'unhealthy':
        return 'var(--error)';
      default:
        return 'var(--text-secondary)';
    }
  };

  const getCategoryIcon = (category: string) => {
    switch (category) {
      case 'python':
        return '🐍';
      case 'packages':
        return '📦';
      case 'connectivity':
        return '🌐';
      case 'config':
        return '⚙️';
      case 'process':
        return '🔧';
      default:
        return '📋';
    }
  };

  const groupByCategory = (checks: DiagnosticCheck[]) => {
    const groups: Record<string, DiagnosticCheck[]> = {};
    for (const check of checks) {
      if (!groups[check.category]) {
        groups[check.category] = [];
      }
      groups[check.category].push(check);
    }
    return groups;
  };

  const formatTimestamp = (ts: string) => {
    const date = new Date(ts);
    return date.toLocaleString();
  };

  if (loading && !report) {
    return (
      <div class="card">
        <div style={{ textAlign: 'center', padding: '3rem' }}>
          <span class="spinner" style={{ width: '32px', height: '32px' }} />
          <div style={{ marginTop: '1rem', color: 'var(--text-secondary)' }}>
            Running diagnostics...
          </div>
        </div>
      </div>
    );
  }

  if (error && !report) {
    return (
      <div class="card" style={{ borderColor: 'var(--error)' }}>
        <div style={{ textAlign: 'center', padding: '2rem' }}>
          <div style={{ fontSize: '2rem', marginBottom: '1rem' }}>❌</div>
          <div style={{ color: 'var(--error)', marginBottom: '1rem' }}>{error}</div>
          <button class="btn btn-primary" onClick={runDiagnostics}>
            Retry
          </button>
        </div>
      </div>
    );
  }

  const categoryGroups = report ? groupByCategory(report.checks) : {};

  return (
    <Fragment>
      {/* Header */}
      <div style={{ marginBottom: '1rem', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2 style={{ fontSize: '1.25rem', fontWeight: 600 }}>System Diagnostics</h2>
        <button
          class="btn btn-primary"
          onClick={runDiagnostics}
          disabled={loading}
        >
          {loading ? (
            <Fragment>
              <span class="spinner" style={{ width: 14, height: 14 }} />
              Running...
            </Fragment>
          ) : (
            '🔄 Run Diagnostics'
          )}
        </button>
      </div>

      {report && (
        <Fragment>
          {/* Overall Status Card */}
          <div
            class="card"
            style={{
              marginBottom: '1rem',
              borderColor: getOverallColor(report.overall),
              background: `${getOverallColor(report.overall)}15`,
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
                <div
                  style={{
                    width: '48px',
                    height: '48px',
                    borderRadius: '50%',
                    background: getOverallColor(report.overall),
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: '1.5rem',
                  }}
                >
                  {report.overall === 'healthy' ? '✓' : report.overall === 'degraded' ? '⚠' : '✗'}
                </div>
                <div>
                  <div style={{ fontSize: '1.25rem', fontWeight: 600, textTransform: 'capitalize' }}>
                    {report.overall}
                  </div>
                  <div style={{ color: 'var(--text-secondary)', fontSize: '0.875rem' }}>
                    Last checked: {formatTimestamp(report.timestamp)}
                  </div>
                </div>
              </div>
              <div style={{ display: 'flex', gap: '1.5rem', textAlign: 'center' }}>
                <div>
                  <div style={{ fontSize: '1.5rem', fontWeight: 600, color: 'var(--success)' }}>
                    {report.summary.passed}
                  </div>
                  <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>Passed</div>
                </div>
                <div>
                  <div style={{ fontSize: '1.5rem', fontWeight: 600, color: 'var(--error)' }}>
                    {report.summary.failed}
                  </div>
                  <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>Failed</div>
                </div>
                <div>
                  <div style={{ fontSize: '1.5rem', fontWeight: 600, color: 'var(--warning)' }}>
                    {report.summary.warnings}
                  </div>
                  <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>Warnings</div>
                </div>
                {report.summary.skipped > 0 && (
                  <div>
                    <div style={{ fontSize: '1.5rem', fontWeight: 600, color: 'var(--text-muted)' }}>
                      {report.summary.skipped}
                    </div>
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>Skipped</div>
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* Category Cards */}
          <div style={{ display: 'grid', gap: '1rem' }}>
            {Object.entries(categoryGroups).map(([category, checks]) => {
              const failedCount = checks.filter((c) => c.status === 'fail').length;
              const warningCount = checks.filter((c) => c.status === 'warning').length;
              const categoryStatus =
                failedCount > 0 ? 'fail' : warningCount > 0 ? 'warning' : 'pass';

              return (
                <div class="card" key={category}>
                  <div class="card-header">
                    <h3 class="card-title" style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                      <span>{getCategoryIcon(category)}</span>
                      <span style={{ textTransform: 'capitalize' }}>{category}</span>
                      <span
                        class={`badge ${
                          categoryStatus === 'pass'
                            ? 'badge-success'
                            : categoryStatus === 'warning'
                            ? 'badge-warning'
                            : 'badge-error'
                        }`}
                        style={{ marginLeft: '0.5rem' }}
                      >
                        {checks.filter((c) => c.status === 'pass').length}/{checks.length}
                      </span>
                    </h3>
                  </div>

                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
                    {checks.map((check) => (
                      <div
                        key={check.name}
                        style={{
                          padding: '0.75rem',
                          background:
                            check.status === 'fail'
                              ? 'rgba(255, 107, 107, 0.1)'
                              : check.status === 'warning'
                              ? 'rgba(255, 193, 7, 0.1)'
                              : 'var(--bg-hover)',
                          borderRadius: '6px',
                          cursor: check.remediation || check.details ? 'pointer' : 'default',
                        }}
                        onClick={() => (check.remediation || check.details) && toggleExpanded(check.name)}
                      >
                        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                            <span
                              style={{
                                color: getStatusColor(check.status),
                                fontWeight: 600,
                                fontSize: '1rem',
                              }}
                            >
                              {getStatusIcon(check.status)}
                            </span>
                            <div>
                              <div style={{ fontWeight: 500 }}>{check.name}</div>
                              <div style={{ fontSize: '0.8125rem', color: 'var(--text-secondary)' }}>
                                {check.message}
                              </div>
                            </div>
                          </div>
                          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                            <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                              {check.duration_ms}ms
                            </span>
                            {(check.remediation || check.details) && (
                              <span style={{ color: 'var(--text-muted)' }}>
                                {expandedChecks.has(check.name) ? '▼' : '▶'}
                              </span>
                            )}
                          </div>
                        </div>

                        {/* Expanded details */}
                        {expandedChecks.has(check.name) && (
                          <div style={{ marginTop: '0.75rem', paddingTop: '0.75rem', borderTop: '1px solid var(--border)' }}>
                            {check.remediation && (
                              <div style={{ marginBottom: '0.5rem' }}>
                                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginBottom: '0.25rem' }}>
                                  Remediation:
                                </div>
                                <div
                                  style={{
                                    fontSize: '0.8125rem',
                                    padding: '0.5rem',
                                    background: 'var(--bg-primary)',
                                    borderRadius: '4px',
                                    fontFamily: 'monospace',
                                    whiteSpace: 'pre-wrap',
                                  }}
                                >
                                  {check.remediation}
                                </div>
                              </div>
                            )}
                            {check.details && (
                              <div>
                                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginBottom: '0.25rem' }}>
                                  Details:
                                </div>
                                <pre
                                  style={{
                                    fontSize: '0.75rem',
                                    padding: '0.5rem',
                                    background: 'var(--bg-primary)',
                                    borderRadius: '4px',
                                    overflow: 'auto',
                                    margin: 0,
                                  }}
                                >
                                  {JSON.stringify(check.details, null, 2)}
                                </pre>
                              </div>
                            )}
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              );
            })}
          </div>

          {/* Quick Actions for Failures */}
          {report.summary.failed > 0 && (
            <div class="card" style={{ marginTop: '1rem' }}>
              <div class="card-header">
                <h3 class="card-title">Suggested Actions</h3>
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
                {report.checks
                  .filter((c) => c.status === 'fail' && c.remediation)
                  .map((check) => (
                    <div
                      key={check.name}
                      style={{
                        padding: '0.75rem',
                        background: 'var(--bg-hover)',
                        borderRadius: '6px',
                        borderLeft: '3px solid var(--error)',
                      }}
                    >
                      <div style={{ fontWeight: 500, marginBottom: '0.25rem' }}>{check.name}</div>
                      <div
                        style={{
                          fontSize: '0.8125rem',
                          fontFamily: 'monospace',
                          color: 'var(--text-secondary)',
                        }}
                      >
                        {check.remediation}
                      </div>
                    </div>
                  ))}
              </div>
            </div>
          )}
        </Fragment>
      )}
    </Fragment>
  );
}
