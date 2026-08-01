package identity

import "context"

// ResolveRequester returns the GLPI requester identity for a Mattermost user.
// It never fails: Mode A always returns the integration account (id 0), and
// Mode B falls back to the integration account when no GLPI user matches.
// Ticket creation must never abort because of an unknown GLPI user.
func (s *Service) ResolveRequester(ctx context.Context, user *MMUser) Requester {
	if user == nil {
		return Requester{Mode: ModeIntegration, GLPIUserID: 0}
	}
	if !s.enableMapping || user.Email == "" {
		return Requester{Mode: ModeIntegration, GLPIUserID: 0, User: *user}
	}
	if id, ok := s.cachedGLPIUserID(user.Email); ok {
		if id > 0 {
			return Requester{Mode: ModeMapped, GLPIUserID: id, User: *user}
		}
		// Cached negative (email not present in GLPI) — fallback.
		return Requester{Mode: ModeIntegration, GLPIUserID: 0, User: *user}
	}
	if s.lookup != nil {
		if id, err := s.lookup.FindUserIDByEmail(ctx, user.Email); err == nil && id > 0 {
			s.cacheGLPIUserID(user.Email, id)
			return Requester{Mode: ModeMapped, GLPIUserID: id, User: *user}
		}
	}
	// No match (or lookup error) — cache the negative briefly and fall back.
	s.cacheGLPIUserID(user.Email, 0)
	return Requester{Mode: ModeIntegration, GLPIUserID: 0, User: *user}
}
