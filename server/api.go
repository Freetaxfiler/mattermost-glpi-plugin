package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/commands"
	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/glpi"
)

// apiResponse is the standard envelope for all API JSON responses.
type apiResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeOK(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, apiResponse{Status: "ok", Data: data})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiResponse{Status: "error", Error: msg})
}

func (p *Plugin) apiAuth(r *http.Request) (string, error) {
	uid := r.Header.Get("Mattermost-User-Id")
	if uid == "" {
		return "", fmt.Errorf("not authenticated")
	}
	return uid, nil
}

// handleAPI dispatches /api/v1/* requests.
func (p *Plugin) handleAPI(w http.ResponseWriter, r *http.Request) {
	// Every /api/v1 endpoint exposes or modifies user data, so require an
	// authenticated Mattermost session on all of them. Unauthenticated
	// callers must not reach ticket, asset, knowledge-base, or configuration
	// data. (Webhooks, slash commands, and dialog submissions are routed
	// outside this handler and are unaffected.)
	if _, err := p.apiAuth(r); err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Strip the /api/v1 prefix
	path := strings.TrimPrefix(r.URL.Path, "/api/v1")
	path = strings.TrimPrefix(path, "/")

	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "unknown API endpoint")
		return
	}

	resource := parts[0]

	switch resource {
	case "status":
		p.apiStatus(w, r)
	case "config":
		p.apiConfig(w, r)
	case "system":
		p.apiSystem(w, r)
	case "dashboard":
		p.apiDashboard(w, r)
	case "categories":
		p.apiCategories(w, r)
	case "tickets":
		p.apiTickets(w, r, parts)
	case "assets":
		if len(parts) >= 3 {
			p.apiAssetDetail(w, r, parts[1], parts[2])
		} else {
			p.apiAssets(w, r)
		}
	case "knowledge":
		switch {
		case len(parts) >= 2 && parts[1] == "categories":
			p.apiKnowledgeCategories(w, r)
		case len(parts) >= 2:
			p.apiKnowledgeArticle(w, r, parts[1])
		default:
			p.apiKnowledge(w, r)
		}
	case "notifications":
		switch {
		case len(parts) >= 3 && parts[2] == "read":
			p.apiNotificationRead(w, r, parts[1])
		case len(parts) >= 3 && parts[2] == "dismiss":
			p.apiNotificationDismiss(w, r, parts[1])
		default:
			p.apiNotifications(w, r)
		}
	case "user":
		p.apiUser(w, r)
	default:
		writeError(w, http.StatusNotFound, "unknown API endpoint: "+resource)
	}
}

// ---------- /api/v1/status ----------

func (p *Plugin) apiStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	config := p.currentConfiguration()

	resp := map[string]interface{}{
		"glpi_url":     "",
		"configured":   false,
		"glpi_version": "",
		"glpi_online":  false,
		"plugin_version": "0.2.0",
	}

	if config != nil && config.GLPIURL != "" {
		resp["glpi_url"] = config.GLPIURL
		resp["configured"] = true
	}

	client := p.GetGLPIClient()
	if client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if result, err := client.HealthCheck(ctx); err == nil {
			resp["glpi_version"] = result.Version
			resp["glpi_online"] = true
		}
	}

	writeOK(w, resp)
}

// ---------- /api/v1/config ----------

func (p *Plugin) apiConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	config := p.currentConfiguration()
	if config == nil {
		writeError(w, http.StatusServiceUnavailable, "configuration unavailable")
		return
	}

	writeOK(w, config.Redacted())
}

// ---------- /api/v1/tickets ----------

