package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/handlers"
	"github.com/mattermost/mattermost/server/public/model"
)

// NotificationService centralizes all notification logic.
type NotificationService struct {
	p *Plugin
}

func NewNotificationService(p *Plugin) *NotificationService {
	return &NotificationService{p: p}
}

// NotifyTicketCreated centralizes all ticket-creation notifications.
// It records the event in the persistent store, pushes a websocket event,
// posts to the configured channel, and DMs the requester if matched.
func (s *NotificationService) NotifyTicketCreated(ticketID int, subject string) {
	s.p.recordTicketCreatedNotification(ticketID, subject)

	// Emit websocket event for live UI refresh
	s.publishTicketEvent("ticket_created", map[string]any{
		"ticket_id": ticketID,
		"subject":   subject,
	})
}

// NotifyTicketUpdated notifies about ticket status/priority/etc. changes.
func (s *NotificationService) NotifyTicketUpdated(ticketID int, field, oldVal, newVal string) {
	s.p.recordNotification(Notification{
		ID:        fmt.Sprintf("n-%d-%d", time.Now().UnixNano(), ticketID),
		Type:      "ticket_updated",
		TicketID:  ticketID,
		Title:     fmt.Sprintf("%s changed: %s → %s", field, oldVal, newVal),
		Status:    newVal,
		CreatedAt: time.Now().Unix(),
	})
	s.publishTicketEvent("ticket_updated", map[string]any{
		"ticket_id": ticketID,
		"field":     field,
		"old_value": oldVal,
		"new_value": newVal,
	})
}

// NotifyTicketAssigned notifies when a ticket is assigned to a technician.
func (s *NotificationService) NotifyTicketAssigned(ticketID int, assigneeGLPIUserID int) {
	s.p.recordNotification(Notification{
		ID:        fmt.Sprintf("n-%d-%d", time.Now().UnixNano(), ticketID),
		Type:      "ticket_assigned",
		TicketID:  ticketID,
		Title:     "Ticket assigned",
		CreatedAt: time.Now().Unix(),
	})
	s.publishTicketEvent("ticket_assigned", map[string]any{
		"ticket_id": ticketID,
		"assignee":  assigneeGLPIUserID,
	})
}

// NotifyFollowupAdded notifies when a followup is added to a ticket.
func (s *NotificationService) NotifyFollowupAdded(ticketID int, author string, isPrivate bool) {
	s.p.recordNotification(Notification{
		ID:        fmt.Sprintf("n-%d-%d", time.Now().UnixNano(), ticketID),
		Type:      "followup_added",
		TicketID:  ticketID,
		Title:     "Follow-up added",
		Status:    map[bool]string{true: "Private", false: "Public"}[isPrivate],
		CreatedAt: time.Now().Unix(),
	})
	s.publishTicketEvent("followup_added", map[string]any{
		"ticket_id":  ticketID,
		"author":     author,
		"is_private": isPrivate,
	})
}

// NotifySolutionAdded notifies when a solution is added.
func (s *NotificationService) NotifySolutionAdded(ticketID int) {
	s.p.recordNotification(Notification{
		ID:        fmt.Sprintf("n-%d-%d", time.Now().UnixNano(), ticketID),
		Type:      "solution_added",
		TicketID:  ticketID,
		Title:     "Solution recorded",
		CreatedAt: time.Now().Unix(),
	})
	s.publishTicketEvent("solution_added", map[string]any{
		"ticket_id": ticketID,
	})
}

// NotifyTicketClosed notifies when a ticket is closed.
func (s *NotificationService) NotifyTicketClosed(ticketID int) {
	s.p.recordNotification(Notification{
		ID:        fmt.Sprintf("n-%d-%d", time.Now().UnixNano(), ticketID),
		Type:      "ticket_closed",
		TicketID:  ticketID,
		Title:     "Ticket closed",
		Status:    "Closed",
		CreatedAt: time.Now().Unix(),
	})
	s.publishTicketEvent("ticket_closed", map[string]any{
		"ticket_id": ticketID,
	})
}

