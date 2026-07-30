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

export default function TicketList({ type, onNavigate, onOpenTicket }: TicketListProps) {
  const [tickets, setTickets] = useState<TicketSummary[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);

  const fetchTickets = useCallback(async (pageNum: number) => {
    setLoading(true);
    setError(null);
    try {
      const result = await api.listTickets(type, undefined, pageNum);
      setTickets(result.tickets);
      setTotal(result.total);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load tickets');
    } finally {
      setLoading(false);
    }
  }, [type]);

  useEffect(() => {
    fetchTickets(page);
  }, [fetchTickets, page]);

  const title = type === 'my' ? 'My Tickets' : 'Assigned Tickets';
  const totalPages = Math.ceil(total / 15);

  return (
    <div>
      <div className="glpi-section">
        <div className="glpi-section-title">{title}</div>

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