func (p *Plugin) apiTickets(w http.ResponseWriter, r *http.Request, parts []string) {
	// /api/v1/tickets or /api/v1/tickets/{id} or /api/v1/tickets/{id}/{action}
	if len(parts) >= 2 && parts[1] != "" {
		ticketID, err := strconv.Atoi(parts[1])
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid ticket id")
			return
		}

		if len(parts) >= 3 {
			// /api/v1/tickets/{id}/{action}
			switch parts[2] {
			case "followup":
				if r.Method == http.MethodPost {
					p.apiAddFollowup(w, r, ticketID)
				} else {
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				}
				return
			case "solution":
				if r.Method == http.MethodPost {
					p.apiAddSolution(w, r, ticketID)
				} else {
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				}
				return
			case "timeline":
				if r.Method == http.MethodGet {
					p.apiGetTimeline(w, r, ticketID)
				} else {
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				}
				return
			case "attach":
				if r.Method == http.MethodPost {
					p.apiAttachFile(w, r, ticketID)
				} else {
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				}
				return
			case "documents":
				// /api/v1/tickets/{id}/documents or /api/v1/tickets/{id}/documents/{docId}
				if r.Method == http.MethodGet {
					if len(parts) >= 4 {
						p.apiDownloadDocument(w, r, ticketID, parts[3])
					} else {
						p.apiListDocuments(w, r, ticketID)
					}
				} else {
					writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				}
				return
			default:
				writeError(w, http.StatusNotFound, "unknown action: "+parts[2])
				return
			}
		}

		// /api/v1/tickets/{id}
		switch r.Method {
		case http.MethodGet:
			p.apiGetTicket(w, r, ticketID)
		case http.MethodPut:
			p.apiUpdateTicket(w, r, ticketID)
		case http.MethodDelete:
			p.apiDeleteTicket(w, r, ticketID)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// /api/v1/tickets
	switch r.Method {
	case http.MethodGet:
		p.apiListTickets(w, r)
	case http.MethodPost:
		p.apiCreateTicket(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (p *Plugin) apiListTickets(w http.ResponseWriter, r *http.Request) {
	uid, err := p.apiAuth(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	q := r.URL.Query()
	filterType := q.Get("type")
	search := q.Get("search")
	limit := 15
	if l, err := strconv.Atoi(q.Get("per_page")); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	page := 1
	if pn, err := strconv.Atoi(q.Get("page")); err == nil && pn > 0 {
		page = pn
	}

	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	var glpiFilter glpi.TicketFilter
	glpiFilter.Limit = limit
	glpiFilter.Page = page

	if s := q.Get("status"); s != "" {
		if status, err := strconv.Atoi(s); err == nil && status > 0 {
			glpiFilter.Status = status
		}
	}
	if s := q.Get("sort"); s != "" {
		if sortField, err := strconv.Atoi(s); err == nil && sortField > 0 {
			glpiFilter.Sort = sortField
		}
	}
	if order := q.Get("order"); order == "ASC" || order == "DESC" {
		glpiFilter.Order = order
	}

	switch filterType {
	case "my":
		glpiUserID, err := p.GetGLPIUserID(uid)
		if err != nil {
			writeError(w, http.StatusNotFound, "could not resolve glpi user")
			return
		}
		if glpiUserID > 0 {
			glpiFilter.RequesterID = glpiUserID
		} else {
			// No individual GLPI account (integration mode): fall back to the
			// identity-service ownership mapping so "My Tickets" never fails.
			svc := p.currentIdentity()
			if svc == nil {
				writeError(w, http.StatusServiceUnavailable, "identity service not initialized")
				return
			}
			tickets, total, err := svc.ListOwnedTickets(ctx, uid, page, limit, client.GetTicket)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "ticket lookup failed: "+err.Error())
				return
			}
			writeOK(w, map[string]interface{}{"tickets": tickets, "total": total, "count": len(tickets)})
			return
		}
	case "assigned":
		glpiUserID, err := p.GetGLPIUserID(uid)
		if err != nil {
			writeError(w, http.StatusNotFound, "could not resolve glpi user")
			return
		}
		if glpiUserID > 0 {
			glpiFilter.AssigneeID = glpiUserID
		} else {
			// A user without an individual GLPI account cannot be an assignee.
			writeOK(w, map[string]interface{}{"tickets": []glpi.TicketSummary{}, "total": 0, "count": 0})
			return
		}
	default:
		if search != "" {
			glpiFilter.TitleQuery = search
		} else {
			glpiFilter.RequesterID = -1 // match all
		}
	}

	if search != "" {
		glpiFilter.TitleQuery = search
	}

	tickets, total, err := client.SearchTickets(ctx, glpiFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ticket search failed: "+err.Error())
		return
	}

	writeOK(w, map[string]interface{}{
		"tickets": tickets,
		"total":   total,
		"count":   len(tickets),
	})
}

func (p *Plugin) apiCreateTicket(w http.ResponseWriter, r *http.Request) {
	uid, err := p.apiAuth(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		Subject    string `json:"subject"`
		Content    string `json:"content"`
		Priority   int    `json:"priority"`
		Urgency    int    `json:"urgency"`
		CategoryID int    `json:"category_id"`
		Type       int    `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if strings.TrimSpace(req.Subject) == "" {
		writeError(w, http.StatusBadRequest, "subject is required")
		return
	}

	config := p.currentConfiguration()
	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	entityID := 0
	if config != nil && strings.TrimSpace(config.DefaultEntity) != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(config.DefaultEntity)); err == nil && parsed > 0 {
			entityID = parsed
		}
	}

	requester, mm := p.resolveRequesterFor(uid, "", "")
	requesterID := requester.GLPIUserID
	content := strings.TrimSpace(req.Content)
	if requester.GLPIUserID == 0 && mm != nil {
		// Integration account is the requester; preserve the Mattermost identity
		// as ticket metadata so ticket creation never loses the human.
		content += mm.MetadataHTML()
	}

	categoryID := req.CategoryID
	if categoryID <= 0 && config != nil && strings.TrimSpace(config.DefaultCategory) != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(config.DefaultCategory)); err == nil && parsed > 0 {
			categoryID = parsed
		}
	}

	createReq := glpi.CreateTicketRequest{
		Name:           strings.TrimSpace(req.Subject),
		Content:        content,
		Priority:       req.Priority,
		Urgency:        req.Urgency,
		Type:           req.Type,
		ITILCategoryID: categoryID,
		EntityID:       entityID,
		RequesterID:    requesterID,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	result, err := client.CreateTicket(ctx, createReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ticket creation failed: "+err.Error())
		return
	}

	p.recordOwnership(uid, result.ID)
	p.recordTicketCreatedNotification(result.ID, createReq.Name)
	writeOK(w, result)
}

func (p *Plugin) apiGetTicket(w http.ResponseWriter, r *http.Request, ticketID int) {
	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	ticket, err := client.GetTicket(ctx, ticketID)
	if err != nil {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}

	writeOK(w, ticket)
}

func (p *Plugin) apiUpdateTicket(w http.ResponseWriter, r *http.Request, ticketID int) {
	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	var input map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if err := client.UpdateTicket(ctx, ticketID, input); err != nil {
		writeError(w, http.StatusInternalServerError, "ticket update failed: "+err.Error())
		return
	}

	writeOK(w, map[string]string{"status": "updated"})
}

func (p *Plugin) apiDeleteTicket(w http.ResponseWriter, r *http.Request, ticketID int) {
	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if err := client.DeleteTicket(ctx, ticketID); err != nil {
		writeError(w, http.StatusInternalServerError, "ticket delete failed: "+err.Error())
		return
	}

	writeOK(w, map[string]string{"status": "deleted"})
}

func (p *Plugin) apiAddFollowup(w http.ResponseWriter, r *http.Request, ticketID int) {
	var req struct {
		Content   string `json:"content"`
		IsPrivate bool   `json:"is_private"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if strings.TrimSpace(req.Content) == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if err := client.AddFollowup(ctx, ticketID, req.Content, req.IsPrivate); err != nil {
		writeError(w, http.StatusInternalServerError, "add followup failed: "+err.Error())
		return
	}

	writeOK(w, map[string]string{"status": "followup_added"})
}

func (p *Plugin) apiAddSolution(w http.ResponseWriter, r *http.Request, ticketID int) {
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if strings.TrimSpace(req.Content) == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if err := client.AddSolution(ctx, ticketID, req.Content); err != nil {
		writeError(w, http.StatusInternalServerError, "add solution failed: "+err.Error())
		return
	}

	writeOK(w, map[string]string{"status": "solution_added"})
}

func (p *Plugin) apiGetTimeline(w http.ResponseWriter, r *http.Request, ticketID int) {
	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	q := r.URL.Query()
	page := 1
	perPage := 20
	if pn, err := strconv.Atoi(q.Get("page")); err == nil && pn > 0 {
		page = pn
	}
	if pp, err := strconv.Atoi(q.Get("per_page")); err == nil && pp > 0 && pp <= 100 {
		perPage = pp
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	timeline, err := client.GetTicketTimeline(ctx, ticketID, glpi.TimelinePageRequest{Page: page, PerPage: perPage})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "timeline fetch failed: "+err.Error())
		return
	}

	// Private timeline events are only exposed to system administrators to
	// prevent leaking confidential follow-ups through the shared GLPI API
	// account. handleAPI has already authenticated the request, so apiAuth
	// here only recovers the user ID for the role check.
	if uid, err := p.apiAuth(r); err == nil && !p.IsSystemAdmin(uid) {
		timeline.Events = commands.VisibleTimelineEvents(timeline.Events, false)
		timeline.Total = len(timeline.Events)
		timeline.HasMore = false
	}

	writeOK(w, timeline)
}

func (p *Plugin) apiAttachFile(w http.ResponseWriter, r *http.Request, ticketID int) {
	_, err := p.apiAuth(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Parse multipart upload
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file field is required")
		return
	}
	defer file.Close()

	data := make([]byte, header.Size)
	if _, err := file.Read(data); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read uploaded file")
		return
	}

	maxUpload := 10 * 1024 * 1024
	if cfg := p.currentConfiguration(); cfg != nil && cfg.MaxUploadSizeBytes > 0 {
		maxUpload = cfg.MaxUploadSizeBytes
	}
	if len(data) > maxUpload {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("file exceeds maximum allowed size (%d bytes)", maxUpload))
		return
	}

	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	docID, err := client.UploadDocument(ctx, header.Filename, data, ticketID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upload failed: "+err.Error())
		return
	}

	writeOK(w, map[string]interface{}{
		"document_id": docID,
		"filename":    header.Filename,
	})
}

// ---------- /api/v1/assets ----------

func (p *Plugin) apiAssets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	uid, err := p.apiAuth(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	q := r.URL.Query()
	itemType := q.Get("type")
	if itemType == "" {
		itemType = "Computer"
	}
	search := q.Get("search")
	limit := 15
	if l, err := strconv.Atoi(q.Get("per_page")); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	page := 1
	if p, err := strconv.Atoi(q.Get("page")); err == nil && p > 0 {
		page = p
	}

	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	filter := glpi.AssetFilter{
		ItemType:  itemType,
		NameQuery: search,
		Limit:     limit,
		Page:      page,
	}

	if search == "" && glpi.SupportsUserFilter(itemType) {
		if glpiUserID, err := p.GetGLPIUserID(uid); err == nil {
			filter.GLPIUserID = glpiUserID
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	assets, total, err := client.SearchAssets(ctx, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "asset search failed: "+err.Error())
		return
	}

	writeOK(w, map[string]interface{}{
		"assets": assets,
		"total":  total,
		"count":  len(assets),
	})
}

// ---------- /api/v1/knowledge ----------

func (p *Plugin) apiKnowledge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	q := r.URL.Query()
	query := q.Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "search query (q) is required")
		return
	}

	categoryID := 0
	if c := q.Get("category"); c != "" {
		if id, err := strconv.Atoi(c); err == nil && id > 0 {
			categoryID = id
		}
	}

	limit := 15
	if l, err := strconv.Atoi(q.Get("per_page")); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	page := 1
	if pn, err := strconv.Atoi(q.Get("page")); err == nil && pn > 0 {
		page = pn
	}

	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	articles, total, err := client.SearchKnowledge(ctx, query, categoryID, limit, page)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "knowledge search failed: "+err.Error())
		return
	}

	writeOK(w, map[string]interface{}{
		"articles": articles,
		"total":    total,
		"count":    len(articles),
	})
}

