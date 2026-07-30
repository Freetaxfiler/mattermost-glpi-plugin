package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	case "tickets":
		p.apiTickets(w, r, parts)
	case "assets":
		p.apiAssets(w, r)
	case "knowledge":
		p.apiKnowledge(w, r)
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

	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	var glpiFilter glpi.TicketFilter
	glpiFilter.Limit = limit

	switch filterType {
	case "my":
		glpiUserID, err := p.GetGLPIUserID(uid)
		if err != nil {
			writeError(w, http.StatusNotFound, "could not resolve glpi user")
			return
		}
		glpiFilter.RequesterID = glpiUserID
	case "assigned":
		glpiUserID, err := p.GetGLPIUserID(uid)
		if err != nil {
			writeError(w, http.StatusNotFound, "could not resolve glpi user")
			return
		}
		glpiFilter.AssigneeID = glpiUserID
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

	requesterID := 0
	if glpiUserID, err := p.GetGLPIUserID(uid); err == nil {
		requesterID = glpiUserID
	}

	categoryID := req.CategoryID
	if categoryID <= 0 && config != nil && strings.TrimSpace(config.DefaultCategory) != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(config.DefaultCategory)); err == nil && parsed > 0 {
			categoryID = parsed
		}
	}

	createReq := glpi.CreateTicketRequest{
		Name:           strings.TrimSpace(req.Subject),
		Content:        strings.TrimSpace(req.Content),
		Priority:       req.Priority,
		Urgency:        req.Urgency,
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

	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	filter := glpi.AssetFilter{
		ItemType:  itemType,
		NameQuery: search,
		Limit:     limit,
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

	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "search query (q) is required")
		return
	}

	limit := 15
	if l, err := strconv.Atoi(r.URL.Query().Get("per_page")); err == nil && l > 0 && l <= 100 {
		limit = l
	}

	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	articles, total, err := client.SearchKnowledge(ctx, query, limit)
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
