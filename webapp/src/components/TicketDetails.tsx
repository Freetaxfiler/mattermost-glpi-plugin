import React, { useState, useEffect, useCallback } from 'react';
import type { ViewName, Ticket, TimelinePage, TimelineEvent } from '../types';
import { api } from '../api';
import Loading from './common/Loading';
import ErrorState from './common/ErrorState';
import { StatusBadge } from './common/StatusBadge';
import ConfirmDialog from './common/ConfirmDialog';

interface TicketDetailsProps {
  ticketId: number;
  onNavigate: (view: ViewName) => void;
}

export default function TicketDetails({ ticketId, onNavigate }: TicketDetailsProps) {
  const [ticket, setTicket] = useState<Ticket | null>(null);
  const [timeline, setTimeline] = useState<TimelinePage | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [commentText, setCommentText] = useState('');
  const [isPrivate, setIsPrivate] = useState(false);
  const [solutionText, setSolutionText] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [confirmClose, setConfirmClose] = useState(false);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [t, tl] = await Promise.all([
        api.getTicket(ticketId),
        api.getTimeline(ticketId),
      ]);
      setTicket(t);
      setTimeline(tl);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load ticket');
    } finally {
      setLoading(false);
    }
  }, [ticketId]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handleAddComment = async () => {
    if (!commentText.trim()) return;
    setSubmitting(true);
    try {
      await api.addFollowup(ticketId, commentText.trim(), isPrivate);
      setCommentText('');
      setSuccessMsg('Comment added');
      fetchData();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to add comment');
    } finally {
      setSubmitting(false);
    }
  };

  const handleAddSolution = async () => {
    if (!solutionText.trim()) return;
    setSubmitting(true);
    try {
      await api.addSolution(ticketId, solutionText.trim());
      setSolutionText('');
      setSuccessMsg('Solution recorded');
      fetchData();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to record solution');
    } finally {
      setSubmitting(false);
    }
  };

  const handleClose = async () => {
    setConfirmClose(false);
    setSubmitting(true);
    try {
      await api.updateTicket(ticketId, { status: 6 });
      setSuccessMsg('Ticket closed');
      fetchData();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to close ticket');
    } finally {
      setSubmitting(false);
    }
  };

  const handleReopen = async () => {
    setSubmitting(true);
    try {
      await api.updateTicket(ticketId, { status: 2 });
      setSuccessMsg('Ticket reopened');
      fetchData();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to reopen ticket');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async () => {
    setConfirmDelete(false);
    setSubmitting(true);
    try {
      await api.deleteTicket(ticketId);
      setSuccessMsg('Ticket deleted');
      onNavigate('my-tickets');
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to delete ticket');
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) return <Loading text="Loading ticket..." />;
  if (error) return <ErrorState message={error} onRetry={fetchData} />;
  if (!ticket) return <ErrorState message="Ticket not found" />;

  const isClosed = ticket.status === 6;
  const isSolved = ticket.status === 5;

  return (
    <div>
      {/* Success toast */}
      {successMsg && (
        <div className="glpi-toast glpi-toast-success" onClick={() => setSuccessMsg(null)}>
          {successMsg}
        </div>
      )}

      {/* Header */}
      <div className="glpi-section">
        <div className="glpi-flex glpi-flex-between glpi-flex-center" style={{ marginBottom: 10 }}>
          <div>
            <div style={{ fontSize: 18, fontWeight: 600 }}>#{ticket.id}</div>
            <div style={{ fontSize: 14, fontWeight: 500, marginTop: 2 }}>{ticket.name}</div>
          </div>
          <StatusBadge status={ticket.status} />
        </div>

        <div className="glpi-card">
          <div className="glpi-flex glpi-flex-between glpi-flex-center">
            <span style={{ fontSize: 12, color: 'var(--center-channel-color-60)' }}>Priority</span>
            <span style={{ fontSize: 13 }}>{['', 'Very low', 'Low', 'Medium', 'High', 'Very high'][ticket.priority] || ticket.priority}</span>
          </div>
          <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
            <span style={{ fontSize: 12, color: 'var(--center-channel-color-60)' }}>Urgency</span>
            <span style={{ fontSize: 13 }}>{['', 'Very low', 'Low', 'Medium', 'High', 'Very high'][ticket.urgency] || ticket.urgency}</span>
          </div>
          <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
            <span style={{ fontSize: 12, color: 'var(--center-channel-color-60)' }}>Date</span>
            <span style={{ fontSize: 13 }}>{ticket.date}</span>
          </div>
          <div className="glpi-flex glpi-flex-between glpi-flex-center glpi-mt-10">
            <span style={{ fontSize: 12, color: 'var(--center-channel-color-60)' }}>Last modified</span>
            <span style={{ fontSize: 13 }}>{ticket.date_mod}</span>
          </div>
        </div>
      </div>

      {/* Description */}
      {ticket.content && (
        <div className="glpi-section">
          <div className="glpi-section-title">Description</div>
          <div className="glpi-card">
            <div style={{ fontSize: 13, lineHeight: 1.6, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
              {stripHtml(ticket.content)}
            </div>
          </div>
        </div>
      )}

      {/* Timeline */}
      {timeline && timeline.Events.length > 0 && (
        <div className="glpi-section">
          <div className="glpi-section-title">Timeline ({timeline.Total})</div>
          {timeline.Events.slice(0, 20).map((event: TimelineEvent) => (
            <div key={`${event.Kind}-${event.ID}`} className="glpi-timeline-event">
              <div className="glpi-timeline-event-meta">
                {event.Kind}{event.IsPrivate ? ' (private)' : ''} — {event.Author || 'System'} — {event.Date}
              </div>
              <div className="glpi-timeline-event-content">
                {stripHtml(event.Content).substring(0, 300)}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Add comment */}
      <div className="glpi-section">
        <div className="glpi-section-title">Add Follow-up</div>
        <textarea
          className="glpi-textarea"
          value={commentText}
          onChange={(e) => setCommentText(e.target.value)}
          placeholder="Write a comment..."
          rows={3}
        />
        <div className="glpi-flex glpi-flex-center glpi-gap-8 glpi-mt-10">
          <label className="glpi-flex glpi-flex-center glpi-gap-4" style={{ fontSize: 13, cursor: 'pointer' }}>
            <input
              type="checkbox"
              checked={isPrivate}
              onChange={(e) => setIsPrivate(e.target.checked)}
            />
            Private
          </label>
          <button
            className="glpi-btn glpi-btn-primary glpi-btn-sm"
            onClick={handleAddComment}
            disabled={submitting || !commentText.trim()}
          >
            {submitting ? 'Posting...' : 'Post'}
          </button>
        </div>
      </div>

      {/* Solution / Close */}
      {!isClosed && (
        <div className="glpi-section">
          <div className="glpi-section-title">Solution</div>
          <textarea
            className="glpi-textarea"
            value={solutionText}
            onChange={(e) => setSolutionText(e.target.value)}
            placeholder="Solution text (optional)"
            rows={2}
          />
          <div className="glpi-flex glpi-gap-8 glpi-mt-10">
            <button
              className="glpi-btn glpi-btn-success glpi-btn-sm"
              onClick={() => setConfirmClose(true)}
              disabled={submitting}
            >
              Close with Solution
            </button>
            {isSolved && (
              <button
                className="glpi-btn glpi-btn-secondary glpi-btn-sm"
                onClick={handleReopen}
                disabled={submitting}
              >
                Reopen
              </button>
            )}
          </div>
        </div>
      )}

      {/* Reopen for closed tickets */}
      {isClosed && (
        <div className="glpi-section">
          <button
            className="glpi-btn glpi-btn-primary glpi-btn-sm"
            onClick={handleReopen}
            disabled={submitting}
          >
            Reopen Ticket
          </button>
        </div>
      )}

      {/* Delete */}
      <div className="glpi-section">
        <button
          className="glpi-btn glpi-btn-danger glpi-btn-sm"
          onClick={() => setConfirmDelete(true)}
          disabled={submitting}
        >
          Delete Ticket
        </button>
      </div>

      {/* Confirm dialogs */}
      {confirmDelete && (
        <ConfirmDialog
          title="Delete Ticket"
          message={`Are you sure you want to delete ticket #${ticketId}? It will be moved to the GLPI trash.`}
          confirmText="Delete"
          variant="danger"
          onConfirm={handleDelete}
          onCancel={() => setConfirmDelete(false)}
        />
      )}

      {confirmClose && (
        <ConfirmDialog
          title="Close Ticket"
          message={solutionText.trim() ? 'Record the solution and close this ticket?' : 'Close this ticket without a solution?'}
          confirmText="Close"
          onConfirm={() => {
            if (solutionText.trim()) {
              api.addSolution(ticketId, solutionText.trim()).then(() => {
                api.updateTicket(ticketId, { status: 6 }).then(() => {
                  setSolutionText('');
                  setSuccessMsg('Ticket closed with solution');
                  fetchData();
                });
              }).catch(() => setError('Failed to close ticket'));
            } else {
              handleClose();
            }
          }}
          onCancel={() => setConfirmClose(false)}
        />
      )}

      {/* Back button */}
      <div className="glpi-mt-10">
        <button className="glpi-btn glpi-btn-secondary glpi-btn-sm" onClick={() => onNavigate('my-tickets')}>
          ← Back to Tickets
        </button>
      </div>
    </div>
  );
}

function stripHtml(html: string): string {
  if (!html) return '';
  const div = document.createElement('div');
  div.innerHTML = html;
  return div.textContent || div.innerText || '';
}
