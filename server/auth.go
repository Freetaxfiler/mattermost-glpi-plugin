package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/middleware"
)

// authMiddleware is the lazily-constructed single authentication layer for the
// REST API. It is safe for concurrent use.
func (p *Plugin) authMiddleware() *middleware.Middleware {
	p.authMWOnce.Do(func() {
		p.authMW = middleware.New(p.authenticate)
	})
	return p.authMW
}

// authenticate resolves the authenticated Mattermost user for an API request.
//
// Mattermost's plugin HTTP router validates the request session server-side and
// then injects the Mattermost-User-Id header derived from that validated
// session (it strips any client-supplied value first, and strips the bearer
// token/cookie so the plugin cannot read raw credentials). This method trusts
// that server-injected header as the single authentication input, then resolves
// the full user record once so handlers never re-validate.
func (p *Plugin) authenticate(r *http.Request) (*middleware.CurrentUser, error) {
	userID := strings.TrimSpace(r.Header.Get("Mattermost-User-Id"))
	if userID == "" {
		return nil, errors.New("not authenticated")
	}

	user, appErr := p.API.GetUser(userID)
	if appErr != nil || user == nil {
		return nil, fmt.Errorf("authenticated user not found: %s", userID)
	}
	if user.IsBot {
		return nil, errors.New("bot sessions are not permitted")
	}

	// IsSystemAdmin mirrors the existing helper (HasPermissionTo ManageSystem)
	// so admin gating behaves exactly as before the refactor.
	isSystemAdmin := p.IsSystemAdmin(user.Id)

	// Team/Channel are optional attribution metadata supplied by the caller
	// (e.g. the webapp) and are never used for authorization.
	cu := &middleware.CurrentUser{
		UserID:        user.Id,
		Username:      user.Username,
		Email:         user.Email,
		Roles:         user.Roles,
		IsSystemAdmin: isSystemAdmin,
		User:          user,
	}
	cu.TeamID = strings.TrimSpace(r.URL.Query().Get("team_id"))
	cu.ChannelID = strings.TrimSpace(r.URL.Query().Get("channel_id"))
	return cu, nil
}
