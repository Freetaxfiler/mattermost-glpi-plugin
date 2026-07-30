package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/glpi"
	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/handlers"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// ServeHTTP handles all incoming HTTP requests to the plugin.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	// Add security headers to all plugin HTTP responses
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none';")

	// REST API for the webapp plugin
	if strings.HasPrefix(r.URL.Path, "/api/v1") {
		p.handleAPI(w, r)
		return
	}

	switch r.URL.Path {
	case "/dialog/submit":
		// Dialog submissions are proxied through the Mattermost server, which
		// sets the authenticated user's ID on this header.
		if r.Header.Get("Mattermost-User-Id") == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		// attach a request id to the HTTP request context for correlation
		if r.Context().Value("request_id") == nil {
			reqID := fmt.Sprintf("dlg-%d-%d", time.Now().Unix(), time.Now().UnixNano()%100000)
			r = r.WithContext(context.WithValue(r.Context(), "request_id", reqID))
		}
		handler := &handlers.DialogHandler{Submit: p.handleDialogSubmission}
		handler.HandleSubmit(w, r)

	case "/webhook":
		config := p.currentConfiguration()
		secret := ""
		if config != nil {
			secret = config.WebhookSecret
		}
		window := 24 * 60 * 60 // default 24 hours
		if config != nil && config.WebhookReplayWindowSeconds > 0 {
			window = config.WebhookReplayWindowSeconds
		}
		handler := &handlers.WebhookHandler{
			Secret: secret,
			Notify: p.notifyWebhookEvent,
			Dedupe: func(fingerprint string) (bool, error) {
				key := "glpi_webhook_" + fingerprint
				if data, appErr := p.API.KVGet(key); appErr == nil && len(data) > 0 {
					return true, nil
				}
				if appErr := p.API.KVSetWithExpiry(key, []byte("1"), int64(window)); appErr != nil {
					return false, fmt.Errorf("kv set failed: %s", appErr.Error())
				}
				return false, nil
			},
		}
		handler.HandleWebhook(w, r)

	default:
		http.NotFound(w, r)
	}
}

// OpenCreateTicketDialog opens the interactive ticket-creation dialog.
func (p *Plugin) OpenCreateTicketDialog(args *model.CommandArgs) error {
	siteURL := strings.TrimRight(args.SiteURL, "/")
	if siteURL == "" {
		if config := p.API.GetConfig(); config != nil && config.ServiceSettings.SiteURL != nil {
			siteURL = strings.TrimRight(*config.ServiceSettings.SiteURL, "/")
		}
	}
	if siteURL == "" {
		return fmt.Errorf("the Mattermost Site URL is not configured; set it in System Console > Web Server")
	}

	priorityOptions := []*model.PostActionOptions{
		{Text: "Very low", Value: "1"},
		{Text: "Low", Value: "2"},
		{Text: "Medium", Value: "3"},
		{Text: "High", Value: "4"},
		{Text: "Very high", Value: "5"},
	}
	urgencyOptions := []*model.PostActionOptions{
		{Text: "Very low", Value: "1"},
		{Text: "Low", Value: "2"},
		{Text: "Medium", Value: "3"},
		{Text: "High", Value: "4"},
		{Text: "Very high", Value: "5"},
	}

	dialog := model.Dialog{
		CallbackId:  "glpi_create_ticket",
		Title:       "Create GLPI Ticket",
		SubmitLabel: "Create Ticket",
		State:       args.ChannelId,
		Elements: []model.DialogElement{
			{
				DisplayName: "Subject",
				Name:        "subject",
				Type:        "text",
				Placeholder: "Enter ticket subject",
			},
			{
				DisplayName: "Description",
				Name:        "description",
				Type:        "textarea",
				Placeholder: "Describe the issue",
			},
			{
				DisplayName: "Priority",
				Name:        "priority",
				Type:        "select",
				Default:     "3",
				Options:     priorityOptions,
			},
			{
				DisplayName: "Urgency",
				Name:        "urgency",
				Type:        "select",
				Default:     "3",
				Optional:    true,
				Options:     urgencyOptions,
			},
			{
				DisplayName: "Category ID",
				Name:        "category",
				Type:        "text",
				Optional:    true,
				Placeholder: "GLPI category ID (leave empty for the default)",
				HelpText:    "Numeric GLPI ITIL category ID",
			},
		},
	}

	request := model.OpenDialogRequest{
		TriggerId: args.TriggerId,
		URL:       siteURL + "/plugins/" + PluginID + "/dialog/submit",
		Dialog:    dialog,
	}

	if err := p.API.OpenInteractiveDialog(request); err != nil {
		p.API.LogError("OpenInteractiveDialog failed", "error", err.Error())
		return err
	}

	return nil
}

