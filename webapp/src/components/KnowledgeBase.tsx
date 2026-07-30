import React, { useState, useCallback } from 'react';
import type { KnowledgeSummary } from '../types';
import { api } from '../api';
import Loading from './common/Loading';
import ErrorState from './common/ErrorState';
import EmptyState from './common/EmptyState';

export default function KnowledgeBase() {
  const [query, setQuery] = useState('');
  const [articles, setArticles] = useState<KnowledgeSummary[]>([]);
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
      const result = await api.searchKnowledge(query.trim());
      setArticles(result.articles);
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
        <div className="glpi-section-title">Knowledge Base</div>

        <div className="glpi-search-bar">
          <input
            className="glpi-input"
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Search knowledge base..."
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

        {!loading && searched && articles.length === 0 && (
          <EmptyState icon="📚" text="No articles found" subtext={`No results for "${query}"`} />
        )}

        {!loading && articles.length > 0 && (
          <>
            <div className="glpi-text-small glpi-mb-10">{total} article{total !== 1 ? 's' : ''} found</div>
            {articles.map((a) => (
              <div key={a.ID} className="glpi-card" style={{ cursor: 'pointer' }}>
                <div style={{ fontWeight: 600, fontSize: 13, marginBottom: 4 }}>{a.Subject}</div>
                <div className="glpi-text-small">ID: {a.ID}</div>
              </div>
            ))}
          </>
        )}
      </div>
    </div>
  );
}
