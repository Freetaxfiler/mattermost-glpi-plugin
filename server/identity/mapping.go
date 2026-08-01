package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/glpi"
)

const ownedTicketsKeyPrefix = "glpi_owned_"

// RecordTicketOwnership associates a Mattermost user with a created ticket so
// "My Tickets" keeps working even when the user has no individual GLPI account
// (Mode A). Best-effort; failures are ignored.
func (s *Service) RecordTicketOwnership(userID string, ticketID int) {
	if s.kv == nil || userID == "" || ticketID <= 0 {
		return
	}
	key := ownedTicketsKeyPrefix + userID
	raw, _ := s.kv.KVGet(key)
	var ids []int
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &ids)
	}
	ids = append(ids, ticketID)
	if len(ids) > s.ownedTicketsLimit {
		ids = ids[len(ids)-s.ownedTicketsLimit:]
	}
	if data, err := json.Marshal(ids); err == nil {
		_ = s.kv.KVSet(key, data)
	}
}

// ListOwnedTicketIDs returns the Mattermost user's created ticket IDs,
// newest first.
func (s *Service) ListOwnedTicketIDs(userID string) []int {
	if s.kv == nil || userID == "" {
		return nil
	}
	raw, _ := s.kv.KVGet(ownedTicketsKeyPrefix + userID)
	if len(raw) == 0 {
		return nil
	}
	var ids []int
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil
	}
	// stored oldest-first; reverse for newest-first
	for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
		ids[i], ids[j] = ids[j], ids[i]
	}
	return ids
}

// ListOwnedTickets returns TicketSummary rows for the Mattermost user's owned
// tickets (Mode A fallback), newest first and paginated.
func (s *Service) ListOwnedTickets(ctx context.Context, userID string, page, perPage int, fetch FetchTicket) ([]glpi.TicketSummary, int, error) {
	ids := s.ListOwnedTicketIDs(userID)
	total := len(ids)
	if page < 1 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 15
	}
	start := (page - 1) * perPage
	if start > total {
		return []glpi.TicketSummary{}, total, nil
	}
	end := start + perPage
	if end > total {
		end = total
	}
	summaries := make([]glpi.TicketSummary, 0, end-start)
	for _, id := range ids[start:end] {
		t, err := fetch(ctx, id)
		if err != nil {
			continue
		}
		summaries = append(summaries, glpi.TicketSummary{
			ID:       t.ID,
			Name:     t.Name,
			Status:   t.Status,
			Priority: t.Priority,
			Opened:   t.Date,
		})
	}
	return summaries, total, nil
}

// MetadataHTML renders the Mattermost identity as a compact HTML block for
// inclusion at the end of a ticket description when the integration account is
// the requester (Mode A). GLPI 11 has no per-ticket custom fields, so the
// content field is the supported per-ticket free-text storage. Values are
// HTML-escaped to prevent injection into the GLPI UI.
func (m *MMUser) MetadataHTML() string {
	if m == nil {
		return ""
	}
	esc := html.EscapeString
	var b strings.Builder
	b.WriteString("<br/><hr/><strong>Mattermost Metadata</strong><br/>")
	b.WriteString(fmt.Sprintf("User ID: %s<br/>", esc(m.UserID)))
	b.WriteString(fmt.Sprintf("Username: %s<br/>", esc(m.Username)))
	if m.DisplayName != "" {
		b.WriteString(fmt.Sprintf("Display Name: %s<br/>", esc(m.DisplayName)))
	}
	b.WriteString(fmt.Sprintf("Email: %s<br/>", esc(m.Email)))
	if m.Team != "" {
		b.WriteString(fmt.Sprintf("Team: %s<br/>", esc(m.Team)))
	}
	if m.Channel != "" {
		b.WriteString(fmt.Sprintf("Channel: %s<br/>", esc(m.Channel)))
	}
	b.WriteString("Created From: Mattermost")
	return b.String()
}
