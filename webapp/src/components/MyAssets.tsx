import React, { useState, useEffect, useCallback } from 'react';
import type { MyAssetsResponse, ConfigResponse, AssetDetail } from '../types';
import { api } from '../api';
import Loading from './common/Loading';
import ErrorState from './common/ErrorState';
import EmptyState from './common/EmptyState';

const ITEM_LABELS: Record<string, string> = {
  Computer: '💻 Computer',
  Printer: '🖨️ Printer',
  Monitor: '🖥️ Monitor',
  NetworkEquipment: '🌐 Network',
  Software: '💿 Software',
  SoftwareLicense: '🔑 License',
};

// formNameForType maps a GLPI item type to its GLPI form name for deep links.
function formNameForType(itemType: string): string {
  switch (itemType) {
    case 'NetworkEquipment': return 'networkequipment';
    case 'SoftwareLicense': return 'softwarelicense';
    default: return itemType.toLowerCase();
  }
}

export default function MyAssets() {
  const [data, setData] = useState<MyAssetsResponse | null>(null);
  const [glpiUrl, setGlpiUrl] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Detail view state
  const [detail, setDetail] = useState<AssetDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);

  // Create-ticket-from-asset state
  const [creating, setCreating] = useState(false);
  const [createdTicketId, setCreatedTicketId] = useState<number | null>(null);
  const [ticketNote, setTicketNote] = useState('');

  useEffect(() => {
    api.getConfig().then((c: ConfigResponse) => setGlpiUrl(c.glpi_url || '')).catch(() => {});
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setData(await api.getMyAssets());
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load assets');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const openDetail = useCallback(async (itemType: string, id: number) => {
    setDetailLoading(true);
    setDetailError(null);
    setCreatedTicketId(null);
    try {
      setDetail(await api.getAssetDetail(itemType, id));
    } catch (err: unknown) {
      setDetailError(err instanceof Error ? err.message : 'Failed to load asset');
    } finally {
      setDetailLoading(false);
    }
  }, []);

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

  if (loading) return <Loading />;
  if (error) return <ErrorState message={error} onRetry={load} />;

  const assets = data?.assets || [];

  const assetURL = (t: string, id: number) => {
    const base = glpiUrl.replace(/\/$/, '');
    return `${base}/front/${formNameForType(t)}.form.php?id=${id}`;
  };

  // Asset detail view
  if (detail) {
    return (
      <div className="glpi-content-inner">
        <div className="glpi-flex glpi-flex-between glpi-flex-center">
          <div className="glpi-section-title">Asset Detail</div>
          <button className="glpi-btn glpi-btn-secondary glpi-btn-sm" onClick={() => { setDetail(null); setDetailError(null); }}>
            ← Back to my assets
          </button>
        </div>

        <div className="glpi-card">
          <div style={{ fontSize: 16, fontWeight: 600, marginBottom: 4 }}>
            {detail.name || 'Unnamed'}
          </div>
          <div className="glpi-text-small glpi-mb-10">
            {ITEM_LABELS[detail.itemtype] || detail.itemtype} #{detail.id}
          </div>

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
                  <span style={{ fontSize: 12, color: 'var(--center-channel-color-60)' }}>Assigned User</span>
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
          {glpiUrl && (
            <a
              className="glpi-btn glpi-btn-secondary glpi-btn-sm"
              style={{ textDecoration: 'none' }}
              href={assetURL(detail.itemtype, detail.id)}
              target="_blank"
              rel="noreferrer"
            >
              Open in GLPI
            </a>
          )}
        </div>
      </div>
    );
  }

  // Asset list view
  return (
    <div className="glpi-content-inner">
      <div className="glpi-flex glpi-flex-between glpi-flex-center">
        <div className="glpi-section-title">My Assets ({assets.length})</div>
        <button className="glpi-btn glpi-btn-secondary glpi-btn-sm" onClick={load}>↻ Refresh</button>
      </div>

      {data && !data.mapped && (
        <div className="glpi-text-small glpi-mb-10">
          Your Mattermost account is not linked to a GLPI user, so no assets are shown.
        </div>
      )}

      {assets.length === 0 ? (
        <EmptyState icon="📦" text="No assets assigned" subtext="Assets assigned to you in GLPI will appear here." />
      ) : (
        assets.map((a) => (
          <div
            key={`${a.ItemType}-${a.ID}`}
            className="glpi-card glpi-mb-10"
            style={{ padding: '10px 12px', cursor: 'pointer' }}
            onClick={() => openDetail(a.ItemType || 'Computer', a.ID)}
          >
            <div className="glpi-flex glpi-flex-between glpi-flex-center">
              <div>
                <div style={{ fontSize: 13, fontWeight: 600 }}>
                  {ITEM_LABELS[a.ItemType || ''] || a.ItemType || 'Asset'} — {a.Name}
                </div>
                {a.Serial && <div className="glpi-text-small">Serial: {a.Serial}</div>}
              </div>
              <span className="glpi-text-small">Details →</span>
            </div>
          </div>
        ))
      )}
    </div>
  );
}
