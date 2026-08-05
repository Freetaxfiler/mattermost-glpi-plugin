package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/handlers"
	"github.com/mattermost/mattermost/server/public/model"
)

// NotificationService centralizes all notification logic.
type NotificationService struct {
	p *Plugin
}

// formatWebhookMessage renders a GLPI webhook event as a Mattermost markdown
// message for channel posts and DMs.
func formatWebhookMessage(event handlers.WebhookEvent) string {
	var b strings.Builder
	b.WriteString("🔔 **GLPI** — ")
	b.WriteString(event.Event)

	if event.TicketID > 0 {
		b.WriteString(fmt.Sprintf("\nTicket #%d", event.TicketID))
	}
	if event.Title != "" {
		b.WriteString(": " + event.Title)
	}
	if event.Status != "" {
		b.WriteString("\nStatus: " + event.Status)
	}
	if event.URL != "" {
		b.WriteString("\n" + event.URL)
	}
	return b.String()
}

func NewNotificationService(p *Plugin) *NotificationService {
	return &NotificationService{p: p}
}

// NotifyTicketCreated records the event in the persistent store and pushes a
// websocket event for live UI refresh.  The caller must separately invoke
// NotifyAdminsTicketCreated when it has full ticket details available (the
// service does not carry them).
func (s *NotificationService) NotifyTicketCreated(ticketID int, subject string) {
	s.p.recordNotification(Notification{
		ID:        fmt.Sprintf("n-%d-%d", time.Now().UnixNano(), ticketID),
		Type:      "ticket_created",
		TicketID:  ticketID,
		Title:     subject,
		Status:    "New",
		CreatedAt: time.Now().Unix(),
	})

	// Push a WebSocket event so open sidebars refresh their badge/views in
	// near real time. The Mattermost server prefixes plugin events with
	// custom_<pluginId>_; the webapp subscribes to that prefixed name.
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

	message := formatWebhookMessage(event)
	config := s.p.currentConfiguration()

	posted := false
	if config != nil && strings.TrimSpace(config.NotificationChannelID) != "" {
		if ok := s.postToChannel(strings.TrimSpace(config.NotificationChannelID), message); ok {
			posted = true
		}
	}

	if event.RequesterEmail != "" {
		if ok := s.dmRequester(event.RequesterEmail, message); ok {
			posted = true
		}
	}

	if !posted {
		s.p.API.LogInfo("GLPI webhook received but no notification target was available", "event", event.Event, "ticket_id", event.TicketID)
	}
}

// postToChannel posts a message from the GLPI bot to a channel. It returns
// true when the post was created successfully.
func (s *NotificationService) postToChannel(channelID, message string) bool {
	if s.p.botUserID == "" || strings.TrimSpace(channelID) == "" {
		return false
	}
	post := &model.Post{
		UserId:    s.p.botUserID,
		ChannelId: strings.TrimSpace(channelID),
		Message:   message,
	}
	if _, appErr := s.p.API.CreatePost(post); appErr != nil {
		s.p.API.LogWarn("failed to post GLPI notification to channel", "err", appErr.Error())
		return false
	}
	return true
}

// dmRequester DMs the GLPI bot's notification to the Mattermost user matching
// the requester email, if one exists. It returns true when delivered.
func (s *NotificationService) dmRequester(email, message string) bool {
	if s.p.botUserID == "" || strings.TrimSpace(email) == "" {
		return false
	}
	user, appErr := s.p.API.GetUserByEmail(strings.TrimSpace(email))
	if appErr != nil || user == nil {
		return false
	}
	channel, appErr := s.p.API.GetDirectChannel(s.p.botUserID, user.Id)
	if appErr != nil || channel == nil {
		return false
	}
	post := &model.Post{
		UserId:    s.p.botUserID,
		ChannelId: channel.Id,
		Message:   message,
	}
	if _, appErr := s.p.API.CreatePost(post); appErr != nil {
		s.p.API.LogWarn("failed to DM GLPI notification", "err", appErr.Error())
		return false
	}
	return true
}

// TicketCreatedDetails carries the additional context needed to notify the
// IT admin channel and the requester about a newly-created ticket.
type TicketCreatedDetails struct {
	TicketID    int
	Subject     string
	CreatorName string
	CreatorID   string
	Priority    string
	Category    string
	Status      string
	GLPIURL     string
}

// NotifyAdminsTicketCreated posts a new-ticket notification to the configured
// IT admin channel (NotificationChannelID) and DMs the requester, so support
// staff are alerted as soon as an employee files a ticket.
func (s *NotificationService) NotifyAdminsTicketCreated(details TicketCreatedDetails) {
	config := s.p.currentConfiguration()
	if config == nil {
		return
	}

	var b strings.Builder
	b.WriteString("🎫 **New ticket #" + strconv.Itoa(details.TicketID) + "**\n")
	if details.Subject != "" {
		b.WriteString("**" + details.Subject + "**\n")
	}
	if details.CreatorName != "" {
		b.WriteString("Employee: " + details.CreatorName + "\n")
	}
	if details.Priority != "" {
		b.WriteString("Priority: " + details.Priority + "\n")
	}
	if details.Category != "" {
		b.WriteString("Category: " + details.Category + "\n")
	}
	if details.Status != "" {
		b.WriteString("Status: " + details.Status + "\n")
	}
	if details.GLPIURL != "" {
		b.WriteString(details.GLPIURL)
	}
	message := b.String()

	// Admin channel (best-effort; failures are logged, not fatal)
	if config.NotificationChannelID != "" {
		s.postToChannel(config.NotificationChannelID, message)
	}

	// DM the requester with the same summary (resolve email via MM identity)
	if details.CreatorID != "" {
		if user, appErr := s.p.API.GetUser(details.CreatorID); appErr == nil && user != nil && user.Email != "" {
			s.dmRequester(user.Email, message)
		}
	}
}