// apiKnowledgeCategories lists knowledge base categories for the KB filter.
func (p *Plugin) apiKnowledgeCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	limit := 100
	if l, err := strconv.Atoi(r.URL.Query().Get("per_page")); err == nil && l > 0 && l <= 200 {
		limit = l
	}

	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	categories, total, err := client.SearchKnowledgeBaseCategories(ctx, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "knowledge category search failed: "+err.Error())
		return
	}

	writeOK(w, map[string]interface{}{
		"categories": categories,
		"total":      total,
		"count":      len(categories),
	})
}

// ---------- /api/v1/user ----------

func (p *Plugin) apiUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	uid, err := p.apiAuth(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	user, appErr := p.API.GetUser(uid)
	if appErr != nil {
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}

	glpiUserID := 0
	if id, err := p.GetGLPIUserID(uid); err == nil {
		glpiUserID = id
	}

	resp := map[string]interface{}{
		"id":            user.Id,
		"username":      user.Username,
		"email":         user.Email,
		"glpi_user_id":  glpiUserID,
		"is_system_admin": p.IsSystemAdmin(uid),
	}
	writeOK(w, resp)
}

// ---------- /api/v1/system ----------

// apiSystem returns plugin runtime information for the Settings page:
// version, retry-queue configuration, and webhook status. GLPI connectivity is
// reported by the status endpoint to avoid a duplicate health-check round trip.
func (p *Plugin) apiSystem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	config := p.currentConfiguration()

	retryWorkers := 1
	retryMaxAttempts := 5
	retryBackoff := int64(2)
	if p.retryQueue != nil {
		p.retryQueue.mu.Lock()
		retryWorkers = p.retryQueue.workerCount
		retryMaxAttempts = p.retryQueue.maxAttempts
		retryBackoff = int64(p.retryQueue.backoffBase.Seconds())
		p.retryQueue.mu.Unlock()
	}

	webhookConfigured := config != nil && strings.TrimSpace(config.WebhookSecret) != ""

	writeOK(w, map[string]interface{}{
		"plugin_version": commands.PluginVersion,
		"retry_queue": map[string]interface{}{
			"workers":      retryWorkers,
			"max_attempts": retryMaxAttempts,
			"backoff_base": retryBackoff,
		},
		"webhook_configured": webhookConfigured,
	})
}

