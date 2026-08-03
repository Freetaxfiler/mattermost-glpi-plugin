import React, { useState, useCallback, useEffect } from 'react';
import type { AdminMappingsResponse } from '../types';
import { api } from '../api';
import Loading from './common/Loading';
import ErrorState from './common/ErrorState';

const PROFILE_OPTIONS = [
  { id: 1, label: 'Self-Service (Employee)' },
  { id: 5, label: 'Technician' },
  { id: 8, label: 'Manager' },
  { id: 3, label: 'Admin' },
  { id: 4, label: 'Super-Admin' },
];

export default function Admin() {
  const [data, setData] = useState<AdminMappingsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [provisionProfile, setProvisionProfile] = useState<Record<string, number>>({});

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setData(await api.getAdminMappings());
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load mappings');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const doSync = async () => {
    setBusy('sync');
    setError(null);
    try {
      const r = await api.adminSyncUsers();
      alert(`Sync complete: ${r.mapped} mapped, ${r.errors} errors.`);
      await load();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Sync failed');
    } finally {
      setBusy(null);
    }
  };

  const doClearCache = async () => {
    setBusy('clear');
    setError(null);
    try {
      await api.adminClearCache();
      await load();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Clear cache failed');
    } finally {
      setBusy(null);
    }
  };

  const doProvision = async (userId: string, username: string) => {
    setBusy(userId);
    setError(null);
    try {
      const profileId = provisionProfile[userId] || 1;
      const r = await api.adminProvisionUser(userId, profileId);
      alert(`Provisioned ${username}: GLPI user #${r.glpi_user_id}`);
      await load();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Provision failed');
    } finally {
      setBusy(null);
    }
  };

  if (loading) return <Loading />;
  if (error && !data) return <ErrorState message={error} onRetry={load} />;

  if (!data) return null;

  const { mappings, unmapped, duplicate_emails, mm_user_count, mapping_enabled } = data;

  return (
    <div className="glpi-content-inner">
      {error && <div className="glpi-error-banner">{error}</div>}

      <div className="glpi-section-title">User Mapping — Administration</div>
      <p className="glpi-text-small">
        Mattermost users: <strong>{mm_user_count}</strong> · Mapped: <strong>{mappings.length}</strong> ·
        Unmapped: <strong>{unmapped.length}</strong> · Mode B (Map Mattermost Users):{' '}
        <strong>{mapping_enabled ? 'enabled' : 'disabled'}</strong>
      </p>

      <div className="glpi-flex glpi-mb-10" style={{ gap: 8 }}>
        <button className="glpi-btn glpi-btn-primary glpi-btn-sm" onClick={doSync} disabled={busy !== null}>
          {busy === 'sync' ? 'Syncing…' : '⟳ Sync Users'}
        </button>
        <button className="glpi-btn glpi-btn-secondary glpi-btn-sm" onClick={doClearCache} disabled={busy !== null}>
          {busy === 'clear' ? 'Clearing…' : 'Clear Cache'}
        </button>
        <button className="glpi-btn glpi-btn-secondary glpi-btn-sm" onClick={load} disabled={busy !== null}>
          Refresh
        </button>
      </div>

      {/* Mapped users */}
      <div className="glpi-section">
        <div className="glpi-section-title">Mapped Users ({mappings.length})</div>
        {mappings.length === 0 ? (
          <div className="glpi-text-small">No mappings yet.</div>
        ) : (
          <table className="glpi-table">
            <thead>
              <tr>
                <th>Mattermost</th>
                <th>GLPI User</th>
                <th>Profiles</th>
                <th>Role</th>
                <th>Last Sync</th>
              </tr>
            </thead>
            <tbody>
              {mappings.map((m) => (
                <tr key={m.mm_user_id}>
                  <td>
                    <div>{m.mm_display_name || m.mm_username}</div>
                    <div className="glpi-text-small">{m.mm_email}</div>
                  </td>
                  <td>
                    <div>#{m.glpi_user_id} {m.glpi_login}</div>
                    <div className="glpi-text-small">{m.glpi_email}</div>
                  </td>
                  <td className="glpi-text-small">{(m.profiles || []).join(', ') || '—'}</td>
                  <td>{m.role}</td>
                  <td className="glpi-text-small">{m.last_sync ? new Date(m.last_sync * 1000).toLocaleString() : '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Unmapped users */}
      <div className="glpi-section">
        <div className="glpi-section-title">Unmapped Users ({unmapped.length})</div>
        {unmapped.length === 0 ? (
          <div className="glpi-text-small">All Mattermost users are mapped.</div>
        ) : (
          <table className="glpi-table">
            <thead>
              <tr>
                <th>User</th>
                <th>Email</th>
                <th>Profile on creation</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {unmapped.map((u) => (
                <tr key={u.user_id}>
                  <td>{u.username}</td>
                  <td className="glpi-text-small">{u.email}</td>
                  <td>
                    <select
                      className="glpi-select"
                      style={{ width: 'auto', fontSize: 12 }}
                      value={provisionProfile[u.user_id] || 1}
                      onChange={(e) => setProvisionProfile((p) => ({ ...p, [u.user_id]: Number(e.target.value) }))}
                    >
                      {PROFILE_OPTIONS.map((o) => (
                        <option key={o.id} value={o.id}>{o.label}</option>
                      ))}
                    </select>
                  </td>
                  <td>
                    <button
                      className="glpi-btn glpi-btn-secondary glpi-btn-sm"
                      onClick={() => doProvision(u.user_id, u.username)}
                      disabled={busy !== null}
                    >
                      {busy === u.user_id ? 'Provisioning…' : 'Provision GLPI Account'}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Duplicate emails */}
      {duplicate_emails.length > 0 && (
        <div className="glpi-section">
          <div className="glpi-section-title">Duplicate Emails ({duplicate_emails.length})</div>
          <div className="glpi-text-small">Several Mattermost users share an email — mapping may be ambiguous.</div>
          {duplicate_emails.map((group, i) => (
            <div key={i} className="glpi-card glpi-mt-10" style={{ padding: '8px 12px' }}>
              {group.map((u) => (
                <div key={u.user_id} className="glpi-text-small">
                  {u.username} — {u.email}
                </div>
              ))}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
