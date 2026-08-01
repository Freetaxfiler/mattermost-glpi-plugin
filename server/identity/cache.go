package identity

import "strconv"

const (
	userMapKeyPrefix      = "glpi_uid_map_"
	mapPositiveTTLSeconds = 24 * 60 * 60 // 24h
	mapNegativeTTLSeconds = 15 * 60      // 15m
)

// cachedGLPIUserID reads a cached email→GLPI user mapping. The boolean is
// false when there is no cached entry.
func (s *Service) cachedGLPIUserID(email string) (int, bool) {
	if s.kv == nil {
		return 0, false
	}
	raw, appErr := s.kv.KVGet(userMapKeyPrefix + email)
	if appErr != nil || len(raw) == 0 {
		return 0, false
	}
	id, err := strconv.Atoi(string(raw))
	if err != nil {
		return 0, false
	}
	return id, true
}

// cacheGLPIUserID stores an email→GLPI user mapping. id 0 caches a short
// negative so newly-created GLPI users are picked up quickly.
func (s *Service) cacheGLPIUserID(email string, id int) {
	if s.kv == nil {
		return
	}
	ttl := int64(mapPositiveTTLSeconds)
	if id == 0 {
		ttl = mapNegativeTTLSeconds
	}
	_ = s.kv.KVSetWithExpiry(userMapKeyPrefix+email, []byte(strconv.Itoa(id)), ttl)
}