// NotifyTicketReopened notifies when a ticket is reopened.
func (s *NotificationService) NotifyTicketReopened(ticketID int) {
	s.p.recordNotification(Notification{
		ID:        fmt.Sprintf("n-%d-%d", time.Now().UnixNano(), ticketID),
		Type:      "ticket_reopened",
		TicketID:  ticketID,
		Title:     "Ticket reopened",
		Status:    "Reopened",
		CreatedAt: time.Now().Unix(),
	})
	s.publishTicketEvent("ticket_reopened", map[string]any{
		"ticket_id": ticketID,
	})
}

// NotifyAttachmentAdded notifies when an attachment is uploaded to a ticket.
func (s *NotificationService) NotifyAttachmentAdded(ticketID int, filename string) {
	s.p.recordNotification(Notification{
		ID:        fmt.Sprintf("n-%d-%d", time.Now().UnixNano(), ticketID),
		Type:      "attachment_added",
		TicketID:  ticketID,
		Title:     "Attachment added: " + filename,
		CreatedAt: time.Now().Unix(),
	})
	s.publishTicketEvent("attachment_added", map[string]any{
		"ticket_id": ticketID,
		"filename":  filename,
	})
}

// publishTicketEvent emits a websocket event for live UI refresh.
// Event names are prefixed with "custom_<pluginId>_" by the Mattermost server.
func (s *NotificationService) publishTicketEvent(eventType string, payload map[string]any) {
	s.p.API.PublishWebSocketEvent(
		eventType,
		payload,
		&model.WebsocketBroadcast{},
	)
}

// NotifyWebhookEvent handles incoming GLPI webhook events and routes them
// to the appropriate notification channels.
func (s *NotificationService) NotifyWebhookEvent(event handlers.WebhookEvent) {
	s.p.recordNotification(notificationFromWebhookEvent(event))
	s.publishTicketEvent("webhook_"+event.Event, map[string]any{
		"ticket_id": event.TicketID,
		"title":     event.Title,
		"status":    event.Status,
		"url":       event.URL,
	})

	// Route to appropriate Mattermost destinations
	message := formatWebhookMessage(event)
	config := s.p.currentConfiguration()

	posted := false
	if config != nil && strings.TrimSpace(config.NotificationChannelID) != "" && s.p.botUserID != "" {
		post := &model.Post{
			UserId:    s.p.botUserID,
			ChannelId: strings.TrimSpace(config.NotificationChannelID),
			Message:   message,
		}
		if _, appErr := s.p.API.CreatePost(post); appErr != nil {
			s.p.API.LogWarn("failed to post GLPI notification to channel", "err", appErr.Error())
		} else {
			posted = true
		}
	}

	if event.RequesterEmail != "" && s.p.botUserID != "" {
		if user, appErr := s.p.API.GetUserByEmail(event.RequesterEmail); appErr == nil && user != nil {
			if channel, appErr := s.p.API.GetDirectChannel(s.p.botUserID, user.Id); appErr == nil && channel != nil {
				post := &model.Post{
					UserId:    s.p.botUserID,
					ChannelId: channel.Id,
					Message:   message,
				}
				if _, appErr := s.p.API.CreatePost(post); appErr != nil {
					s.p.API.LogWarn("failed to DM GLPI notification", "err", appErr.Error())
				} else {
					posted = true
				}
			}
		}
	}

	if !posted {
		s.p.API.LogInfo("GLPI webhook received but no notification target was available", "event", event.Event, "ticket_id", event.TicketID)
	}
}

// recordOwnership is a thin wrapper kept for backwards compatibility
// (called from retry queue and legacy paths).
func (s *NotificationService) recordOwnership(userID string, ticketID int) {
	s.p.recordOwnership(userID, ticketID)
}
