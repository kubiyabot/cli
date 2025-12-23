import { h, Fragment } from 'preact';
import { useState, useEffect } from 'preact/hooks';

interface EnvVariable {
  key: string;
  value: string;
  source: 'custom' | 'inherited' | 'system' | 'worker';
  sensitive: boolean;
  editable: boolean;
}

export function Environment() {
  const [variables, setVariables] = useState<EnvVariable[]>([]);
  const [envFilePath, setEnvFilePath] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [searchTerm, setSearchTerm] = useState('');
  const [sourceFilter, setSourceFilter] = useState('');
  const [showAddModal, setShowAddModal] = useState(false);
  const [newKey, setNewKey] = useState('');
  const [newValue, setNewValue] = useState('');
  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [editValue, setEditValue] = useState('');

  const fetchVariables = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch('/api/env');
      const data = await res.json();
      setVariables(data.variables || []);
      setEnvFilePath(data.env_file || '');
    } catch (err) {
      setError(`Failed to fetch environment variables: ${err}`);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchVariables();
  }, []);

  const handleSaveToFile = async () => {
    setSaving(true);
    setError(null);
    setSuccess(null);
    try {
      const res = await fetch('/api/env/save', { method: 'POST' });
      const data = await res.json();
      if (data.success) {
        setSuccess('Environment saved to .env file');
      } else {
        setError(data.message || 'Failed to save');
      }
    } catch (err) {
      setError(`Failed to save: ${err}`);
    } finally {
      setSaving(false);
    }
  };

  const handleReload = async () => {
    setSaving(true);
    setError(null);
    setSuccess(null);
    try {
      const res = await fetch('/api/env/reload', { method: 'POST' });
      const data = await res.json();
      if (data.success) {
        setSuccess('Environment reloaded from .env file');
        fetchVariables();
      } else {
        setError(data.message || 'Failed to reload');
      }
    } catch (err) {
      setError(`Failed to reload: ${err}`);
    } finally {
      setSaving(false);
    }
  };

  const handleAddVariable = async () => {
    if (!newKey.trim()) {
      setError('Key is required');
      return;
    }

    setSaving(true);
    setError(null);
    try {
      const res = await fetch('/api/env', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          variables: { [newKey]: newValue },
          save_to_file: true,
        }),
      });
      const data = await res.json();
      if (data.success) {
        setSuccess('Variable added');
        setNewKey('');
        setNewValue('');
        setShowAddModal(false);
        fetchVariables();
      } else {
        setError(data.message || 'Failed to add variable');
      }
    } catch (err) {
      setError(`Failed to add: ${err}`);
    } finally {
      setSaving(false);
    }
  };

  const handleUpdateVariable = async (key: string, value: string) => {
    setSaving(true);
    setError(null);
    try {
      const res = await fetch('/api/env', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          variables: { [key]: value },
          save_to_file: true,
        }),
      });
      const data = await res.json();
      if (data.success) {
        setSuccess(`Updated ${key}`);
        setEditingKey(null);
        fetchVariables();
      } else {
        setError(data.message || 'Failed to update');
      }
    } catch (err) {
      setError(`Failed to update: ${err}`);
    } finally {
      setSaving(false);
    }
  };

  const startEdit = (variable: EnvVariable) => {
    if (!variable.editable) return;
    setEditingKey(variable.key);
    setEditValue(variable.value);
  };

  const cancelEdit = () => {
    setEditingKey(null);
    setEditValue('');
  };

  const getSourceBadge = (source: string) => {
    const badges: Record<string, { class: string; label: string }> = {
      custom: { class: 'badge-success', label: 'Custom' },
      worker: { class: 'badge-info', label: 'Worker' },
      inherited: { class: 'badge-warning', label: 'Inherited' },
      system: { class: 'badge', label: 'System' },
    };
    const badge = badges[source] || { class: 'badge', label: source };
    return <span class={`badge ${badge.class}`} style={{ fontSize: '0.625rem' }}>{badge.label}</span>;
  };

  const filteredVariables = variables.filter((v) => {
    if (searchTerm && !v.key.toLowerCase().includes(searchTerm.toLowerCase())) {
      return false;
    }
    if (sourceFilter && v.source !== sourceFilter) {
      return false;
    }
    return true;
  });

  // Group by source
  const customVars = filteredVariables.filter((v) => v.source === 'custom' || v.source === 'worker');
  const otherVars = filteredVariables.filter((v) => v.source !== 'custom' && v.source !== 'worker');

  if (loading && variables.length === 0) {
    return (
      <div class="card">
        <div style={{ textAlign: 'center', padding: '3rem' }}>
          <span class="spinner" style={{ width: '32px', height: '32px' }} />
          <div style={{ marginTop: '1rem', color: 'var(--text-secondary)' }}>
            Loading environment variables...
          </div>
        </div>
      </div>
    );
  }

  return (
    <Fragment>
      {/* Header */}
      <div style={{ marginBottom: '1rem', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2 style={{ fontSize: '1.25rem', fontWeight: 600 }}>Environment Variables</h2>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <button class="btn btn-primary" onClick={() => setShowAddModal(true)}>
            + Add Variable
          </button>
          <button class="btn btn-secondary" onClick={handleReload} disabled={saving}>
            ↻ Reload
          </button>
          <button class="btn btn-secondary" onClick={handleSaveToFile} disabled={saving}>
            💾 Save to .env
          </button>
        </div>
      </div>

      {/* Messages */}
      {error && (
        <div class="card" style={{ borderColor: 'var(--error)', marginBottom: '1rem', padding: '0.75rem' }}>
          <div style={{ color: 'var(--error)', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <span>❌</span>
            <span>{error}</span>
            <button
              style={{ marginLeft: 'auto', background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer' }}
              onClick={() => setError(null)}
            >
              ✕
            </button>
          </div>
        </div>
      )}
      {success && (
        <div class="card" style={{ borderColor: 'var(--success)', marginBottom: '1rem', padding: '0.75rem' }}>
          <div style={{ color: 'var(--success)', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <span>✓</span>
            <span>{success}</span>
            <button
              style={{ marginLeft: 'auto', background: 'none', border: 'none', color: 'var(--text-muted)', cursor: 'pointer' }}
              onClick={() => setSuccess(null)}
            >
              ✕
            </button>
          </div>
        </div>
      )}

      {/* Restart Warning */}
      <div class="card" style={{ borderColor: 'var(--warning)', marginBottom: '1rem', padding: '0.75rem', background: 'rgba(255, 193, 7, 0.1)' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', fontSize: '0.875rem' }}>
          <span>⚠️</span>
          <span>Changes to environment variables require worker restart to take effect.</span>
        </div>
      </div>

      {/* File Path */}
      {envFilePath && (
        <div style={{ marginBottom: '1rem', fontSize: '0.8125rem', color: 'var(--text-secondary)' }}>
          <span style={{ fontWeight: 500 }}>.env file:</span>{' '}
          <code style={{ background: 'var(--bg-hover)', padding: '0.125rem 0.375rem', borderRadius: '3px' }}>
            {envFilePath}
          </code>
        </div>
      )}

      {/* Add Variable Modal */}
      {showAddModal && (
        <div class="card" style={{ marginBottom: '1rem' }}>
          <div class="card-header">
            <h3 class="card-title">Add Environment Variable</h3>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
            <div>
              <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>Key</label>
              <input
                type="text"
                class="filter-input"
                style={{ width: '100%' }}
                value={newKey}
                onInput={(e) => setNewKey((e.target as HTMLInputElement).value.toUpperCase().replace(/[^A-Z0-9_]/g, '_'))}
                placeholder="MY_VARIABLE"
              />
            </div>
            <div>
              <label style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>Value</label>
              <input
                type="text"
                class="filter-input"
                style={{ width: '100%' }}
                value={newValue}
                onInput={(e) => setNewValue((e.target as HTMLInputElement).value)}
                placeholder="value"
              />
            </div>
            <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'flex-end' }}>
              <button class="btn btn-secondary" onClick={() => setShowAddModal(false)}>
                Cancel
              </button>
              <button class="btn btn-primary" onClick={handleAddVariable} disabled={saving}>
                {saving ? 'Adding...' : 'Add'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Filters */}
      <div class="filter-bar" style={{ marginBottom: '1rem' }}>
        <input
          type="text"
          class="filter-input"
          placeholder="Search variables..."
          value={searchTerm}
          onInput={(e) => setSearchTerm((e.target as HTMLInputElement).value)}
          style={{ flex: 1 }}
        />
        <select
          class="filter-select"
          value={sourceFilter}
          onChange={(e) => setSourceFilter((e.target as HTMLSelectElement).value)}
        >
          <option value="">All Sources</option>
          <option value="custom">Custom</option>
          <option value="worker">Worker</option>
          <option value="inherited">Inherited</option>
          <option value="system">System</option>
        </select>
        <button
          class="btn btn-secondary"
          onClick={() => { setSearchTerm(''); setSourceFilter(''); }}
          style={{ fontSize: '0.75rem', padding: '0.4rem 0.6rem' }}
        >
          Clear
        </button>
      </div>

      {/* Custom/Worker Variables */}
      {customVars.length > 0 && (
        <div class="card" style={{ marginBottom: '1rem' }}>
          <div class="card-header">
            <h3 class="card-title">Custom & Worker Variables ({customVars.length})</h3>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
            {customVars.map((v) => (
              <div
                key={v.key}
                style={{
                  padding: '0.75rem',
                  background: 'var(--bg-hover)',
                  borderRadius: '6px',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '0.75rem',
                }}
              >
                <div style={{ flex: 1 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '0.25rem' }}>
                    <span style={{ fontFamily: 'monospace', fontWeight: 500 }}>{v.key}</span>
                    {getSourceBadge(v.source)}
                    {v.sensitive && <span class="badge badge-warning" style={{ fontSize: '0.625rem' }}>Sensitive</span>}
                  </div>
                  {editingKey === v.key ? (
                    <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
                      <input
                        type={v.sensitive ? 'password' : 'text'}
                        class="filter-input"
                        style={{ flex: 1, padding: '0.375rem' }}
                        value={editValue}
                        onInput={(e) => setEditValue((e.target as HTMLInputElement).value)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter') handleUpdateVariable(v.key, editValue);
                          if (e.key === 'Escape') cancelEdit();
                        }}
                        autoFocus
                      />
                      <button class="btn btn-primary" style={{ padding: '0.25rem 0.5rem' }} onClick={() => handleUpdateVariable(v.key, editValue)}>
                        Save
                      </button>
                      <button class="btn btn-secondary" style={{ padding: '0.25rem 0.5rem' }} onClick={cancelEdit}>
                        Cancel
                      </button>
                    </div>
                  ) : (
                    <div
                      style={{
                        fontFamily: 'monospace',
                        fontSize: '0.8125rem',
                        color: 'var(--text-secondary)',
                        cursor: v.editable ? 'pointer' : 'default',
                      }}
                      onClick={() => startEdit(v)}
                      title={v.editable ? 'Click to edit' : ''}
                    >
                      {v.value}
                    </div>
                  )}
                </div>
                {v.editable && editingKey !== v.key && (
                  <button
                    class="btn btn-secondary"
                    style={{ padding: '0.25rem 0.5rem', fontSize: '0.75rem' }}
                    onClick={() => startEdit(v)}
                  >
                    Edit
                  </button>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Other Variables */}
      {otherVars.length > 0 && (
        <div class="card">
          <div class="card-header">
            <h3 class="card-title">System & Inherited Variables ({otherVars.length})</h3>
          </div>
          <div style={{ maxHeight: '400px', overflow: 'auto' }}>
            <table class="table">
              <thead>
                <tr>
                  <th>Key</th>
                  <th>Value</th>
                  <th>Source</th>
                </tr>
              </thead>
              <tbody>
                {otherVars.map((v) => (
                  <tr key={v.key}>
                    <td style={{ fontFamily: 'monospace', fontSize: '0.8125rem' }}>{v.key}</td>
                    <td style={{ fontFamily: 'monospace', fontSize: '0.75rem', maxWidth: '300px', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                      {v.value}
                    </td>
                    <td>{getSourceBadge(v.source)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Empty State */}
      {filteredVariables.length === 0 && (
        <div class="card" style={{ textAlign: 'center', padding: '3rem' }}>
          <div style={{ fontSize: '2rem', marginBottom: '0.5rem', opacity: 0.5 }}>🔧</div>
          <div style={{ color: 'var(--text-secondary)' }}>
            {variables.length === 0 ? 'No environment variables found' : 'No variables match your search'}
          </div>
        </div>
      )}

      {/* Summary */}
      <div style={{ marginTop: '1rem', fontSize: '0.75rem', color: 'var(--text-muted)', textAlign: 'center' }}>
        Total: {variables.length} variables
        ({variables.filter((v) => v.source === 'custom').length} custom,
        {variables.filter((v) => v.source === 'worker').length} worker,
        {variables.filter((v) => v.source === 'inherited').length} inherited,
        {variables.filter((v) => v.source === 'system').length} system)
      </div>
    </Fragment>
  );
}
