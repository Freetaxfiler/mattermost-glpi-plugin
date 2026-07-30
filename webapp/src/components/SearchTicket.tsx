import React, { useState, useCallback } from 'react';
import type { ViewName, TicketSummary } from '../types';
import { api } from '../api';
import Loading from './common/Loading';
import ErrorState from './common/ErrorState';
import EmptyState from './common/EmptyState';
import { StatusBadge, PriorityDot } from './common/StatusBadge';

interface SearchTicketProps {
  onNavigate: (view: ViewName) => void;
  onOpenTicket: (id: number) => void;
}

export default function SearchTicket({ onNavigate, onOpenTicket }: SearchTicketProps) {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<TicketSummary[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searched, setSearched] = useState(false);

  const handleSearch = useCallback(async () => {
    if (!query.trim()) return;
    setLoading(true);
    setError(null);
    setSearched(true);
    try {
      const result = await api.listTickets('all', query.trim());
      setResults(result.tickets);
      setTotal(result.total);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Search failed');
    } finally {
      setLoading(false);
    }
  }, [query]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') handleSearch();
  };

  return (
    <div>
      <div className="glpi-section">
        <div className="glpi-section-title">Search Tickets</div>

        <div className="glpi-search-bar">
          <input
            className="glpi-input"
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Search by title..."
            maxLength={100}
          />
          <button
            className="glpi-btn glpi-btn-primary glpi-btn-sm"
            onClick={handleSearch}
            disabled={loading || !query.trim()}
          >
            {loading ? '...' : 'Search'}
          </button>
        </div>

        {loading && <Loading text="Searching..." />}
        {error && <ErrorState message={error} onRetry={handleSearch} />}

        {!loading && searched && results.length === 0 && (
          <EmptyState icon="🔍" text="No tickets found" subtext={`No results for "${query}"`} />
        )}

        {!loading && results.length > 0 && (
          <>
            <div className="glpi-text-small glpi-mb-10">{total} ticket{total !== 1 ? 's' : ''} found</div>
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
                {results.map((t) => (
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
          </>
        )}
      </div>
    </div>
  );
}
