package handlers

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// WebhookEvent is a normalized GLPI notification event.
type WebhookEvent struct {
	Event          string
	TicketID       int
	Title          string
	Status         string
	URL            string
	RequesterEmail string
}

// WebhookNotifier delivers a webhook event into Mattermost.
type WebhookNotifier func(event WebhookEvent)

// WebhookHandler receives notification callbacks from GLPI webhooks.
type WebhookHandler struct {
	// Secret must match the value sent by GLPI in the X-GLPI-Secret header.
	Secret string
	Notify WebhookNotifier
	// Dedupe is an optional function provided by the host to atomically claim a
	// payload fingerprint. It returns (true, nil) if the payload was already
	// claimed by another request.
	Dedupe func(fingerprint string) (bool, error)
}

// HandleWebhook validates and parses an incoming GLPI webhook payload.
// The payload is parsed leniently because GLPI webhook payloads are
// admin-configurable templates.
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if strings.TrimSpace(h.Secret) == "" {
		http.Error(w, "webhook secret is not configured", http.StatusForbidden)
		return
	}

	provided := r.Header.Get("X-GLPI-Secret")
	if provided == "" {
		http.Error(w, "missing X-GLPI-Secret header", http.StatusUnauthorized)
		return
	}
	if subtle.ConstantTimeCompare([]byte(provided), []byte(h.Secret)) != 1 {
		http.Error(w, "invalid webhook secret", http.StatusForbidden)
		return
	}

	if h.Notify == nil {
		http.Error(w, "webhook notifier not configured", http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
	if err != nil {
		http.Error(w, "failed to read webhook body", http.StatusBadRequest)
		return
	}

	// Parse the webhook body FIRST so malformed payloads are rejected
	// before we compute a fingerprint or check deduplication.
	event, err := parseWebhookEvent(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// compute fingerprint for replay protection — only for valid payloads
	hash := sha256.Sum256(body)
	fingerprint := hex.EncodeToString(hash[:])
	if h.Dedupe != nil {
		seen, err := h.Dedupe(fingerprint)
		if err != nil {
			http.Error(w, "failed to verify webhook dedupe", http.StatusInternalServerError)
			return
		}
		if seen {
			// already processed — return OK for idempotency
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok","note":"replay"}`))
			return
		}
	}

	h.Notify(event)

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func parseWebhookEvent(body []byte) (WebhookEvent, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return WebhookEvent{}, fmt.Errorf("invalid webhook JSON: %w", err)
	}

	event := WebhookEvent{
		Event:          firstString(raw, "event", "event_type", "action"),
		Title:          firstString(raw, "title", "name", "subject"),
		Status:         firstString(raw, "status"),
		URL:            firstString(raw, "url", "link"),
		RequesterEmail: firstString(raw, "requester_email", "email"),
		TicketID:       firstInt(raw, "ticket_id", "items_id", "id"),
	}

	if event.Event == "" {
		event.Event = "notification"
	}
	return event, nil
}

func firstString(raw map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			switch t := value.(type) {
			case string:
				trimmed := strings.TrimSpace(t)
				if trimmed != "" {
					return trimmed
				}
			case float64:
				return strconv.FormatInt(int64(t), 10)
			}
		}
	}
	return ""
}

func firstInt(raw map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			switch t := value.(type) {
			case float64:
				return int(t)
			case string:
				parsed, err := strconv.Atoi(strings.TrimSpace(t))
				if err == nil {
					return parsed
				}
			}
		}
	}
	return 0
}