// ---------- /api/v1/dashboard ----------

// apiDashboard aggregates ticket statistics for the dashboard. Counts come
// from GLPI search totals; a count of -1 means that search failed.
func (p *Plugin) apiDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	uid, _ := p.apiAuth(r)

	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	glpiUserID, err := p.GetGLPIUserID(uid)
	if err != nil {
		writeError(w, http.StatusNotFound, "could not resolve glpi user")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	if glpiUserID <= 0 {
		// No individual GLPI account (integration mode): derive the stats from
		// the identity-service ownership mapping.
		svc := p.currentIdentity()
		if svc == nil {
			writeError(w, http.StatusServiceUnavailable, "identity service not initialized")
			return
		}
		owned, _, err := svc.ListOwnedTickets(ctx, uid, 1, 100, client.GetTicket)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "dashboard lookup failed: "+err.Error())
			return
		}
		writeOK(w, dashboardFromSummaries(owned))
		return
	}

	countFor := func(filter glpi.TicketFilter) int {
		_, total, err := client.SearchTickets(ctx, filter)
		if err != nil {
			return -1
		}
		return total
	}

	openCount := countFor(glpi.TicketFilter{RequesterID: glpiUserID, StatusBelow: glpi.StatusSolved, Limit: 1})
	assignedCount := countFor(glpi.TicketFilter{AssigneeID: glpiUserID, StatusBelow: glpi.StatusSolved, Limit: 1})
	resolvedCount := countFor(glpi.TicketFilter{RequesterID: glpiUserID, Status: glpi.StatusSolved, Limit: 1})
	pendingCount := countFor(glpi.TicketFilter{RequesterID: glpiUserID, Status: glpi.StatusPending, Limit: 1})
	closedCount := countFor(glpi.TicketFilter{RequesterID: glpiUserID, Status: glpi.StatusClosed, Limit: 1})
	criticalCount := countFor(glpi.TicketFilter{RequesterID: glpiUserID, PriorityAtLeast: 5, Limit: 1})
	today := time.Now().Format("2006-01-02")
	overdueCount := countFor(glpi.TicketFilter{RequesterID: glpiUserID, StatusBelow: glpi.StatusSolved, DueDateBefore: today, Limit: 1})

	recent, _, err := client.SearchTickets(ctx, glpi.TicketFilter{RequesterID: glpiUserID, Limit: 5})
	if err != nil {
		recent = nil
	}

	writeOK(w, map[string]interface{}{
		"open":     openCount,
		"assigned": assignedCount,
		"resolved": resolvedCount,
		"pending":  pendingCount,
		"closed":   closedCount,
		"critical": criticalCount,
		"overdue":  overdueCount,
		"recent":   recent,
	})
}