// handleDialogSubmission validates and processes the create-ticket dialog.
func (p *Plugin) handleDialogSubmission(ctx context.Context, req *model.SubmitDialogRequest) (map[string]string, error) {
	subject := strings.TrimSpace(handlers.StringField(req.Submission, "subject"))
	description := strings.TrimSpace(handlers.StringField(req.Submission, "description"))
	priorityRaw := handlers.StringField(req.Submission, "priority")
	urgencyRaw := handlers.StringField(req.Submission, "urgency")
	categoryRaw := strings.TrimSpace(handlers.StringField(req.Submission, "category"))

	fieldErrors := map[string]string{}
	if subject == "" {
		fieldErrors["subject"] = "Subject is required."
	}
	if description == "" {
		fieldErrors["description"] = "Description is required."
	}

	priority, _ := strconv.Atoi(priorityRaw)
	urgency, _ := strconv.Atoi(urgencyRaw)

	config := p.currentConfiguration()
	categoryID := 0
	if categoryRaw != "" {
		parsed, err := strconv.Atoi(categoryRaw)
		if err != nil || parsed <= 0 {
			fieldErrors["category"] = "Category must be a numeric GLPI category ID."
		} else {
			categoryID = parsed
		}
	} else if config != nil && strings.TrimSpace(config.DefaultCategory) != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(config.DefaultCategory)); err == nil && parsed > 0 {
			categoryID = parsed
		}
	}

	if len(fieldErrors) > 0 {
		return fieldErrors, nil
	}

	client := p.GetGLPIClient()
	if client == nil {
		return nil, fmt.Errorf("GLPI client is not initialized; check the plugin configuration")
	}

	entityID := 0
	if config != nil && strings.TrimSpace(config.DefaultEntity) != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(config.DefaultEntity)); err == nil && parsed > 0 {
			entityID = parsed
		}
	}

	// Best-effort: file the ticket under the submitting user's GLPI account.
	requesterID := 0
	if glpiUserID, err := p.GetGLPIUserID(req.UserId); err == nil {
		requesterID = glpiUserID
	} else {
		p.API.LogDebug("could not resolve GLPI requester for dialog submission", "err", err.Error())
	}

	// preserve any request_id value from the incoming HTTP context when creating a ticket
	reqID, _ := ctx.Value("request_id").(string)
	if reqID == "" {
		reqID = fmt.Sprintf("dlg-%d-%d", time.Now().Unix(), time.Now().UnixNano()%100000)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	createReq := glpi.CreateTicketRequest{
		Name:           subject,
		Content:        description,
		Priority:       priority,
		Urgency:        urgency,
		ITILCategoryID: categoryID,
		EntityID:       entityID,
		RequesterID:    requesterID,
	}

	result, err := client.CreateTicket(reqCtx, createReq)
	if err != nil {
		// attempt durable enqueue for retry
		payload := CreateTicketPayload{
			Request:             createReq,
			RequesterMattermost: req.UserId,
			ChannelID:           req.ChannelId,
			RequestID:           reqID,
		}
		if p.retryQueue != nil {
			if err2 := p.EnqueueCreateTicket(reqCtx, payload); err2 == nil {
				// inform user that their operation has been queued for retry
				msg := "⚠️ Ticket creation failed due to a temporary error and has been queued for retry. You will be notified if creation succeeds."
				post := &model.Post{UserId: p.botUserID, ChannelId: req.ChannelId, Message: msg}
				p.API.SendEphemeralPost(req.UserId, post)
				p.API.LogWarn("CreateTicket failed, enqueued for retry", "err", err.Error(), "job_info", payload)
				return nil, nil
			}
			// enqueue failed; fallthrough to return original error
		}
		return nil, err
	}

	ticketURL := ""
	if config != nil {
		ticketURL = strings.TrimRight(config.GLPIURL, "/") + "/front/ticket.form.php?id=" + strconv.Itoa(result.ID)
	}

	message := fmt.Sprintf("✅ Ticket #%d created successfully.\n%s", result.ID, ticketURL)
	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: req.ChannelId,
		Message:   message,
	}
	p.API.SendEphemeralPost(req.UserId, post)

	p.API.LogInfo("Ticket created from dialog", "id", result.ID, "user_id", req.UserId, "request_id", reqID)
	return nil, nil
}

