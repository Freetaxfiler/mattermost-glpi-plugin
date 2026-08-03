package identity

// Service is the centralized identity layer. All plugin modules resolve a
// Mattermost user's GLPI identity (requester, role, owned tickets) through it
// instead of searching GLPI directly.
type Service struct {
	kv                KVStore
	users             UserStore
	enableMapping     bool
	ownedTicketsLimit int
}

// New builds a Service. users may be nil (Mode A only). enableMapping toggles
// Mode B ("Map Mattermost Users").
func New(kv KVStore, users UserStore, enableMapping bool) *Service {
	return &Service{
		kv:                kv,
		users:             users,
		enableMapping:     enableMapping,
		ownedTicketsLimit: 200,
	}
}

// EnableMapping toggles Mode B at runtime (e.g. on configuration change).
func (s *Service) EnableMapping(enabled bool) {
	s.enableMapping = enabled
}
