package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// Per-user knowledge base personalization, stored in plugin KV:
//
//	glpi_kb_fav_<userID>   → []kbItem  (favorites, newest first)
//	glpi_kb_recent_<userID> → []kbItem  (recently viewed, newest first)
const (
	kbFavoritesKeyPrefix = "glpi_kb_fav_"
	kbRecentKeyPrefix    = "glpi_kb_recent_"
	kbPersonalLimit      = 20
)

type kbItem struct {
	ID      int    `json:"id"`
	Subject string `json:"subject"`
	At      int64  `json:"at"`
}

func (p *Plugin) loadKBItems(key string) []kbItem {
	raw, appErr := p.API.KVGet(key)
	if appErr != nil || len(raw) == 0 {
		return nil
	}
	var items []kbItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	return items
}

func (p *Plugin) saveKBItems(key string, items []kbItem) {
	if len(items) > kbPersonalLimit {
		items = items[:kbPersonalLimit]
	}
	if data, err := json.Marshal(items); err == nil {
		_ = p.API.KVSet(key, data)
	}
}

// recordKBView records a knowledge article view (read tracking + recently
// viewed). Best-effort.
func (p *Plugin) recordKBView(userID string, id int, subject string) {
	if userID == "" || id <= 0 {
		return
	}
	key := kbRecentKeyPrefix + userID
	items := p.loadKBItems(key)
	filtered := items[:0]
	for _, it := range items {
		if it.ID != id {
			filtered = append(filtered, it)
		}
	}
	filtered = append([]kbItem{{ID: id, Subject: subject, At: time.Now().Unix()}}, filtered...)
	p.saveKBItems(key, filtered)
}

func (p *Plugin) toggleFavorite(userID string, id int, subject string, add bool) {
	key := kbFavoritesKeyPrefix + userID
	items := p.loadKBItems(key)
	filtered := items[:0]
	found := false
	for _, it := range items {
		if it.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, it)
	}
	if add && !found {
		filtered = append([]kbItem{{ID: id, Subject: subject, At: time.Now().Unix()}}, filtered...)
	}
	p.saveKBItems(key, filtered)
}

func (p *Plugin) isFavorite(userID string, id int) bool {
	for _, it := range p.loadKBItems(kbFavoritesKeyPrefix + userID) {
		if it.ID == id {
			return true
		}
	}
	return false
}

// apiKnowledgeFavorites lists the current user's favorite articles.
func (p *Plugin) apiKnowledgeFavorites(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	uid := currentUserID(r)
	if uid == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	items := p.loadKBItems(kbFavoritesKeyPrefix + uid)
	writeOK(w, map[string]interface{}{"items": items, "count": len(items)})
}

// apiKnowledgeRecent lists the current user's recently viewed articles.
func (p *Plugin) apiKnowledgeRecent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	uid := currentUserID(r)
	if uid == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	items := p.loadKBItems(kbRecentKeyPrefix + uid)
	writeOK(w, map[string]interface{}{"items": items, "count": len(items)})
}

// apiKnowledgeSetFavorite toggles a favorite for an article.
// POST adds; DELETE removes.
func (p *Plugin) apiKnowledgeSetFavorite(w http.ResponseWriter, r *http.Request, idStr string) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	uid := currentUserID(r)
	if uid == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid article id")
		return
	}
	var subject string
	var payload struct {
		Subject string `json:"subject"`
	}
	if r.Method == http.MethodPost && r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&payload)
		subject = payload.Subject
	}
	p.toggleFavorite(uid, id, subject, r.Method == http.MethodPost)
	writeOK(w, map[string]interface{}{"id": id, "favorite": p.isFavorite(uid, id)})
}
