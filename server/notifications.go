package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/handlers"
	"github.com/mattermost/mattermost/server/public/model"
)

const (
	notificationStoreKey        = "glpi_notifications"
	notificationReadKeyPrefix   = "glpi_notif_read_"
	notificationDismissedPrefix = "glpi_notif_dismissed_"
	maxStoredNotifications      = 100
)

// Notification is a persisted GLPI event shown in the notification center.
type Notification struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	TicketID  int    `json:"ticket_id,omitempty"`
	Title     string `json:"title"`
	Status    string `json:"status,omitempty"`
	URL       string `json:"url,omitempty"`
	CreatedAt int64  `json:"created_at"`
}

// recordNotification persists a notification (newest first) for the webapp
// notification center. Storage is best-effort: failures are logged and ignored.
func (p *Plugin) recordNotification(n Notification) {
	raw, _ := p.API.KVGet(notificationStoreKey)
	var list []Notification
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &list)
	}
	list = append([]Notification{n}, list...)
	if len(list) > maxStoredNotifications {
		list = list[:maxStoredNotifications]
	}
	if data, err := json.Marshal(list); err == nil {
		if appErr := p.API.KVSet(notificationStoreKey, data); appErr != nil {
			p.API.LogWarn("failed to persist notification", "err", appErr.Error())
		}
	}

	// Push a WebSocket event so open sidebars refresh their badge/views in
	// near real time. The Mattermost server prefixes plugin events with
	// custom_<pluginId>_; the webapp subscribes to that prefixed name.
	p.API.PublishWebSocketEvent(
		"notification",
		map[string]any{"type": n.Type, "ticket_id": n.TicketID},
		&model.WebsocketBroadcast{},
	)
}

// notificationFromWebhookEvent converts a GLPI webhook event into a persisted
// notification.
func notificationFromWebhookEvent(ev handlers.WebhookEvent) Notification {
	return Notification{
		ID:        fmt.Sprintf("n-%d-%d", time.Now().UnixNano(), ev.TicketID),
		Type:      ev.Event,
		TicketID:  ev.TicketID,
		Title:     ev.Title,
		Status:    ev.Status,
		URL:       ev.URL,
		CreatedAt: time.Now().Unix(),
	}
}

// recordTicketCreatedNotification persists a ticket-creation event for the
// notification center (used by the API/dialog create paths).
func (p *Plugin) recordTicketCreatedNotification(ticketID int, subject string) {
	if svc := p.currentNotification(); svc != nil {
		svc.NotifyTicketCreated(ticketID, subject)
	} else {
		// Fallback to legacy behavior if notification service not initialized
		p.recordNotification(Notification{
			ID:        fmt.Sprintf("n-%d-%d", time.Now().UnixNano(), ticketID),
			Type:      "ticket_created",
			TicketID:  ticketID,
			Title:     subject,
			Status:    "New",
			CreatedAt: time.Now().Unix(),
		})
	}
}

// loadNotifications returns stored notifications (newest first).
func (p *Plugin) loadNotifications() []Notification {
	raw, _ := p.API.KVGet(notificationStoreKey)
	if len(raw) == 0 {
		return nil
	}
	var list []Notification
	if err := json.Unmarshal(raw, &list); err != nil {
		p.API.LogWarn("failed to decode stored notifications", "err", err.Error())
		return nil
	}
	return list
}

// notificationReadAt returns the last-read timestamp for a user.
func (p *Plugin) notificationReadAt(userID string) int64 {
	raw, _ := p.API.KVGet(notificationReadKeyPrefix + userID)
	if len(raw) == 0 {
		return 0
	}
	var t int64
	_ = json.Unmarshal(raw, &t)
	return t
}

// setNotificationReadAt stores the last-read timestamp for a user.
func (p *Plugin) setNotificationReadAt(userID string, t int64) {
	if data, err := json.Marshal(t); err == nil {
		_ = p.API.KVSet(notificationReadKeyPrefix+userID, data)
	}
}

// notificationDismissed returns the set of dismissed notification IDs for a user.
func (p *Plugin) notificationDismissed(userID string) map[string]bool {
	raw, _ := p.API.KVGet(notificationDismissedPrefix + userID)
	if len(raw) == 0 {
		return map[string]bool{}
	}
	var ids []string
	if err := json.Unmarshal(raw, &ids); err != nil {
		return map[string]bool{}
	}
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// dismissNotification marks a notification as dismissed for a user.
func (p *Plugin) dismissNotification(userID, id string) {
	set := p.notificationDismissed(userID)
	if set[id] {
		return
	}
	set[id] = true
	ids := make([]string, 0, len(set))
	for nid := range set {
		ids = append(ids, nid)
	}
	if data, err := json.Marshal(ids); err == nil {
		_ = p.API.KVSet(notificationDismissedPrefix+userID, data)
	}
}

// ---------- /api/v1/notifications ----------

// apiNotifications lists persisted notifications for the authenticated user,
// applying per-user dismissal and reporting an unread count.
func (p *Plugin) apiNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	uid := currentUserID(r)
	readAt := p.notificationReadAt(uid)
	dismissed := p.notificationDismissed(uid)

	all := p.loadNotifications()
	out := make([]Notification, 0, len(all))
	unread := 0
	for _, n := range all {
		if dismissed[n.ID] {
			continue
		}
		out = append(out, n)
		if n.CreatedAt > readAt {
			unread++
		}
	}

	writeOK(w, map[string]interface{}{
		"notifications": out,
		"unread":        unread,
	})
}

// apiNotificationRead marks a single notification as read by advancing the
// user's read timestamp to the notification's creation time (if newer).
func (p *Plugin) apiNotificationRead(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	uid := currentUserID(r)
	for _, n := range p.loadNotifications() {
		if n.ID == id {
			current := p.notificationReadAt(uid)
			if n.CreatedAt > current {
				p.setNotificationReadAt(uid, n.CreatedAt)
			}
			writeOK(w, map[string]string{"status": "ok"})
			return
		}
	}
	writeError(w, http.StatusNotFound, "notification not found")
}

// apiNotificationDismiss removes a notification from the user's list.
func (p *Plugin) apiNotificationDismiss(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	uid := currentUserID(r)
	p.dismissNotification(uid, id)
	writeOK(w, map[string]string{"status": "ok"})
}
