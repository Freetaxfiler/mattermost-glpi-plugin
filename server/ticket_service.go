package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/glpi"
	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/handlers"
	"github.com/mattermost/mattermost/server/public/model"
)

// TicketInput represents all data needed to create a ticket, regardless of
// the entry point (API, dialog, retry queue, or slash command).
type TicketInput struct {
	Subject       string
	Content       string
	Type          int
	Priority      int
	Urgency       int
	CategoryID    int
	EntityID      int
	RequesterID   int // 0 = integration account (Mode A)
	IsPrivate     bool
	CreatorUserID string // Mattermost user ID who initiated creation (for ownership)
	ChannelID     string // for follow-up notifications
	RequestID     string // for idempotency/correlation
	TeamID        string // optional Mattermost team for attribution
	AssetID       int    // optional linked GLPI asset (Create Ticket from Asset)
	AssetType     string // optional GLPI item type for the linked asset
}

// CreateTicketResult contains the result of ticket creation.
type CreateTicketResult struct {
	Ticket *glpi.CreateTicketResponse
	// Whether the ticket was created by the integration account (Mode A)
	IntegrationMode bool
}

// TicketService handles all ticket creation logic centrally.
type TicketService struct {
	p *Plugin
}

// NewTicketService creates a new ticket service bound to the plugin.
func NewTicketService(p *Plugin) *TicketService {
	return &TicketService{p: p}
}

// CreateTicket creates a ticket using the centralized logic.
// It handles requester resolution, metadata injection, GLPI API call,
// ownership recording, notifications, and websocket broadcast.
func (s *TicketService) CreateTicket(ctx context.Context, input TicketInput) (*CreateTicketResult, error) {
	// Resolve requester through identity service
	requester, mmUser := s.p.resolveRequesterFor(input.CreatorUserID, input.TeamID, input.ChannelID)
	requesterID := input.RequesterID
	if requesterID == 0 {
		requesterID = requester.GLPIUserID
	}

	// Preserve Mattermost identity as metadata when using integration account
	content := strings.TrimSpace(input.Content)
	if requesterID == 0 && mmUser != nil {
		content += mmUser.MetadataHTML()
	}

	// Resolve category and entity defaults
	categoryID := input.CategoryID
	if categoryID <= 0 {
		if cfg := s.p.currentConfiguration(); cfg != nil && strings.TrimSpace(cfg.DefaultCategory) != "" {
			if parsed, err := strconv.Atoi(strings.TrimSpace(cfg.DefaultCategory)); err == nil && parsed > 0 {
				categoryID = parsed
			}
		}
	}
	entityID := input.EntityID
	if entityID <= 0 {
		if cfg := s.p.currentConfiguration(); cfg != nil && strings.TrimSpace(cfg.DefaultEntity) != "" {
			if parsed, err := strconv.Atoi(strings.TrimSpace(cfg.DefaultEntity)); err == nil && parsed > 0 {
				entityID = parsed
			}
		}
	}

	// Build GLPI create request
	createReq := glpi.CreateTicketRequest{
		Name:           strings.TrimSpace(input.Subject),
		Content:        content,
		Priority:       input.Priority,
		Urgency:        input.Urgency,
		Type:           input.Type,
		ITILCategoryID: categoryID,
		EntityID:       entityID,
		RequesterID:    requesterID,
		AssetID:        input.AssetID,
		AssetType:      input.AssetType,
	}

	// Call GLPI
	client := s.p.GetGLPIClient()
	if client == nil {
		return nil, fmt.Errorf("GLPI client not initialized")
	}

	result, err := client.CreateTicket(ctx, createReq)
	if err != nil {
		return nil, err
	}

	// Record ownership for "My Tickets" fallback (Mode A)
	if input.CreatorUserID != "" {
		s.p.recordOwnership(input.CreatorUserID, result.ID)
	}

	// Record notification for webapp notification center + websocket push
	s.p.recordTicketCreatedNotification(result.ID, strings.TrimSpace(input.Subject))

	// Build result
	integrationMode := requesterID == 0
	return &CreateTicketResult{
		Ticket:          result,
		IntegrationMode: integrationMode,
	}, nil
}

// InputBuilder helps construct TicketInput from various entry points.
type InputBuilder struct {
	Subject       string
	Content       string
	Type          int
	Priority      int
	Urgency       int
	CategoryID    int
	EntityID      int
	RequesterID   int
	IsPrivate     bool
	CreatorUserID string
	ChannelID     string
	RequestID     string
	TeamID        string
	AssetID       int
	AssetType     string
}

func (b *InputBuilder) Build() TicketInput {
	return TicketInput{
		Subject:       b.Subject,
		Content:       b.Content,
		Type:          b.Type,
		Priority:      b.Priority,
		Urgency:       b.Urgency,
		CategoryID:    b.CategoryID,
		EntityID:      b.EntityID,
		RequesterID:   b.RequesterID,
		IsPrivate:     b.IsPrivate,
		CreatorUserID: b.CreatorUserID,
		ChannelID:     b.ChannelID,
		RequestID:     b.RequestID,
		TeamID:        b.TeamID,
		AssetID:       b.AssetID,
		AssetType:     b.AssetType,
	}
}

// DialogInputToTicketInput converts a dialog submission to TicketInput.
func DialogInputToTicketInput(req *model.SubmitDialogRequest, defaultCategory, defaultEntity int) TicketInput {
	b := &InputBuilder{
		Subject:    strings.TrimSpace(handlers.StringField(req.Submission, "subject")),
		Content:    strings.TrimSpace(handlers.StringField(req.Submission, "description")),
		Priority:   parseIntDefault(handlers.StringField(req.Submission, "priority"), 3),
		Urgency:    parseIntDefault(handlers.StringField(req.Submission, "urgency"), 3),
		CategoryID: parseIntDefault(strings.TrimSpace(handlers.StringField(req.Submission, "category")), 0),
		Type:       parseIntDefault(handlers.StringField(req.Submission, "type"), 1),
	}
	if b.CategoryID == 0 {
		b.CategoryID = defaultCategory
	}
	// Entity ID from config if not provided in dialog
	if b.EntityID == 0 {
		if cfg := currentPlugin.currentConfiguration(); cfg != nil && strings.TrimSpace(cfg.DefaultEntity) != "" {
			if parsed, err := strconv.Atoi(strings.TrimSpace(cfg.DefaultEntity)); err == nil && parsed > 0 {
				b.EntityID = parsed
			}
		}
	}
	return b.Build()
}

var currentPlugin *Plugin

func SetCurrentPlugin(p *Plugin) {
	currentPlugin = p
}

func parseIntDefault(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, _ := strconv.Atoi(s)
	if v <= 0 {
		return defaultVal
	}
	return v
}