// notifyWebhookEvent posts a GLPI notification into Mattermost.
func (p *Plugin) notifyWebhookEvent(event handlers.WebhookEvent) {
	message := formatWebhookMessage(event)

	config := p.currentConfiguration()

	posted := false
	if config != nil && strings.TrimSpace(config.NotificationChannelID) != "" && p.botUserID != "" {
		post := &model.Post{
			UserId:    p.botUserID,
			ChannelId: strings.TrimSpace(config.NotificationChannelID),
			Message:   message,
		}
		if _, appErr := p.API.CreatePost(post); appErr != nil {
			p.API.LogWarn("failed to post GLPI notification to channel", "err", appErr.Error())
		} else {
			posted = true
		}
	}

	if event.RequesterEmail != "" && p.botUserID != "" {
		if user, appErr := p.API.GetUserByEmail(event.RequesterEmail); appErr == nil && user != nil {
			if channel, appErr := p.API.GetDirectChannel(p.botUserID, user.Id); appErr == nil && channel != nil {
				post := &model.Post{
					UserId:    p.botUserID,
					ChannelId: channel.Id,
					Message:   message,
				}
				if _, appErr := p.API.CreatePost(post); appErr != nil {
					p.API.LogWarn("failed to DM GLPI notification", "err", appErr.Error())
				} else {
					posted = true
				}
			}
		}
	}

	if !posted {
		p.API.LogInfo("GLPI webhook received but no notification target was available", "event", event.Event, "ticket_id", event.TicketID)
	}
}

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

// kvStore defines the subset of the Mattermost KV store operations needed for
// atomic webhook fingerprint deduplication.
type kvStore interface {
	KVCompareAndSet(key string, oldValue, newValue []byte) (bool, *model.AppError)
	KVSetWithExpiry(key string, value []byte, expireInSeconds int64) *model.AppError
	KVDelete(key string) *model.AppError
}

// claimWebhookFingerprint atomically claims a webhook fingerprint in the KV store
// to detect duplicate webhook deliveries across a Mattermost cluster. It returns
// (true, nil) when the fingerprint was already claimed (replay), or (false, nil)
// when the claim was new. The TTL controls how long duplicates are recognised.
func claimWebhookFingerprint(store kvStore, fingerprint string, ttl int64) (bool, error) {
	key := "glpi_webhook_" + fingerprint
	// KVCompareAndSet with nil oldValue atomically creates the key only if it
	// does not already exist.
	claimed, appErr := store.KVCompareAndSet(key, nil, []byte("1"))
	if appErr != nil {
		return false, fmt.Errorf("kv compare-and-set failed: %s", appErr.Error())
	}
	if !claimed {
		// The key already existed — this is a replay.
		return true, nil
	}
	// First claim — set the TTL so the key auto-expires.
	if appErr := store.KVSetWithExpiry(key, []byte("1"), ttl); appErr != nil {
		// Best-effort: log the expiry failure and clean up the key to avoid
		// leaving a permanent entry. The original claim succeeded, so we still
		// process the event.
		_ = store.KVDelete(key)
		return false, fmt.Errorf("kv set expiry failed: %s", appErr.Error())
	}
	return false, nil
}