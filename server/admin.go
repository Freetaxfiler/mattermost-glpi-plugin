package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/glpi"
	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/identity"
	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/middleware"
	"github.com/mattermost/mattermost/server/public/model"
)

// lastSyncKey records the last user-sync timestamp (epoch seconds).
const lastSyncKey = "glpi_last_sync"

// apiAdmin guards every /api/v1/admin/* route behind system-admin membership
// and dispatches to the sub-handlers. Authentication and the system-admin flag
// come from the auth middleware request context.
func (p *Plugin) apiAdmin(w http.ResponseWriter, r *http.Request, parts []string) {
	cu := middleware.FromRequest(r)
	if cu == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if !cu.IsSystemAdmin {
		writeError(w, http.StatusForbidden, "administrator privileges required")
		return
	}

	switch {
	case len(parts) >= 2 && parts[1] == "mappings":
		p.apiAdminMappings(w, r)
	case len(parts) >= 2 && parts[1] == "sync":
		p.apiAdminSync(w, r)
	case len(parts) >= 2 && parts[1] == "provision":
		p.apiAdminProvision(w, r)
	case len(parts) >= 2 && parts[1] == "clear-cache":
		p.apiAdminClearCache(w, r)
	default:
		writeError(w, http.StatusNotFound, "unknown admin endpoint")
	}
}

// apiAdminMappings lists the mapping state: stored mappings, unmapped
// Mattermost users, and Mattermost users sharing an email (duplicate risk).
func (p *Plugin) apiAdminMappings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	svc := p.currentIdentity()
	mappings := []identity.Mapping{}
	if svc != nil {
		if all, err := svc.AllMappings(); err == nil {
			mappings = all
		}
	}
	mappedByID := map[string]bool{}
	for _, m := range mappings {
		mappedByID[m.MMUserID] = true
	}

	// Enumerate Mattermost users for unmapped/duplicate classification.
	mmUsers, appErr := p.API.GetUsers(&model.UserGetOptions{Page: 0, PerPage: 1000})
	if appErr != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users: "+appErr.Error())
		return
	}

	type mmRow struct {
		UserID   string `json:"user_id"`
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	unmapped := []mmRow{}
	emailSeen := map[string]string{} // email -> first MM user id
	duplicates := [][]mmRow{}
	for _, u := range mmUsers {
		if u.IsBot {
			continue
		}
		row := mmRow{UserID: u.Id, Username: u.Username, Email: u.Email}
		if !mappedByID[u.Id] {
			unmapped = append(unmapped, row)
		}
		if prev, ok := emailSeen[u.Email]; ok && u.Email != "" {
			// group duplicates: find existing group or start one
			appended := false
			for i := range duplicates {
				if duplicates[i][0].Email == u.Email {
					duplicates[i] = append(duplicates[i], row)
					appended = true
					break
				}
			}
			if !appended {
				duplicates = append(duplicates, []mmRow{{UserID: prev, Email: u.Email}, row})
			}
		} else if u.Email != "" {
			emailSeen[u.Email] = u.Id
		}
	}
	sort.Slice(mappings, func(i, j int) bool { return mappings[i].LastSync > mappings[j].LastSync })

	writeOK(w, map[string]interface{}{
		"mappings":         mappings,
		"unmapped":         unmapped,
		"duplicate_emails": duplicates,
		"mm_user_count":    len(mmUsers),
		"mapping_enabled":  p.currentConfiguration() != nil && p.currentConfiguration().EnableUserMapping,
	})
}

