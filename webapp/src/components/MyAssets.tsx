import React, { useState, useEffect, useCallback } from 'react';
import type { MyAssetsResponse, ConfigResponse } from '../types';
import { api } from '../api';
import Loading from './common/Loading';
import ErrorState from './common/ErrorState';
import EmptyState from './common/EmptyState';

const ITEM_LABELS: Record<string, string> = {
  Computer: '💻 Computer',
  Printer: '🖨️ Printer',
  Monitor: '🖥️ Monitor',
  NetworkEquipment: '🌐 Network',
};

export default function MyAssets() {
  const [data, setData] = useState<MyAssetsResponse | null>(null);
  const [glpiUrl, setGlpiUrl] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

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

  if (loading) return <Loading />;
  if (error) return <ErrorState message={error} onRetry={load} />;

  const assets = data?.assets || [];

  const assetURL = (t: string, id: number) => {
    const base = glpiUrl.replace(/\/$/, '');
    return `${base}/front/computer.form.php?id=${id}`;
  };

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
          <div key={`${a.ItemType}-${a.ID}`} className="glpi-card glpi-mb-10" style={{ padding: '10px 12px' }}>
            <div className="glpi-flex glpi-flex-between glpi-flex-center">
              <div>
                <div style={{ fontSize: 13, fontWeight: 600 }}>
                  {ITEM_LABELS[a.ItemType || ''] || a.ItemType || 'Asset'} — {a.Name}
                </div>
                {a.Serial && <div className="glpi-text-small">Serial: {a.Serial}</div>}
              </div>
              {glpiUrl && (
                <a className="glpi-btn glpi-btn-secondary glpi-btn-sm" href={assetURL(a.ItemType || '', a.ID)} target="_blank" rel="noreferrer">
                  Open in GLPI
                </a>
              )}
            </div>
          </div>
        ))
      )}
    </div>
  );
}