// dashboardFromSummaries computes dashboard stats from an ownership-mapped
// ticket list (integration mode, where GLPI searches cannot be requester-
// scoped). "assigned" and "overdue" are 0 because TicketSummary carries no
// assignee or due-date data.
func dashboardFromSummaries(list []glpi.TicketSummary) map[string]interface{} {
	open, resolved, pending, closed, critical := 0, 0, 0, 0, 0
	for _, t := range list {
		switch {
		case t.Status == glpi.StatusSolved:
			resolved++
		case t.Status == glpi.StatusPending:
			pending++
		case t.Status == glpi.StatusClosed:
			closed++
		case t.Status < glpi.StatusSolved:
			open++
		}
		if t.Priority >= 5 {
			critical++
		}
	}
	recent := list
	if len(recent) > 5 {
		recent = recent[:5]
	}
	return map[string]interface{}{
		"open":     open,
		"assigned": 0,
		"resolved": resolved,
		"pending":  pending,
		"closed":   closed,
		"critical": critical,
		"overdue":  0,
		"recent":   recent,
	}
}

// ---------- /api/v1/categories ----------

func (p *Plugin) apiCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	q := r.URL.Query()
	limit := 50
	if l, err := strconv.Atoi(q.Get("per_page")); err == nil && l > 0 && l <= 200 {
		limit = l
	}

	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	categories, total, err := client.SearchITILCategories(ctx, q.Get("q"), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "category search failed: "+err.Error())
		return
	}

	writeOK(w, map[string]interface{}{
		"categories": categories,
		"total":      total,
		"count":      len(categories),
	})
}