// apiAdminSync re-runs automatic discovery for every unmapped Mattermost user.
// It is idempotent and best-effort per user.
func (p *Plugin) apiAdminSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	svc := p.currentIdentity()
	if svc == nil {
		writeError(w, http.StatusServiceUnavailable, "identity service not initialized")
		return
	}

	mmUsers, appErr := p.API.GetUsers(&model.UserGetOptions{Page: 0, PerPage: 1000})
	if appErr != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users: "+appErr.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	mapped, errorsN := 0, 0
	for _, u := range mmUsers {
		if u.IsBot {
			continue
		}
		if _, err := svc.GetMappingByMMID(u.Id); err == nil {
			continue // already mapped
		}
		mm := &identity.MMUser{
			UserID:      u.Id,
			Username:    u.Username,
			DisplayName: displayNameFor(u),
			Email:       u.Email,
		}
		if _, err := svc.DiscoverAndMap(ctx, mm); err == nil {
			mapped++
		} else if err != identity.ErrMappingNotFound {
			errorsN++
		}
	}

	now := time.Now().Unix()
	_ = p.API.KVSet(lastSyncKey, []byte(strconv.FormatInt(now, 10)))

	writeOK(w, map[string]interface{}{
		"mapped":    mapped,
		"skipped":   len(mmUsers) - mapped,
		"errors":    errorsN,
		"synced_at": now,
	})
}

// apiAdminProvision creates a GLPI account for an unmapped Mattermost user and
// maps it. Body: {"user_id": "...", "profile_id": 5, "entity_id": 0}.
func (p *Plugin) apiAdminProvision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		UserID    string `json:"user_id"`
		ProfileID int    `json:"profile_id"`
		EntityID  int    `json:"entity_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	svc := p.currentIdentity()
	if svc == nil {
		writeError(w, http.StatusServiceUnavailable, "identity service not initialized")
		return
	}
	user, appErr := p.API.GetUser(req.UserID)
	if appErr != nil {
		writeError(w, http.StatusNotFound, "mattermost user not found")
		return
	}
	mm := &identity.MMUser{
		UserID:      user.Id,
		Username:    user.Username,
		DisplayName: displayNameFor(user),
		Email:       user.Email,
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	m, err := svc.ProvisionUser(ctx, mm, req.ProfileID, req.EntityID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "provisioning failed: "+err.Error())
		return
	}
	writeOK(w, map[string]interface{}{
		"mapping":      m,
		"provisioned":  true,
		"glpi_user_id": m.GLPIUserID,
	})
}

// apiAdminClearCache drops the mapping cache (not the ownership mapping).
func (p *Plugin) apiAdminClearCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if svc := p.currentIdentity(); svc != nil {
		if err := svc.ClearCache(); err != nil {
			writeError(w, http.StatusInternalServerError, "clear cache failed: "+err.Error())
			return
		}
	}
	writeOK(w, map[string]interface{}{"cleared": true})
}

// apiMyAssets returns assets assigned to the mapped GLPI user across all
// asset types that support user links. Unmapped users get an empty list.
func (p *Plugin) apiMyAssets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	uid := currentUserID(r)
	if uid == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	client := p.GetGLPIClient()
	if client == nil {
		writeError(w, http.StatusServiceUnavailable, "glpi client not initialized")
		return
	}
	glpiUserID, err := p.GetGLPIUserID(uid)
	if err != nil {
		glpiUserID = 0
	}
	if glpiUserID <= 0 {
		writeOK(w, map[string]interface{}{"assets": []glpi.AssetSummary{}, "total": 0, "count": 0, "mapped": false})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	all := []glpi.AssetSummary{}
	total := 0
	for itemType := range glpi.AssetItemTypes {
		if !glpi.SupportsUserFilter(glpi.AssetItemTypes[itemType]) {
			continue
		}
		assets, t, err := client.SearchAssets(ctx, glpi.AssetFilter{
			ItemType:   glpi.AssetItemTypes[itemType],
			GLPIUserID: glpiUserID,
			Limit:      50,
			Page:       1,
		})
		if err != nil {
			continue // best-effort per type
		}
		total += t
		for i := range assets {
			assets[i].ItemType = glpi.AssetItemTypes[itemType]
			all = append(all, assets[i])
		}
	}
	writeOK(w, map[string]interface{}{"assets": all, "total": total, "count": len(all), "mapped": true})
}

func displayNameFor(u *model.User) string {
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name == "" {
		name = u.Username
	}
	return name
}
