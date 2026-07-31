import React, { useState, useEffect, useCallback } from 'react';
import type { ViewName, TicketSummary } from '../types';
import { api } from '../api';
import Loading from './common/Loading';
import ErrorState from './common/ErrorState';
import EmptyState from './common/EmptyState';
import { StatusBadge, PriorityDot } from './common/StatusBadge';

interface TicketListProps {
  type: 'my' | 'assigned';
  onNavigate: (view: ViewName) => void;
  onOpenTicket: (id: number) => void;
}

const STATUS_FILTERS = [
  { value: 0, label: 'All statuses' },
  { value: 1, label: 'New' },
  { value: 2, label: 'Processing' },
  { value: 3, label: 'Planned' },
  { value: 4, label: 'Pending' },
  { value: 5, label: 'Solved' },
  { value: 6, label: 'Closed' },
];

const SORT_OPTIONS = [
  { value: 19, label: 'Last updated' },
  { value: 2, label: 'ID' },
  { value: 1, label: 'Title' },
  { value: 3, label: 'Priority' },
  { value: 12, label: 'Status' },
];

export default function TicketList({ type, onNavigate, onOpenTicket }: TicketListProps) {
  const [tickets, setTickets] = useState<TicketSummary[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [statusFilter, setStatusFilter] = useState(0);
  const [sortBy, setSortBy] = useState(19);
  const [sortOrder, setSortOrder] = useState<'ASC' | 'DESC'>('DESC');

  const fetchTickets = useCallback(async (pageNum: number) => {
    setLoading(true);
    setError(null);
    try {
      const result = await api.listTickets(type, undefined, pageNum, 15, statusFilter || undefined, sortBy, sortOrder);
      setTickets(result.tickets);
      setTotal(result.total);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load tickets');
    } finally {
      setLoading(false);
    }
  }, [type, statusFilter, sortBy, sortOrder]);

  useEffect(() => {
    fetchTickets(page);
  }, [fetchTickets, page]);

  const handleStatusChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    setStatusFilter(Number(e.target.value));
    setPage(1);
  };

  const handleSortChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    setSortBy(Number(e.target.value));
    setPage(1);
  };

  const title = type === 'my' ? 'My Tickets' : 'Assigned Tickets';
  const totalPages = Math.ceil(total / 15);

  return (
    <div>
      <div className="glpi-section">
        <div className="glpi-section-title">{title}</div>

        <div className="glpi-search-bar">
          <select
            className="glpi-select"
            value={statusFilter}
            onChange={handleStatusChange}
            aria-label="Filter by status"
          >
            {STATUS_FILTERS.map((opt) => (
              <option key={opt.value} value={opt.value}>{opt.label}</option>
            ))}
          </select>
          <select
            className="glpi-select"
            value={sortBy}
            onChange={handleSortChange}
            aria-label="Sort by"
          >
            {SORT_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>{opt.label}</option>
            ))}
          </select>
          <button
            className="glpi-btn glpi-btn-secondary glpi-btn-sm"
            onClick={() => setSortOrder((o) => (o === 'DESC' ? 'ASC' : 'DESC'))}
          >
            {sortOrder === 'DESC' ? '↓ Newest' : '↑ Oldest'}
          </button>
          <button
            className="glpi-btn glpi-btn-secondary glpi-btn-sm"
            onClick={() => fetchTickets(page)}
            disabled={loading}
          >
            ↻ Refresh
          </button>
        </div>

        {loading && <Loading text="Loading tickets..." />}
        {error && <ErrorState message={error} onRetry={() => fetchTickets(page)} />}

        {!loading && !error && tickets.length === 0 && (
          <EmptyState
            icon="🎫"
            text="No tickets found"
            subtext={type === 'my' ? 'You have no open tickets.' : 'No tickets assigned to you.'}
          />
        )}

        {!loading && tickets.length > 0 && (
          <>
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
                {tickets.map((t) => (
                  <tr
                    key={t.ID}
                    className="glpi-clickable"
                    onClick={() => onOpenTicket(t.ID)}
                  >
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

            {total > 15 && (
              <div className="glpi-flex glpi-flex-center" style={{ justifyContent: 'center', gap: 6, marginTop: 12 }}>
                <button
                  className="glpi-btn glpi-btn-sm glpi-btn-secondary"
                  disabled={page <= 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                >
                  ← Prev
                </button>
                <span className="glpi-text-small">Page {page} of {totalPages}</span>
                <button
                  className="glpi-btn glpi-btn-sm glpi-btn-secondary"
                  disabled={page >= totalPages}
                  onClick={() => setPage((p) => p + 1)}
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
