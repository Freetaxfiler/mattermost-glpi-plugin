import React, { useState, useCallback, useEffect, useMemo } from 'react';
import type { ViewName, StatusResponse, UserResponse } from '../types';
import { api } from '../api';
import { isTechnicianOrHigher } from '../roles';

import Dashboard from './Dashboard';
import CreateTicket from './CreateTicket';
import TicketList from './TicketList';
import TicketDetails from './TicketDetails';
import SearchTicket from './SearchTicket';
import Assets from './Assets';
import MyAssets from './MyAssets';
import KnowledgeBase from './KnowledgeBase';
import Notifications from './Notifications';
import Settings from './Settings';
import Admin from './Admin';

import '../styles.css';

interface GLPISidebarProps {
  onClose?: () => void;
}

type NavItem = {
  id: ViewName;
  label: string;
};

const NAV_ITEMS: NavItem[] = [
  { id: 'dashboard', label: 'Dashboard' },
  { id: 'create-ticket', label: 'New Ticket' },
  { id: 'my-tickets', label: 'My Tickets' },
  { id: 'assigned-tickets', label: 'Assigned' },
  { id: 'search', label: 'Search' },
  { id: 'assets', label: 'Assets' },
  { id: 'my-assets', label: 'My Assets' },
  { id: 'knowledge-base', label: 'KB' },
  { id: 'notifications', label: 'Alerts' },
  { id: 'settings', label: 'Settings' },
];

export default function GLPISidebar({ onClose }: GLPISidebarProps) {
  const [currentView, setCurrentView] = useState<ViewName>('dashboard');
  const [selectedTicketId, setSelectedTicketId] = useState<number | null>(null);
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [notificationUnread, setNotificationUnread] = useState(0);
  const [refreshKey, setRefreshKey] = useState(0);
  const [user, setUser] = useState<UserResponse | null>(null);

  useEffect(() => {
    api.getStatus()
      .then(setStatus)
      .catch(() => {})
      .finally(() => setLoading(false));
    api.getUser().then(setUser).catch(() => {});
  }, []);

  // Admin view is available only to Mattermost System Admins.
  const navItems = useMemo(() => {
    if (user?.is_system_admin) {
      return [...NAV_ITEMS, { id: 'admin' as ViewName, label: 'Admin' }];
    }
    return NAV_ITEMS;
  }, [user]);

  const loadUnread = useCallback(() => {
    api.getNotifications().then((r) => setNotificationUnread(r.unread)).catch(() => {});
  }, []);

  useEffect(() => {
    loadUnread();
  }, [loadUnread]);

  // Live refresh: the plugin bootstrap dispatches a window event when the
  // server pushes a notification WebSocket event. Refresh the badge and, on
  // the dashboard/notifications views, remount the content so it re-fetches.
  useEffect(() => {
    const handler = () => {
      loadUnread();
      if (currentView === 'dashboard' || currentView === 'notifications') {
        setRefreshKey((k) => k + 1);
      }
    };
    window.addEventListener('glpi:notification', handler);
    return () => window.removeEventListener('glpi:notification', handler);
  }, [loadUnread, currentView]);

  const navigate = useCallback((view: ViewName) => {
    setCurrentView(view);
    if (view === 'notifications') {
      loadUnread();
    }
  }, [loadUnread]);

  const openTicket = useCallback((id: number) => {
    setSelectedTicketId(id);
    setCurrentView('ticket-details');
  }, []);

  const renderView = () => {
    switch (currentView) {
      case 'dashboard':
        return <Dashboard status={status} loading={loading} onNavigate={navigate} onOpenTicket={openTicket} />;
      case 'create-ticket':
        return <CreateTicket onNavigate={navigate} />;
      case 'my-tickets':
        return <TicketList type="my" onNavigate={navigate} onOpenTicket={openTicket} />;
      case 'assigned-tickets':
        return <TicketList type="assigned" onNavigate={navigate} onOpenTicket={openTicket} />;
      case 'search':
        return <SearchTicket onNavigate={navigate} onOpenTicket={openTicket} />;
      case 'ticket-details':
        return selectedTicketId ? (
          <TicketDetails
            ticketId={selectedTicketId}
            onNavigate={navigate}
            role={user?.role}
            isSystemAdmin={user?.is_system_admin || false}
          />
        ) : (
          <Dashboard status={status} loading={loading} onNavigate={navigate} onOpenTicket={openTicket} />
        );
      case 'assets':
        return <Assets />;
      case 'my-assets':
        return <MyAssets />;
      case 'knowledge-base':
        return <KnowledgeBase />;
      case 'notifications':
        return <Notifications onOpenTicket={openTicket} />;
      case 'settings':
        return <Settings />;
      case 'admin':
        return user?.is_system_admin ? <Admin /> : <Dashboard status={status} loading={loading} onNavigate={navigate} onOpenTicket={openTicket} />;
      default:
        return <Dashboard status={status} loading={loading} onNavigate={navigate} onOpenTicket={openTicket} />;
    }
  };

  const isOnline = status?.glpi_online ?? false;
  const isConfigured = status?.configured ?? false;

  return (
    <div className="glpi-sidebar">
      {/* Header */}
      <div className="glpi-sidebar-header">
        <h2>GLPI</h2>
        <span
          className={`glpi-indicator ${isOnline ? 'glpi-indicator-online' : 'glpi-indicator-offline'}`}
          title={isOnline ? 'Connected to GLPI' : 'Disconnected'}
        >
          <span className={`glpi-indicator-dot ${isOnline ? 'glpi-indicator-dot-on' : 'glpi-indicator-dot-off'}`} />
          {isOnline ? 'Online' : 'Offline'}
        </span>
        {onClose && (
          <button className="glpi-sidebar-close" onClick={onClose} aria-label="Close">
            ✕
          </button>
        )}
      </div>

      {/* Navigation */}
      <nav className="glpi-nav">
        {navItems.map((item) => (
          <button
            key={item.id}
            className={`glpi-nav-btn ${currentView === item.id ? 'active' : ''}`}
            onClick={() => navigate(item.id)}
          >
            {item.label}
            {item.id === 'notifications' && notificationUnread > 0 && (
              <span className="glpi-notif-badge">{notificationUnread}</span>
            )}
          </button>
        ))}
      </nav>

      {/* Content */}
      <div className="glpi-content" key={refreshKey}>
        {!isConfigured && currentView === 'dashboard' ? (
          <div className="glpi-error">
            <div className="glpi-error-icon">⚙️</div>
            <div className="glpi-error-text">
              GLPI is not configured. Set the GLPI URL, App Token, and User Token in the System Console.
            </div>
          </div>
        ) : (
          renderView()
        )}
      </div>
    </div>
  );
}
