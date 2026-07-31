import React, { useState, useCallback, useEffect } from 'react';
import type { AppNotification } from '../types';
import { api } from '../api';
import Loading from './common/Loading';
import ErrorState from './common/ErrorState';
import EmptyState from './common/EmptyState';

interface NotificationsProps {
  onOpenTicket: (id: number) => void;
}

export function friendlyType(type: string): string {
  if (!type) return 'Notification';
  switch (type.toLowerCase()) {
    case 'ticket_created': return 'Ticket Created';
    case 'ticket_updated': return 'Ticket Updated';
    case 'ticket_assigned': return 'Ticket Assigned';
    case 'ticket_closed': return 'Ticket Closed';
    case 'solution_added': return 'Solution Added';
    case 'approval': return 'Approval';
    case 'escalation': return 'Escalation';
    default: return type.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
  }
}

export default function Notifications({ onOpenTicket }: NotificationsProps) {
  const [items, setItems] = useState<AppNotification[]>([]);
  const [unread, setUnread] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const r = await api.getNotifications();
      setItems(r.notifications);
      setUnread(r.unread);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load notifications');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  // Live refresh: re-fetch periodically so new webhook events appear.
  useEffect(() => {
    const timer = setInterval(() => {
      load();
    }, 60000);
    return () => clearInterval(timer);
  }, [load]);

  const handleOpen = async (n: AppNotification) => {
    if (n.ticket_id > 0) {
      try {
        await api.markNotificationRead(n.id);
      } catch {
        // best-effort read tracking
      }
      onOpenTicket(n.ticket_id);
    }
  };

  const handleDismiss = async (id: string) => {
    try {
      await api.dismissNotification(id);
      load();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to dismiss notification');
    }
  };

  const markAllRead = async () => {
    if (items.length === 0) return;
    try {
      await Promise.all(items.map((n) => api.markNotificationRead(n.id)));
      setUnread(0);
    } catch {
      // best-effort
    }
  };

  return (
    <div>
      <div className="glpi-section">
        <div className="glpi-flex glpi-flex-between glpi-flex-center">
          <div className="glpi-section-title">Notifications {unread > 0 ? `(${unread} unread)` : ''}</div>
          <div className="glpi-flex glpi-gap-8">
            <button className="glpi-btn glpi-btn-secondary glpi-btn-sm" onClick={load}>
              ↻ Refresh
            </button>
            {unread > 0 && (
              <button className="glpi-btn glpi-btn-primary glpi-btn-sm" onClick={markAllRead}>
                Mark all read
              </button>
            )}
          </div>
        </div>

        {loading && <Loading text="Loading notifications..." />}
        {error && <ErrorState message={error} onRetry={load} />}

        {!loading && !error && items.length === 0 && (
          <EmptyState icon="🔔" text="No notifications" subtext="GLPI webhook events will appear here." />
        )}

        {!loading && items.map((n) => (
          <div key={n.id} className="glpi-card">
            <div className="glpi-flex glpi-flex-between glpi-flex-center">
              <div style={{ minWidth: 0 }} onClick={() => handleOpen(n)}>
                <div style={{ fontWeight: 600, fontSize: 13, cursor: n.ticket_id > 0 ? 'pointer' : 'default' }}>
                  {friendlyType(n.type)}
                  {n.ticket_id > 0 ? ` — #${n.ticket_id}` : ''}
                </div>
                {n.title && (
                  <div style={{ fontSize: 13, marginTop: 2, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {n.title}
                  </div>
                )}
                <div className="glpi-text-small" style={{ marginTop: 2 }}>
                  {n.status ? `${n.status} · ` : ''}{new Date(n.created_at * 1000).toLocaleString()}
                </div>
              </div>
              <button
                className="glpi-btn glpi-btn-secondary glpi-btn-sm"
                onClick={() => handleDismiss(n.id)}
              >
                Dismiss
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
