import React, { useState, useCallback, useEffect } from 'react';
import type { AssetSummary, AssetDetail } from '../types';
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

const PAGE_SIZE = 15;

function formNameForType(itemType: string): string {
  switch (itemType) {
    case 'NetworkEquipment': return 'networkequipment';
    case 'SoftwareLicense': return 'softwarelicense';
    default: return itemType.toLowerCase();
  }
}

export default function Assets() {
  const [type, setType] = useState('Computer');
  const [search, setSearch] = useState('');
  const [assets, setAssets] = useState<AssetSummary[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [detail, setDetail] = useState<AssetDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);
  const [glpiUrl, setGlpiUrl] = useState('');

  // Create-ticket-from-asset state
  const [creating, setCreating] = useState(false);
  const [createdTicketId, setCreatedTicketId] = useState<number | null>(null);
  const [ticketNote, setTicketNote] = useState('');

  const createTicketFromAsset = async () => {
    if (!detail) return;
    setCreating(true);
    setDetailError(null);
    try {
      const result = await api.createTicket({
        subject: `Issue with ${detail.name || `${detail.itemtype} #${detail.id}`}`,
        content: (ticketNote || `Reported from asset ${detail.name || detail.itemtype} (#${detail.id}).`).trim(),
        priority: 3,
        urgency: 3,
        category_id: 0,
        asset_id: detail.id,
        asset_type: detail.itemtype,
      });
      setCreatedTicketId(result.id);
      setTicketNote('');
    } catch (err: unknown) {
      setDetailError(err instanceof Error ? err.message : 'Failed to create ticket');
    } finally {
      setCreating(false);
    }
  };

  useEffect(() => {
    api.getConfig().then((c) => setGlpiUrl(c.glpi_url || '')).catch(() => {});
  }, []);

  const fetchAssets = useCallback(async (pageNum: number) => {
    setLoading(true);
    setError(null);
    try {
      const result = await api.listAssets(type, search || undefined, pageNum, PAGE_SIZE);
      setAssets(result.assets);
      setTotal(result.total);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load assets');
    } finally {
      setLoading(false);
    }
  }, [type, search]);

  const openDetail = useCallback(async (id: number) => {
    setDetailLoading(true);
    setDetailError(null);
    try {
      const asset = await api.getAssetDetail(type, id);
      setDetail(asset);
    } catch (err: unknown) {
      setDetailError(err instanceof Error ? err.message : 'Failed to load asset');
    } finally {
      setDetailLoading(false);
    }
  }, [type]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      setPage(1);
      fetchAssets(1);
    }
  };

  if (detail) {
    return (
      <div>
        <div className="glpi-section">
          <div className="glpi-section-title">Asset Detail</div>
          <div className="glpi-card">
            <div style={{ fontSize: 16, fontWeight: 600, marginBottom: 4 }}>
              {detail.name || 'Unnamed'}
            </div>
            <div className="glpi-text-small glpi-mb-10">{detail.itemtype} #{detail.id}</div>
            {detailError && <div className="glpi-form-error glpi-mb-10">{detailError}</div>}
            {detailLoading && <Loading text="Loading asset..." />}
            {!detailLoading && (
              <>
                {detail.serial && (
                  <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
                    <span style={{ fontSize: 12, color: 'var(--center-channel-color-60)' }}>Serial</span>
                    <span style={{ fontFamily: 'monospace', fontSize: 13 }}>{detail.serial}</span>
                  </div>
                )}
                {detail.otherserial && (
                  <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
                    <span style={{ fontSize: 12, color: 'var(--center-channel-color-60)' }}>Inventory #</span>
                    <span style={{ fontSize: 13 }}>{detail.otherserial}</span>
                  </div>
                )}
                {detail.manufacturer && (
                  <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
                    <span style={{ fontSize: 12, color: 'var(--center-channel-color-60)' }}>Manufacturer</span>
                    <span style={{ fontSize: 13 }}>{detail.manufacturer}</span>
                  </div>
                )}
                {detail.model && (
                  <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
                    <span style={{ fontSize: 12, color: 'var(--center-channel-color-60)' }}>Model</span>
                    <span style={{ fontSize: 13 }}>{detail.model}</span>
                  </div>
                )}
                {detail.location && (
                  <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
                    <span style={{ fontSize: 12, color: 'var(--center-channel-color-60)' }}>Location</span>
                    <span style={{ fontSize: 13 }}>{detail.location}</span>
                  </div>
                )}
                {detail.user && (
                  <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
                    <span style={{ fontSize: 12, color: 'var(--center-channel-color-60)' }}>User</span>
                    <span style={{ fontSize: 13 }}>{detail.user}</span>
                  </div>
                )}
                {detail.tech_user && (
                  <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
                    <span style={{ fontSize: 12, color: 'var(--center-channel-color-60)' }}>Technician</span>
                    <span style={{ fontSize: 13 }}>{detail.tech_user}</span>
                  </div>
                )}
                {detail.state && (
                  <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
                    <span style={{ fontSize: 12, color: 'var(--center-channel-color-60)' }}>State</span>
                    <span style={{ fontSize: 13 }}>{detail.state}</span>
                  </div>
                )}
                {detail.warranty_date && (
                  <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
                    <span style={{ fontSize: 12, color: 'var(--center-channel-color-60)' }}>Warranty</span>
                    <span style={{ fontSize: 13 }}>{detail.warranty_date}</span>
                  </div>
                )}
                {detail.notes && (
                  <div className="glpi-mt-10">
                    <div style={{ fontSize: 12, color: 'var(--center-channel-color-60)' }}>Notes</div>
                    <div style={{ fontSize: 13, marginTop: 4, whiteSpace: 'pre-wrap' }}>{detail.notes}</div>
                  </div>
                )}
              </>
            )}
          </div>

          {/* Create ticket from asset */}
          <div className="glpi-card glpi-mt-10">
            <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 6 }}>Report an issue with this asset</div>
            <textarea
              className="glpi-textarea"
              value={ticketNote}
              onChange={(e) => setTicketNote(e.target.value)}
              placeholder="Describe the issue (optional)"
              rows={2}
            />
            <div className="glpi-flex glpi-gap-8 glpi-mt-10">
              <button
                className="glpi-btn glpi-btn-primary glpi-btn-sm"
                onClick={createTicketFromAsset}
                disabled={creating}
              >
                {creating ? 'Creating…' : 'Create Ticket'}
              </button>
              {createdTicketId && (
                <span className="glpi-text-small" style={{ color: 'var(--online-indicator)' }}>
                  ✅ Ticket #{createdTicketId} created
                </span>
              )}
            </div>
          </div>

          <div className="glpi-flex glpi-gap-8 glpi-mt-10">
            <button className="glpi-btn glpi-btn-secondary glpi-btn-sm" onClick={() => { setDetail(null); setDetailError(null); }}>
              ← Back to list
            </button>
            {glpiUrl && (
              <a
                className="glpi-btn glpi-btn-primary glpi-btn-sm"
                style={{ textDecoration: 'none' }}
                href={`${glpiUrl}/front/${formNameForType(detail.itemtype)}.form.php?id=${detail.id}`}
                target="_blank"
                rel="noreferrer"
              >
                Open in GLPI
              </a>
            )}
          </div>
        </div>
      </div>
    );
  }

  const totalPages = Math.ceil(total / PAGE_SIZE);

  return (
    <div>
      <div className="glpi-section">
        <div className="glpi-section-title">Assets</div>

        <div className="glpi-form-group">
          <label className="glpi-label">Asset Type</label>
          <select
            className="glpi-select"
            value={type}
            onChange={(e) => { setType(e.target.value); setAssets([]); setPage(1); }}
          >
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
            onClick={() => { setPage(1); fetchAssets(1); }}
            disabled={loading}
          >
            {loading ? '...' : 'Search'}
          </button>
          <button
            className="glpi-btn glpi-btn-secondary glpi-btn-sm"
            onClick={() => fetchAssets(page)}
            disabled={loading}
          >
            ↻ Refresh
          </button>
        </div>

        {loading && <Loading text="Loading assets..." />}
        {error && <ErrorState message={error} onRetry={() => fetchAssets(page)} />}

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
                  <tr key={a.ID} className="glpi-clickable" onClick={() => openDetail(a.ID)}>
                    <td style={{ fontWeight: 600 }}>#{a.ID}</td>
                    <td>{a.Name}</td>
                    <td style={{ fontFamily: 'monospace', fontSize: 12 }}>{a.Serial || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>

            {totalPages > 1 && (
              <div className="glpi-flex glpi-flex-center" style={{ justifyContent: 'center', gap: 6, marginTop: 12 }}>
                <button
                  className="glpi-btn glpi-btn-sm glpi-btn-secondary"
                  disabled={page <= 1}
                  onClick={() => { const p = Math.max(1, page - 1); setPage(p); fetchAssets(p); }}
                >
                  ← Prev
                </button>
                <span className="glpi-text-small">Page {page} of {totalPages}</span>
                <button
                  className="glpi-btn glpi-btn-sm glpi-btn-secondary"
                  disabled={page >= totalPages}
                  onClick={() => { const p = page + 1; setPage(p); fetchAssets(p); }}
                >
                  Next →
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
