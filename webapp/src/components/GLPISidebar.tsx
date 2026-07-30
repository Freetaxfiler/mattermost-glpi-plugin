import React, { useState, useCallback, useEffect } from 'react';
import type { ViewName, StatusResponse } from '../types';
import { api } from '../api';

import Dashboard from './Dashboard';
import CreateTicket from './CreateTicket';
import TicketList from './TicketList';
import TicketDetails from './TicketDetails';
import SearchTicket from './SearchTicket';
import Assets from './Assets';
import KnowledgeBase from './KnowledgeBase';
import Notifications from './Notifications';
import Settings from './Settings';

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
  { id: 'search', label: 'Search' },
  { id: 'assets', label: 'Assets' },
  { id: 'knowledge-base', label: 'KB' },
  { id: 'notifications', label: 'Alerts' },
  { id: 'settings', label: 'Settings' },
];

export default function GLPISidebar({ onClose }: GLPISidebarProps) {
  const [currentView, setCurrentView] = useState<ViewName>('dashboard');
  const [selectedTicketId, setSelectedTicketId] = useState<number | null>(null);
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.getStatus()
      .then(setStatus)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  const navigate = useCallback((view: ViewName) => {
    setCurrentView(view);
  }, []);

  const openTicket = useCallback((id: number) => {
    setSelectedTicketId(id);
    setCurrentView('ticket-details');
  }, []);

  const renderView = () => {
    switch (currentView) {
      case 'dashboard':
        return <Dashboard status={status} loading={loading} onNavigate={navigate} />;
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
          <TicketDetails ticketId={selectedTicketId} onNavigate={navigate} />
        ) : (
          <Dashboard status={status} loading={loading} onNavigate={navigate} />
        );
      case 'assets':
        return <Assets />;
      case 'knowledge-base':
        return <KnowledgeBase />;
      case 'notifications':
        return <Notifications />;
      case 'settings':
        return <Settings />;
      default:
        return <Dashboard status={status} loading={loading} onNavigate={navigate} />;
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
        {NAV_ITEMS.map((item) => (
          <button
            key={item.id}
            className={`glpi-nav-btn ${currentView === item.id ? 'active' : ''}`}
            onClick={() => navigate(item.id)}
          >
            {item.label}
          </button>
        ))}
      </nav>

      {/* Content */}
      <div className="glpi-content">
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
