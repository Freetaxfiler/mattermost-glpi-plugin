import React, { useState, useCallback } from 'react';
import type { AssetSummary } from '../types';
import { api } from '../api';
import Loading from './common/Loading';
import ErrorState from './common/ErrorState';
import EmptyState from './common/EmptyState';

const ASSET_TYPES = [
  { id: 'Computer', label: 'Computers' },
  { id: 'Printer', label: 'Printers' },
  { id: 'Monitor', label: 'Monitors' },
  { id: 'NetworkEquipment', label: 'Network' },
  { id: 'Software', label: 'Software' },
  { id: 'SoftwareLicense', label: 'Licenses' },
];

export default function Assets() {
  const [type, setType] = useState('Computer');
  const [search, setSearch] = useState('');
  const [assets, setAssets] = useState<AssetSummary[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchAssets = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await api.listAssets(type, search || undefined);
      setAssets(result.assets);
      setTotal(result.total);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load assets');
    } finally {
      setLoading(false);
    }
  }, [type, search]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') fetchAssets();
  };

  return (
    <div>
      <div className="glpi-section">
        <div className="glpi-section-title">Assets</div>

        <div className="glpi-form-group">
          <label className="glpi-label">Asset Type</label>
          <select className="glpi-select" value={type} onChange={(e) => { setType(e.target.value); setAssets([]); }}>
            {ASSET_TYPES.map((t) => (
              <option key={t.id} value={t.id}>{t.label}</option>
            ))}
          </select>
        </div>

        <div className="glpi-search-bar">
          <input
            className="glpi-input"
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Search by name..."
          />
          <button
            className="glpi-btn glpi-btn-primary glpi-btn-sm"
            onClick={fetchAssets}
            disabled={loading}
          >
            {loading ? '...' : 'Search'}
          </button>
        </div>

        {loading && <Loading text="Loading assets..." />}
        {error && <ErrorState message={error} onRetry={fetchAssets} />}

        {!loading && !error && assets.length === 0 && (
          <EmptyState icon="💻" text="No assets found" subtext={search ? `No "${type}" matching "${search}"` : `Click Search to list ${type.toLowerCase()}`} />
        )}

        {!loading && assets.length > 0 && (
          <>
            <div className="glpi-text-small glpi-mb-10">{total} asset{total !== 1 ? 's' : ''} found</div>
            <table className="glpi-table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Name</th>
                  <th>Serial</th>
                </tr>
              </thead>
              <tbody>
                {assets.map((a) => (
                  <tr key={a.ID}>
                    <td style={{ fontWeight: 600 }}>#{a.ID}</td>
                    <td>{a.Name}</td>
                    <td style={{ fontFamily: 'monospace', fontSize: 12 }}>{a.Serial || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </>
        )}
      </div>
    </div>
  );
}
