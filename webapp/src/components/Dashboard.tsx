import React, { useState, useEffect } from 'react';
import type { ViewName, StatusResponse, TicketSummary } from '../types';
import { api } from '../api';
import Loading from './common/Loading';
import ErrorState from './common/ErrorState';

interface DashboardProps {
  status: StatusResponse | null;
  loading: boolean;
  onNavigate: (view: ViewName) => void;
}

export default function Dashboard({ status, loading, onNavigate }: DashboardProps) {
  const [myCount, setMyCount] = useState<number | null>(null);
  const [assignedCount, setAssignedCount] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [fetching, setFetching] = useState(true);

  useEffect(() => {
    Promise.all([
      api.listTickets('my', undefined, 1, 1).then((r) => setMyCount(r.total)).catch(() => {}),
      api.listTickets('assigned', undefined, 1, 1).then((r) => setAssignedCount(r.total)).catch(() => {}),
    ]).finally(() => {
      setFetching(false);
      setError(null);
    });
  }, []);

  if (loading || fetching) {
    return <Loading text="Loading dashboard..." />;
  }

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
        <div className="glpi-section-title">Overview</div>
        <div className="glpi-stat-grid">
          <div className="glpi-stat">
            <div className="glpi-stat-value">{myCount ?? '—'}</div>
            <div className="glpi-stat-label">Open Tickets</div>
          </div>
          <div className="glpi-stat">
            <div className="glpi-stat-value">{assignedCount ?? '—'}</div>
            <div className="glpi-stat-label">Assigned</div>
          </div>
        </div>
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

      {error && <ErrorState message={error} />}
    </div>
  );
}
