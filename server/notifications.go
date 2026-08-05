package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/handlers"
	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/identity"
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
	ID            string `json:"id"`
	Type          string `json:"type"`
	TicketID      int    `json:"ticket_id,omitempty"`
	Title         string `json:"title"`
	Status        string `json:"status,omitempty"`
	URL           string `json:"url,omitempty"`
	CreatorUserID string `json:"creator_user_id,omitempty"`
	CreatedAt     int64  `json:"created_at"`
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
func (p *Plugin) recordTicketCreatedNotification(ticketID int, subject, creatorUserID string) {
	if svc := p.currentNotification(); svc != nil {
		svc.NotifyTicketCreated(ticketID, subject, creatorUserID)
	} else {
		p.recordNotification(Notification{
			ID:            fmt.Sprintf("n-%d-%d", time.Now().UnixNano(), ticketID),
			Type:          "ticket_created",
			TicketID:      ticketID,
			Title:         subject,
			Status:        "New",
			CreatorUserID: creatorUserID,
			CreatedAt:     time.Now().Unix(),
		})
	}
}

// loadNotifications returns stored notifications (newest first). When
// ownedTicketIDs is non-nil, only notifications matching one of the owned
// tickets or having a CreatorUserID equal to callerID are returned. A nil
// ownedTicketIDs slice (or callerID of "") returns all notifications
// (admin/special case).
func (p *Plugin) loadNotifications(ownedTicketIDs map[int]bool, callerID string) []Notification {
	raw, _ := p.API.KVGet(notificationStoreKey)
	if len(raw) == 0 {
		return nil
	}
	var list []Notification
	if err := json.Unmarshal(raw, &list); err != nil {
		p.API.LogWarn("failed to decode stored notifications", "err", err.Error())
		return nil
	}
	// No filter → return all (used by admin/special contexts).
	if ownedTicketIDs == nil {
		return list
	}
	filtered := make([]Notification, 0, len(list))
	for _, n := range list {
		if n.TicketID > 0 && ownedTicketIDs[n.TicketID] {
			filtered = append(filtered, n)
			continue
		}
		if callerID != "" && n.CreatorUserID == callerID {
			filtered = append(filtered, n)
			continue
		}
	}
	return filtered
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

// ownedTicketIDsForUser builds a set of ticket IDs that the user owns. The
// set is used to scope notifications, dashboards, and other collection queries
// to the user's own data.
func (p *Plugin) ownedTicketIDsForUser(uid string) map[int]bool {
	if uid == "" {
		return nil
	}
	owned := map[int]bool{}
	// Identity service ownership store (Mode A fallback).
	if svc := p.currentIdentity(); svc != nil {
		for _, id := range svc.ListOwnedTicketIDs(uid) {
			owned[id] = true
		}
	}
	// If the user has a mapped GLPI account (Mode B), they may have tickets
	// owned via GLPI search — we cannot enumerate those from the KV store,
	// but any webhook/API notification that records CreatorUserID will be
	// matched directly in loadNotifications.
	return owned
}

// apiNotifications lists persisted notifications for the authenticated user,
// applying per-user ownership filtering, per-user dismissal, and unread count.
func (p *Plugin) apiNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	uid := currentUserID(r)
	readAt := p.notificationReadAt(uid)
	dismissed := p.notificationDismissed(uid)

	// Role-based ownership scope: admin sees all; others see own only.
	var owned map[int]bool
	if own := p.currentOwnership(); own != nil {
		role, _ := own.ResolveRole(uid)
		if role != identity.RoleAdministrator {
			owned = p.ownedTicketIDsForUser(uid)
		}
		// owned=nil for admins → loadNotifications returns all.
	} else {
		owned = p.ownedTicketIDsForUser(uid)
	}
	all := p.loadNotifications(owned, uid)
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
	owned := p.ownedTicketIDsForUser(uid)
	for _, n := range p.loadNotifications(owned, uid) {
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
