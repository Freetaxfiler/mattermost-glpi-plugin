// Package middleware provides the single authentication and request-context
// layer for the GLPI plugin's REST API.
//
// # Authentication model
//
// Mattermost's plugin HTTP router authenticates every request destined for a
// plugin before it is delivered: it resolves the session token (from the
// Authorization header, the MMAUTHTOKEN cookie, or an access_token query
// parameter), validates it, strips any client-supplied authorization material,
// and then — and only then — injects the server-set Mattermost-User-Id header
// derived from that validated session. The plugin therefore never sees a raw
// bearer token, and a client cannot forge a Mattermost-User-Id header.
//
// This middleware treats that server-injected header as the source of truth
// for authentication, resolves the full user record once, and exposes it to
// handlers via the request context. Every /api/v1 handler receives
// CurrentUser instead of performing its own authentication.
package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

// CurrentUser is the authenticated Mattermost user attached to every
// authenticated API request. TeamID and ChannelID are present only when the
// caller supplied them (as query parameters) and are metadata for ticket
// attribution — never authorization inputs.
type CurrentUser struct {
	UserID        string
	Username      string
	Email         string
	Roles         string
	IsSystemAdmin bool
	TeamID        string
	ChannelID     string

	// User is the full Mattermost user record for callers that need fields
	// beyond the flattened ones above.
	User *model.User
}

// HasRole reports whether the user carries the given role id in their role
// string (e.g. model.SystemAdminRoleId).
func (u *CurrentUser) HasRole(roleID string) bool {
	for _, role := range strings.Split(u.Roles, " ") {
		if role == roleID {
			return true
		}
	}
	return false
}

// Authenticator resolves the current user for a request. Implementations must
// derive the user from server-validated authentication material only.
type Authenticator func(r *http.Request) (*CurrentUser, error)

// Middleware authenticates requests and injects the CurrentUser into the
// request context before the wrapped handler runs.
type Middleware struct {
	authenticate Authenticator
}

// New returns a Middleware using the given authenticator.
func New(authenticate Authenticator) *Middleware {
	if authenticate == nil {
		panic("middleware.New requires an authenticator")
	}
	return &Middleware{authenticate: authenticate}
}

type ctxKey struct{}

// FromRequest returns the authenticated user attached to the request by
// RequireAuth, or nil when the request is not authenticated.
func FromRequest(r *http.Request) *CurrentUser {
	user, _ := r.Context().Value(ctxKey{}).(*CurrentUser)
	return user
}

// RequireAuth wraps next so that only authenticated requests reach it.
// Unauthenticated requests receive a structured JSON error and never invoke
// next. Use it as:
//
//	middleware.RequireAuth(http.HandlerFunc(p.handleAPI)).ServeHTTP(w, r)
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := m.authenticate(r)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "error",
				"error":  "authentication required",
			})
			return
		}
		if user == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "error",
				"error":  "authentication required",
			})
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