// ---------- /api/v1/knowledge/{id} ----------

func (p *Plugin) apiKnowledgeArticle(w http.ResponseWriter, r *http.Request, idStr string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid article id")
		return
	}

	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	article, err := client.GetKnowbaseItem(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}

	writeOK(w, article)
}

// ---------- /api/v1/assets/{itemType}/{id} ----------

func (p *Plugin) apiAssetDetail(w http.ResponseWriter, r *http.Request, itemType, idStr string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid asset id")
		return
	}

	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	asset, err := client.GetAsset(ctx, itemType, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "asset not found")
		return
	}

	writeOK(w, asset)
}

// ---------- /api/v1/tickets/{id}/documents ----------

func (p *Plugin) apiListDocuments(w http.ResponseWriter, r *http.Request, ticketID int) {
	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	docs, err := client.ListTicketDocuments(ctx, ticketID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "document list failed: "+err.Error())
		return
	}

	writeOK(w, map[string]interface{}{
		"documents": docs,
		"count":     len(docs),
	})
}

// apiDownloadDocument proxies a GLPI document's raw content to the browser.
// handleAPI has already authenticated the request.
func (p *Plugin) apiDownloadDocument(w http.ResponseWriter, r *http.Request, ticketID int, docIDStr string) {
	docID, err := strconv.Atoi(docIDStr)
	if err != nil || docID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid document id")
		return
	}

	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	data, contentType, err := client.GetDocumentContent(ctx, docID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "document download failed")
		return
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "attachment")
	_, _ = w.Write(data)
}
