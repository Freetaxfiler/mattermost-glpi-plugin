package identity

// Service is the centralized identity layer. All plugin modules resolve a
// Mattermost user's GLPI requester identity through it instead of searching
// GLPI directly.
type Service struct {
	kv                KVStore
	lookup            UserLookup
	enableMapping     bool
	ownedTicketsLimit int
}

// New builds a Service. lookup may be nil (Mode A only). enableMapping toggles
// Mode B ("Map Mattermost Users").
func New(kv KVStore, lookup UserLookup, enableMapping bool) *Service {
	return &Service{
		kv:                kv,
		lookup:            lookup,
		enableMapping:     enableMapping,
		ownedTicketsLimit: 200,
	}
}

// EnableMapping toggles Mode B at runtime (e.g. on configuration change).
func (s *Service) EnableMapping(enabled bool) {
	s.enableMapping = enabled
}
