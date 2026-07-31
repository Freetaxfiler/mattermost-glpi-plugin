import React, { useState, useCallback, useEffect } from 'react';
import type { KnowledgeSummary, KnowledgeArticle, KnowbaseCategorySummary } from '../types';
import { api } from '../api';
import Loading from './common/Loading';
import ErrorState from './common/ErrorState';
import EmptyState from './common/EmptyState';
import { sanitizeHtml } from '../sanitize';

const PAGE_SIZE = 15;

export default function KnowledgeBase() {
  const [query, setQuery] = useState('');
  const [glpiUrl, setGlpiUrl] = useState('');
  const [categories, setCategories] = useState<KnowbaseCategorySummary[]>([]);
  const [categoryFilter, setCategoryFilter] = useState(0);

  useEffect(() => {
    api.getConfig().then((c) => setGlpiUrl(c.glpi_url || '')).catch(() => {});
    api.getKnowledgeCategories().then((r) => setCategories(r.categories)).catch(() => {});
  }, []);
  const [articles, setArticles] = useState<KnowledgeSummary[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searched, setSearched] = useState(false);
  const [selected, setSelected] = useState<KnowledgeArticle | null>(null);
  const [articleLoading, setArticleLoading] = useState(false);
  const [articleError, setArticleError] = useState<string | null>(null);

  const handleSearch = useCallback(async (pageNum: number) => {
    if (!query.trim()) return;
    setLoading(true);
    setError(null);
    setSearched(true);
    try {
      const result = await api.searchKnowledge(query.trim(), pageNum, PAGE_SIZE, categoryFilter || undefined);
      setArticles(result.articles);
      setTotal(result.total);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Search failed');
    } finally {
      setLoading(false);
    }
  }, [query, categoryFilter]);

  const openArticle = useCallback(async (id: number) => {
    setArticleLoading(true);
    setArticleError(null);
    try {
      const article = await api.getKnowledgeArticle(id);
      setSelected(article);
    } catch (err: unknown) {
      setArticleError(err instanceof Error ? err.message : 'Failed to load article');
    } finally {
      setArticleLoading(false);
    }
  }, []);

  const closeArticle = () => {
    setSelected(null);
    setArticleError(null);
  };

  if (selected) {
    return (
      <div>
        <div className="glpi-section">
          <div className="glpi-section-title">Article</div>
          <div className="glpi-card">
            <div style={{ fontSize: 16, fontWeight: 600, marginBottom: 4 }}>{selected.subject}</div>
            {selected.category && (
              <div className="glpi-text-small glpi-mb-10">Category: {selected.category}</div>
            )}
            {selected.date && (
              <div className="glpi-text-small glpi-mb-10">Updated: {selected.date}</div>
            )}
            {articleError && <div className="glpi-form-error glpi-mb-10">{articleError}</div>}
            {articleLoading && <Loading text="Loading article..." />}
            {!articleLoading && selected.content && (
              <div
                style={{ fontSize: 13, lineHeight: 1.7, wordBreak: 'break-word' }}
                // Sanitized before rendering; GLPI articles are trusted rich text.
                dangerouslySetInnerHTML={{ __html: sanitizeHtml(selected.content) }}
              />
            )}
          </div>
          <div className="glpi-flex glpi-gap-8 glpi-mt-10">
            <button className="glpi-btn glpi-btn-secondary glpi-btn-sm" onClick={closeArticle}>
              ← Back to results
            </button>
            {glpiUrl && (
              <a
                className="glpi-btn glpi-btn-primary glpi-btn-sm"
                style={{ textDecoration: 'none' }}
                href={`${glpiUrl}/front/knowbaseitem.form.php?id=${selected.id}`}
                target="_blank"
                rel="noreferrer"
              >
                Open in GLPI
              </a>
            )}
          </div>
        </div>
      </div>
    );
  }

  const totalPages = Math.ceil(total / PAGE_SIZE);

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
            onKeyDown={(e) => { if (e.key === 'Enter') { setPage(1); handleSearch(1); } }}
            placeholder="Search knowledge base..."
            maxLength={100}
          />
          <button
            className="glpi-btn glpi-btn-primary glpi-btn-sm"
            onClick={() => { setPage(1); handleSearch(1); }}
            disabled={loading || !query.trim()}
          >
            {loading ? '...' : 'Search'}
          </button>
          <button
            className="glpi-btn glpi-btn-secondary glpi-btn-sm"
            onClick={() => handleSearch(page)}
            disabled={loading || !searched}
          >
            ↻ Refresh
          </button>
        </div>

        {categories.length > 0 && (
          <div className="glpi-form-group">
            <label className="glpi-label">Category</label>
            <select
              className="glpi-select"
              value={categoryFilter}
              onChange={(e) => {
                setCategoryFilter(Number(e.target.value));
                setPage(1);
                if (searched) handleSearch(1);
              }}
            >
              <option value={0}>All categories</option>
              {categories.map((c) => (
                <option key={c.ID} value={c.ID}>{c.Name}</option>
              ))}
            </select>
          </div>
        )}

        {loading && <Loading text="Searching..." />}
        {error && <ErrorState message={error} onRetry={() => handleSearch(page)} />}

        {!loading && searched && articles.length === 0 && (
          <EmptyState icon="📚" text="No articles found" subtext={`No results for "${query}"`} />
        )}

        {!loading && articles.length > 0 && (
          <>
            <div className="glpi-text-small glpi-mb-10">{total} article{total !== 1 ? 's' : ''} found</div>
            {articles.map((a) => (
              <div key={a.ID} className="glpi-card" style={{ cursor: 'pointer' }} onClick={() => openArticle(a.ID)}>
                <div style={{ fontWeight: 600, fontSize: 13, marginBottom: 4 }}>{a.Subject}</div>
                <div className="glpi-text-small">ID: {a.ID}</div>
              </div>
            ))}

            {totalPages > 1 && (
              <div className="glpi-flex glpi-flex-center" style={{ justifyContent: 'center', gap: 6, marginTop: 12 }}>
                <button
                  className="glpi-btn glpi-btn-sm glpi-btn-secondary"
                  disabled={page <= 1}
                  onClick={() => { const p = Math.max(1, page - 1); setPage(p); handleSearch(p); }}
                >
                  ← Prev
                </button>
                <span className="glpi-text-small">Page {page} of {totalPages}</span>
                <button
                  className="glpi-btn glpi-btn-sm glpi-btn-secondary"
                  disabled={page >= totalPages}
                  onClick={() => { const p = page + 1; setPage(p); handleSearch(p); }}
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
