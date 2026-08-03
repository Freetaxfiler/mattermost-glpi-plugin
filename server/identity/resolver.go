package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/glpi"
)

// ResolveRequester returns the GLPI requester identity for a Mattermost user.
// It never fails: a permanent mapping wins, otherwise Mode B discovery applies
// (when enabled), and everything falls back to the integration account.
// Ticket creation must never abort because of an unknown GLPI user.
func (s *Service) ResolveRequester(ctx context.Context, user *MMUser) Requester {
	if user == nil {
		return Requester{Mode: ModeIntegration, GLPIUserID: 0}
	}
	if m, err := s.GetMappingByMMID(user.UserID); err == nil && m.GLPIUserID > 0 {
		return Requester{Mode: ModeMapped, GLPIUserID: m.GLPIUserID, User: *user}
	}
	if s.enableMapping && s.users != nil {
		if m, err := s.DiscoverAndMap(ctx, user); err == nil && m.GLPIUserID > 0 {
			return Requester{Mode: ModeMapped, GLPIUserID: m.GLPIUserID, User: *user}
		}
	}
	return Requester{Mode: ModeIntegration, GLPIUserID: 0, User: *user}
}

// RoleForMMUser returns the effective role for a Mattermost user, consulting
// the permanent mapping first and falling back to a live profile lookup.
func (s *Service) RoleForMMUser(ctx context.Context, user *MMUser) (Role, []string) {
	if user == nil {
		return RoleEmployee, nil
	}
	if m, err := s.GetMappingByMMID(user.UserID); err == nil && m.GLPIUserID > 0 {
		return Role(m.Role), m.Profiles
	}
	if s.users != nil && s.enableMapping {
		if m, err := s.DiscoverAndMap(ctx, user); err == nil {
			return Role(m.Role), m.Profiles
		}
	}
	return RoleEmployee, nil
}

// DiscoverAndMap finds a GLPI user for the Mattermost user and persists the
// mapping. Discovery priority: email → login (username) → display name.
// It returns a descriptive error (never panics, never silently returns nil).
func (s *Service) DiscoverAndMap(ctx context.Context, mm *MMUser) (*Mapping, error) {
	if s.users == nil {
		return nil, errors.New("glpi user store is unavailable; identity mapping disabled")
	}
	if mm == nil {
		return nil, errors.New("mattermost user is nil")
	}

	var found *glpi.UserSummary
	var lastErr error
	if mm.Email != "" {
		if u, err := s.users.FindUserByEmail(ctx, mm.Email); err == nil && u != nil && u.ID > 0 {
			found = u
		} else if err != nil {
			lastErr = err
		}
	}
	if found == nil && mm.Username != "" {
		if u, err := s.users.FindUserByLogin(ctx, mm.Username); err == nil && u != nil && u.ID > 0 {
			found = u
		} else if err != nil {
			lastErr = err
		}
	}
	if found == nil && mm.DisplayName != "" {
		first, last := splitDisplayName(mm.DisplayName)
		if u, err := s.users.FindUserByName(ctx, first, last); err == nil && u != nil && u.ID > 0 {
			found = u
		} else if err != nil {
			lastErr = err
		}
	}
	if found == nil {
		if lastErr != nil {
			return nil, fmt.Errorf("%w (glpi lookup: %v)", ErrMappingNotFound, lastErr)
		}
		return nil, ErrMappingNotFound
	}

	return s.mapToUser(ctx, mm, found)
}

// mapToUser persists a mapping against an existing GLPI user and resolves its role.
func (s *Service) mapToUser(ctx context.Context, mm *MMUser, u *glpi.UserSummary) (*Mapping, error) {
	m := &Mapping{
		MMUserID:      mm.UserID,
		MMUsername:    mm.Username,
		MMEmail:       mm.Email,
		MMDisplayName: mm.DisplayName,
		GLPIUserID:    u.ID,
		GLPILogin:     u.Login,
		GLPIFullName:  strings.TrimSpace(u.Firstname + " " + u.Realname),
		GLPIMail:      u.Email,
		SyncStatus:    "mapped",
		LastSync:      time.Now().Unix(),
	}
	if role, profiles, err := s.ResolveRole(ctx, u.ID); err == nil {
		m.Role = string(role)
		m.Profiles = profiles
	} else {
		m.Role = string(RoleEmployee)
	}
	if err := s.SaveMapping(m); err != nil {
		return nil, err
	}
	return m, nil
}

// ProvisionUser creates a GLPI account for a Mattermost user (admin action) and
// maps them. It never duplicates: an existing mapping or a GLPI user with the
// same email is reused instead of creating a second account.
func (s *Service) ProvisionUser(ctx context.Context, mm *MMUser, profileID, entityID int) (*Mapping, error) {
	if s.users == nil {
		return nil, errors.New("glpi user store is unavailable; cannot provision")
	}
	if mm == nil {
		return nil, errors.New("mattermost user is nil")
	}
	if m, err := s.GetMappingByMMID(mm.UserID); err == nil && m.GLPIUserID > 0 {
		return m, nil
	}
	if mm.Email != "" {
		if u, err := s.users.FindUserByEmail(ctx, mm.Email); err == nil && u != nil && u.ID > 0 {
			return s.mapToUser(ctx, mm, u)
		}
	}
	login := sanitizeLogin(mm.Username, mm.Email)
	first, last := splitDisplayName(mm.DisplayName)
	newID, err := s.users.CreateUser(ctx, glpi.CreateUserRequest{
		Login:     login,
		Firstname: first,
		Realname:  last,
		Email:     mm.Email,
		ProfileID: profileID,
		EntityID:  entityID,
		Recursive: 1,
		Active:    1,
	})
	if err != nil {
		return nil, fmt.Errorf("glpi user creation failed: %w", err)
	}
	return s.mapToUser(ctx, mm, &glpi.UserSummary{ID: newID, Login: login, Firstname: first, Realname: last, Email: mm.Email})
}

// splitDisplayName splits a "Firstname Lastname" display name.
func splitDisplayName(display string) (first, last string) {
	parts := strings.Fields(strings.TrimSpace(display))
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], ""
	default:
		return parts[0], parts[len(parts)-1]
	}
}

// sanitizeLogin produces a safe GLPI login from the Mattermost username,
// falling back to the email local-part.
func sanitizeLogin(username, email string) string {
	login := strings.TrimSpace(username)
	if login == "" && strings.Contains(email, "@") {
		login = strings.SplitN(email, "@", 2)[0]
	}
	login = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, login)
	return login
}
