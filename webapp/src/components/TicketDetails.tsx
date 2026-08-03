import React, { useState, useEffect, useCallback } from 'react';
import type { ViewName, Ticket, TimelinePage, TimelineEvent, DocumentInfo } from '../types';
import { api } from '../api';
import Loading from './common/Loading';
import ErrorState from './common/ErrorState';
import { StatusBadge } from './common/StatusBadge';
import ConfirmDialog from './common/ConfirmDialog';

interface TicketDetailsProps {
  ticketId: number;
  onNavigate: (view: ViewName) => void;
  role?: string;
  isSystemAdmin?: boolean;
}

const PRIORITY_OPTIONS = [
  { value: 1, label: 'Very low' },
  { value: 2, label: 'Low' },
  { value: 3, label: 'Medium' },
  { value: 4, label: 'High' },
  { value: 5, label: 'Very high' },
];

function formatBytes(bytes: number): string {
  if (!bytes) return '—';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export default function TicketDetails({ ticketId, onNavigate, role, isSystemAdmin }: TicketDetailsProps) {
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

  // "Assign to me" is a technician-only action. Hide it for definite
  // employees; unknown roles and system admins keep it for backwards
  // compatibility with deployments where role detection is not configured.
  const canAssignToMe = !!isSystemAdmin || role !== 'employee';

  // Edit form
  const [editing, setEditing] = useState(false);
  const [editTitle, setEditTitle] = useState('');
  const [editPriority, setEditPriority] = useState(3);
  const [editUrgency, setEditUrgency] = useState(3);
  const [editImpact, setEditImpact] = useState(3);

  // Attachments
  const [documents, setDocuments] = useState<DocumentInfo[]>([]);
  const [uploading, setUploading] = useState(false);
  const [dragOver, setDragOver] = useState(false);

  // Timeline
  const [timelineFilter, setTimelineFilter] = useState<string>('');

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

  const loadDocuments = useCallback(async () => {
    try {
      const r = await api.listTicketDocuments(ticketId);
      setDocuments(r.documents);
    } catch {
      setDocuments([]);
    }
  }, [ticketId]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  useEffect(() => {
    loadDocuments();
  }, [loadDocuments]);

  const beginEdit = () => {
    if (!ticket) return;
    setEditTitle(ticket.name);
    setEditPriority(ticket.priority || 3);
    setEditUrgency(ticket.urgency || 3);
    setEditImpact(ticket.impact || 3);
    setEditing(true);
  };

  const saveEdit = async () => {
    if (!editTitle.trim()) {
      setError('Title is required');
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await api.updateTicket(ticketId, {
        name: editTitle.trim(),
        priority: editPriority,
        urgency: editUrgency,
        impact: editImpact,
      });
      setEditing(false);
      setSuccessMsg('Ticket updated');
      fetchData();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to update ticket');
    } finally {
      setSubmitting(false);
    }
  };

  const handleUpload = async (file: File | null) => {
    if (!file) return;
    setUploading(true);
    setError(null);
    try {
      await api.attachFile(ticketId, file);
      setSuccessMsg('Attachment added');
      loadDocuments();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to upload attachment');
    } finally {
      setUploading(false);
    }
  };

  const handleDownload = async (doc: DocumentInfo) => {
    try {
      await api.downloadDocument(ticketId, doc.id, doc.filename || doc.name);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to download attachment');
    }
  };

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

  const handleAssignToMe = async () => {
    setSubmitting(true);
    setError(null);
    try {
      const user = await api.getUser();
      if (!user.glpi_user_id) {
        setError('Your Mattermost account is not linked to a GLPI user');
        return;
      }
      await api.updateTicket(ticketId, { _users_id_technician: user.glpi_user_id });
      setSuccessMsg('Ticket assigned to you');
      fetchData();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to assign ticket');
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
          <div className="glpi-flex glpi-gap-8">
            <StatusBadge status={ticket.status} />
            {canAssignToMe && (
              <button
                className="glpi-btn glpi-btn-secondary glpi-btn-sm"
                onClick={handleAssignToMe}
                disabled={submitting}
              >
                Assign to me
              </button>
            )}
            <button
              className="glpi-btn glpi-btn-secondary glpi-btn-sm"
              onClick={beginEdit}
              disabled={submitting}
            >
              Edit
            </button>
          </div>
        </div>

        {editing ? (
          <div className="glpi-card">
            <div className="glpi-form-group">
              <label className="glpi-label">Title</label>
              <input
                className="glpi-input"
                type="text"
                value={editTitle}
                onChange={(e) => setEditTitle(e.target.value)}
                maxLength={200}
              />
            </div>
            <div className="glpi-form-group">
              <label className="glpi-label">Priority</label>
              <select className="glpi-select" value={editPriority} onChange={(e) => setEditPriority(Number(e.target.value))}>
                {PRIORITY_OPTIONS.map((opt) => (
                  <option key={opt.value} value={opt.value}>{opt.label}</option>
                ))}
              </select>
            </div>
            <div className="glpi-form-group">
              <label className="glpi-label">Urgency</label>
              <select className="glpi-select" value={editUrgency} onChange={(e) => setEditUrgency(Number(e.target.value))}>
                {PRIORITY_OPTIONS.map((opt) => (
                  <option key={opt.value} value={opt.value}>{opt.label}</option>
                ))}
              </select>
            </div>
            <div className="glpi-form-group">
              <label className="glpi-label">Impact</label>
              <select className="glpi-select" value={editImpact} onChange={(e) => setEditImpact(Number(e.target.value))}>
                {PRIORITY_OPTIONS.map((opt) => (
                  <option key={opt.value} value={opt.value}>{opt.label}</option>
                ))}
              </select>
            </div>
            <div className="glpi-flex glpi-gap-8">
              <button className="glpi-btn glpi-btn-primary glpi-btn-sm" onClick={saveEdit} disabled={submitting}>
                {submitting ? 'Saving...' : 'Save'}
              </button>
              <button className="glpi-btn glpi-btn-secondary glpi-btn-sm" onClick={() => setEditing(false)}>
                Cancel
              </button>
            </div>
          </div>
        ) : (
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
              <span style={{ fontSize: 12, color: 'var(--center-channel-color-60)' }}>Impact</span>
              <span style={{ fontSize: 13 }}>{['', 'Very low', 'Low', 'Medium', 'High', 'Very high'][ticket.impact] || ticket.impact}</span>
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
        )}
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

      {/* Attachments */}
      <div className="glpi-section">
        <div className="glpi-section-title">Attachments ({documents.length})</div>
        <div
          className="glpi-card"
          style={dragOver ? { borderColor: 'var(--button-bg)', borderStyle: 'dashed' } : undefined}
          onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
          onDragLeave={() => setDragOver(false)}
          onDrop={(e) => {
            e.preventDefault();
            setDragOver(false);
            handleUpload(e.dataTransfer.files?.[0] ?? null);
          }}
        >
          {documents.length === 0 && (
            <div className="glpi-text-small">No attachments.</div>
          )}
          {documents.map((doc) => (
            <div key={doc.id} className="glpi-flex glpi-flex-between glpi-flex-center glpi-mb-10">
              <div style={{ minWidth: 0 }}>
                {doc.mime_type && doc.mime_type.startsWith('image/') && (
                  <img
                    src={`/plugins/com.ntas.glpi/api/v1/tickets/${ticketId}/documents/${doc.id}`}
                    alt={doc.name || doc.filename}
                    style={{ maxWidth: 80, maxHeight: 60, borderRadius: 4, display: 'block', marginBottom: 4, cursor: 'pointer' }}
                    onClick={() => handleDownload(doc)}
                  />
                )}
                <div style={{ fontSize: 13, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {doc.name || doc.filename}
                </div>
                <div className="glpi-text-small">{formatBytes(doc.size)}</div>
              </div>
              <button
                className="glpi-btn glpi-btn-secondary glpi-btn-sm"
                onClick={() => handleDownload(doc)}
              >
                Download
              </button>
            </div>
          ))}
          <div className="glpi-mt-10">
            <label className="glpi-btn glpi-btn-secondary glpi-btn-sm" style={{ display: 'inline-block', cursor: 'pointer' }}>
              {uploading ? 'Uploading...' : 'Upload Attachment'}
              <input
                type="file"
                style={{ display: 'none' }}
                onChange={(e) => {
                  handleUpload(e.target.files?.[0] ?? null);
                  e.target.value = '';
                }}
              />
            </label>
            <span className="glpi-text-small" style={{ marginLeft: 8 }}>
              {uploading ? 'Uploading...' : 'or drag & drop a file here'}
            </span>
          </div>
        </div>
      </div>

      {/* Timeline */}
      {timeline && timeline.Events.length > 0 && (
        <div className="glpi-section">
          <div className="glpi-flex glpi-flex-between glpi-flex-center">
            <div className="glpi-section-title">Timeline ({timeline.Total})</div>
            <select
              className="glpi-select"
              style={{ width: 'auto', fontSize: 12 }}
              value={timelineFilter}
              onChange={(e) => setTimelineFilter(e.target.value)}
              aria-label="Filter timeline by type"
            >
              <option value="">All events</option>
              <option value="followup">Follow-ups</option>
              <option value="solution">Solutions</option>
              <option value="validation">Approvals</option>
              <option value="history">History</option>
            </select>
          </div>
          {timeline.Events.filter((e) => !timelineFilter || e.Kind === timelineFilter).slice(0, 20).map((event: TimelineEvent) => (
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
              Record Solution
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
          title={solutionText.trim() ? 'Record Solution' : 'Close Ticket'}
          message={solutionText.trim() ? 'Record the solution and let GLPI process the ticket closure?' : 'Close this ticket without a solution?'}
          confirmText={solutionText.trim() ? 'Record Solution' : 'Close'}
          onConfirm={() => {
            if (solutionText.trim()) {
              api.addSolution(ticketId, solutionText.trim()).then(() => {
                setSolutionText('');
                setSuccessMsg('Solution recorded — GLPI will process the closure');
                fetchData();
              }).catch(() => setError('Failed to record solution'));
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
