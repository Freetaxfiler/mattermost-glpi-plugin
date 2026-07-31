import React, { useState, useEffect, useCallback } from 'react';
import type { ViewName, StatusResponse, UserResponse, DashboardData, TicketSummary, AppNotification } from '../types';
import { api } from '../api';
import Loading from './common/Loading';
import ErrorState from './common/ErrorState';
import EmptyState from './common/EmptyState';
import { StatusBadge, PriorityDot } from './common/StatusBadge';
import { friendlyType } from './Notifications';

interface DashboardProps {
  status: StatusResponse | null;
  loading: boolean;
  onNavigate: (view: ViewName) => void;
  onOpenTicket: (id: number) => void;
}

function StatCard({ label, value }: { label: string; value: number | null }) {
  return (
    <div className="glpi-stat">
      <div className="glpi-stat-value">{value === null || value < 0 ? '—' : value}</div>
      <div className="glpi-stat-label">{label}</div>
    </div>
  );
}

export default function Dashboard({ status, loading, onNavigate, onOpenTicket }: DashboardProps) {
  const [stats, setStats] = useState<DashboardData | null>(null);
  const [user, setUser] = useState<UserResponse | null>(null);
  const [notifications, setNotifications] = useState<AppNotification[]>([]);
  const [fetching, setFetching] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.getUser().then(setUser).catch(() => {});
  }, []);

  const load = useCallback(async () => {
    setFetching(true);
    setError(null);
    try {
      const data = await api.getDashboard();
      setStats(data);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load dashboard');
    } finally {
      setFetching(false);
    }
    // Recent activity is best-effort.
    try {
      const n = await api.getNotifications();
      setNotifications(n.notifications.slice(0, 5));
    } catch {
      /* ignore */
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  if (loading || fetching) return <Loading text="Loading dashboard..." />;

  const quickActions: Array<{ label: string; view: ViewName; icon: string }> = [
    { label: 'Create Ticket', view: 'create-ticket', icon: '➕' },
    { label: 'My Tickets', view: 'my-tickets', icon: '🎫' },
    { label: 'Search', view: 'search', icon: '🔍' },
    { label: 'Assets', view: 'assets', icon: '💻' },
    { label: 'Knowledge Base', view: 'knowledge-base', icon: '📚' },
    { label: 'Notifications', view: 'notifications', icon: '🔔' },
  ];

  return (
    <div>
      {/* Stats */}
      <div className="glpi-section">
        <div className="glpi-flex glpi-flex-between glpi-flex-center">
          <div className="glpi-section-title">Overview</div>
          <button className="glpi-btn glpi-btn-secondary glpi-btn-sm" onClick={load}>
            ↻ Refresh
          </button>
        </div>
        <div className="glpi-stat-grid">
          <StatCard label="Open Tickets" value={stats ? stats.open : null} />
          <StatCard label="Assigned" value={stats ? stats.assigned : null} />
          <StatCard label="Resolved" value={stats ? stats.resolved : null} />
          <StatCard label="Pending" value={stats ? stats.pending : null} />
          <StatCard label="Closed" value={stats ? stats.closed : null} />
          <StatCard label="Critical" value={stats ? stats.critical : null} />
          <StatCard label="Overdue" value={stats ? stats.overdue : null} />
        </div>
      </div>

      {error && (
        <div className="glpi-section">
          <ErrorState message={error} onRetry={load} />
        </div>
      )}

      {/* Recent tickets */}
      <div className="glpi-section">
        <div className="glpi-section-title">Recent Tickets</div>
        {stats && stats.recent && stats.recent.length > 0 ? (
          <table className="glpi-table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Title</th>
                <th>Status</th>
                <th>P</th>
              </tr>
            </thead>
            <tbody>
              {stats.recent.slice(0, 5).map((t: TicketSummary) => (
                <tr key={t.ID} className="glpi-clickable" onClick={() => onOpenTicket(t.ID)}>
                  <td style={{ fontWeight: 600 }}>#{t.ID}</td>
                  <td style={{ maxWidth: 150, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {t.Name}
                  </td>
                  <td><StatusBadge status={t.Status} /></td>
                  <td><PriorityDot priority={t.Priority} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <EmptyState icon="🎫" text="No recent tickets" subtext="Tickets you requested will appear here." />
        )}
      </div>

      {/* Recent Activity */}
      <div className="glpi-section">
        <div className="glpi-section-title">Recent Activity</div>
        {notifications.length > 0 ? (
          notifications.map((n) => (
            <div key={n.id} className="glpi-card" style={{ padding: '8px 12px', marginBottom: 6 }}>
              <div style={{ fontSize: 13, fontWeight: 600 }}>
                {friendlyType(n.type)}{n.ticket_id > 0 ? ` — #${n.ticket_id}` : ''}
              </div>
              {n.title && <div className="glpi-text-small">{n.title}</div>}
              <div className="glpi-text-small">{new Date(n.created_at * 1000).toLocaleString()}</div>
            </div>
          ))
        ) : (
          <EmptyState icon="🔔" text="No recent activity" subtext="GLPI webhook events will appear here." />
        )}
      </div>

      {/* Connection info */}
      <div className="glpi-section">
        <div className="glpi-section-title">Connection</div>
        <div className="glpi-card">
          <div className="glpi-flex glpi-flex-between glpi-flex-center">
            <span style={{ fontSize: 13 }}>GLPI Server</span>
            <span style={{ fontSize: 12 }} className="glpi-monospace">
              {status?.glpi_url || (status?.configured ? 'configured' : 'not set')}
            </span>
          </div>
          {status?.glpi_version && (
            <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
              <span style={{ fontSize: 13 }}>Version</span>
              <span style={{ fontSize: 12 }}>GLPI {status.glpi_version}</span>
            </div>
          )}
          <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
            <span style={{ fontSize: 13 }}>Plugin</span>
            <span style={{ fontSize: 12 }}>v{status?.plugin_version || '0.2.0'}</span>
          </div>
          {user && (
            <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
              <span style={{ fontSize: 13 }}>User</span>
              <span style={{ fontSize: 12 }}>
                {user.username}{user.glpi_user_id > 0 ? ` · GLPI #${user.glpi_user_id}` : ''}
              </span>
            </div>
          )}
        </div>
      </div>

      {/* Quick Actions */}
      <div className="glpi-section">
        <div className="glpi-section-title">Quick Actions</div>
        <div className="glpi-actions">
          {quickActions.map((action) => (
            <button
              key={action.view}
              className="glpi-btn glpi-btn-secondary"
              onClick={() => onNavigate(action.view)}
            >
              <span>{action.icon}</span>
              {action.label}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
