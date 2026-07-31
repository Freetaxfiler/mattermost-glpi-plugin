import React, { useState, useEffect } from 'react';
import type { ViewName, CategorySummary } from '../types';
import { api } from '../api';

interface CreateTicketProps {
  onNavigate: (view: ViewName) => void;
}

const PRIORITY_OPTIONS = [
  { value: 1, label: 'Very low' },
  { value: 2, label: 'Low' },
  { value: 3, label: 'Medium' },
  { value: 4, label: 'High' },
  { value: 5, label: 'Very high' },
];

const REQUEST_TYPE_OPTIONS = [
  { value: 1, label: 'Incident' },
  { value: 2, label: 'Request' },
];

export default function CreateTicket({ onNavigate }: CreateTicketProps) {
  const [subject, setSubject] = useState('');
  const [content, setContent] = useState('');
  const [type, setType] = useState(1);
  const [priority, setPriority] = useState(3);
  const [urgency, setUrgency] = useState(3);
  const [categoryId, setCategoryId] = useState<number | 0>(0);
  const [categories, setCategories] = useState<CategorySummary[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<number | null>(null);

  useEffect(() => {
    api.getCategories()
      .then((r) => setCategories(r.categories))
      .catch(() => {});
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!subject.trim()) {
      setError('Subject is required');
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      const result = await api.createTicket({
        subject: subject.trim(),
        content: content.trim(),
        type,
        priority,
        urgency,
        category_id: categoryId || 0,
      });
      setSuccess(result.id);
      setSubject('');
      setContent('');
      setType(1);
      setPriority(3);
      setUrgency(3);
      setCategoryId(0);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to create ticket');
    } finally {
      setSubmitting(false);
    }
  };

  if (success) {
    return (
      <div>
        <div className="glpi-section">
          <div className="glpi-section-title">Ticket Created</div>
          <div className="glpi-card" style={{ textAlign: 'center', padding: 30 }}>
            <div style={{ fontSize: 40, marginBottom: 10 }}>✅</div>
            <div style={{ fontSize: 16, fontWeight: 600, marginBottom: 8 }}>
              Ticket #{success} created
            </div>
            <div className="glpi-flex" style={{ justifyContent: 'center', gap: 8, marginTop: 16 }}>
              <button
                className="glpi-btn glpi-btn-primary"
                onClick={() => onNavigate('ticket-details')}
              >
                View Ticket
              </button>
              <button
                className="glpi-btn glpi-btn-secondary"
                onClick={() => { setSuccess(null); setSubject(''); setContent(''); }}
              >
                Create Another
              </button>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div>
      <div className="glpi-section">
        <div className="glpi-section-title">Create New Ticket</div>
        <form onSubmit={handleSubmit}>
          <div className="glpi-form-group">
            <label className="glpi-label">Subject *</label>
            <input
              className="glpi-input"
              type="text"
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
              placeholder="Enter ticket subject"
              maxLength={200}
            />
          </div>

          <div className="glpi-form-group">
            <label className="glpi-label">Description</label>
            <textarea
              className="glpi-textarea"
              value={content}
              onChange={(e) => setContent(e.target.value)}
              placeholder="Describe the issue"
              rows={4}
            />
          </div>

          <div className="glpi-form-group">
            <label className="glpi-label">Request Type</label>
            <select
              className="glpi-select"
              value={type}
              onChange={(e) => setType(Number(e.target.value))}
            >
              {REQUEST_TYPE_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>{opt.label}</option>
              ))}
            </select>
          </div>

          <div className="glpi-form-group">
            <label className="glpi-label">Category</label>
            <select
              className="glpi-select"
              value={categoryId}
              onChange={(e) => setCategoryId(Number(e.target.value))}
            >
              <option value={0}>Default category</option>
              {categories.map((c) => (
                <option key={c.ID} value={c.ID}>{c.Name}</option>
              ))}
            </select>
          </div>

          <div className="glpi-form-group">
            <label className="glpi-label">Priority</label>
            <select
              className="glpi-select"
              value={priority}
              onChange={(e) => setPriority(Number(e.target.value))}
            >
              {PRIORITY_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>{opt.label}</option>
              ))}
            </select>
          </div>

          <div className="glpi-form-group">
            <label className="glpi-label">Urgency</label>
            <select
              className="glpi-select"
              value={urgency}
              onChange={(e) => setUrgency(Number(e.target.value))}
            >
              {PRIORITY_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>{opt.label}</option>
              ))}
            </select>
          </div>

          {error && <div className="glpi-form-error" style={{ marginBottom: 10 }}>{error}</div>}

          <div className="glpi-flex" style={{ gap: 8 }}>
            <button
              type="submit"
              className="glpi-btn glpi-btn-primary"
              disabled={submitting}
            >
              {submitting ? 'Creating...' : 'Create Ticket'}
            </button>
            <button
              type="button"
              className="glpi-btn glpi-btn-secondary"
              onClick={() => onNavigate('dashboard')}
            >
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
