package identity

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/mattermost/mattermost/server/public/model"
)

// KV key layout for the permanent mapping store. Mattermost KV has no prefix
// listing, so a single index key holds the set of mapped Mattermost user ids.
const (
	mapKeyIDPrefix    = "glpi_map_id:"
	mapKeyEmailPrefix = "glpi_map_eml:"
	mapKeyGLPIIDPrefix = "glpi_map_glpi:"
	mapKeyLoginPrefix = "glpi_map_login:"
	mapKeyIndex       = "glpi_map_index"
)

// ErrMappingNotFound is returned when no mapping exists for a lookup key.
var ErrMappingNotFound = errors.New("no identity mapping found")

func (s *Service) loadMapping(key string) (*Mapping, error) {
	if s.kv == nil {
		return nil, ErrMappingNotFound
	}
	raw, appErr := s.kv.KVGet(key)
	if appErr != nil || len(raw) == 0 {
		return nil, ErrMappingNotFound
	}
	var m Mapping
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, ErrMappingNotFound
	}
	return &m, nil
}

// GetMappingByMMID looks up a mapping by Mattermost user ID.
func (s *Service) GetMappingByMMID(userID string) (*Mapping, error) {
	if userID == "" {
		return nil, ErrMappingNotFound
	}
	return s.loadMapping(mapKeyIDPrefix + userID)
}

// GetMappingByEmail looks up a mapping by the Mattermost user's email.
func (s *Service) GetMappingByEmail(email string) (*Mapping, error) {
	if email == "" {
		return nil, ErrMappingNotFound
	}
	raw, appErr := s.kv.KVGet(mapKeyEmailPrefix + email)
	if appErr != nil || len(raw) == 0 {
		return nil, ErrMappingNotFound
	}
	return s.loadMapping(mapKeyIDPrefix + string(raw))
}

// GetMappingByGLPIID looks up a mapping by GLPI user ID.
func (s *Service) GetMappingByGLPIID(glpiID int) (*Mapping, error) {
	if glpiID <= 0 {
		return nil, ErrMappingNotFound
	}
	raw, appErr := s.kv.KVGet(mapKeyGLPIIDPrefix + strconv.Itoa(glpiID))
	if appErr != nil || len(raw) == 0 {
		return nil, ErrMappingNotFound
	}
	return s.loadMapping(mapKeyIDPrefix + string(raw))
}

// GetMappingByLogin looks up a mapping by the GLPI login.
func (s *Service) GetMappingByLogin(login string) (*Mapping, error) {
	if login == "" {
		return nil, ErrMappingNotFound
	}
	raw, appErr := s.kv.KVGet(mapKeyLoginPrefix + login)
	if appErr != nil || len(raw) == 0 {
		return nil, ErrMappingNotFound
	}
	return s.loadMapping(mapKeyIDPrefix + string(raw))
}

// SaveMapping persists a mapping and its secondary indexes.
func (s *Service) SaveMapping(m *Mapping) error {
	if s.kv == nil || m == nil || m.MMUserID == "" {
		return errors.New("identity store is unavailable")
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if appErr := s.kv.KVSet(mapKeyIDPrefix+m.MMUserID, data); appErr != nil {
		return errors.New(appErr.Error())
	}
	if m.MMEmail != "" {
		_ = s.kv.KVSet(mapKeyEmailPrefix+m.MMEmail, []byte(m.MMUserID))
	}
	if m.GLPIUserID > 0 {
		_ = s.kv.KVSet(mapKeyGLPIIDPrefix+strconv.Itoa(m.GLPIUserID), []byte(m.MMUserID))
	}
	if m.GLPILogin != "" {
		_ = s.kv.KVSet(mapKeyLoginPrefix+m.GLPILogin, []byte(m.MMUserID))
	}
	return s.addToIndex(m.MMUserID)
}

// RemoveMapping deletes a mapping and its indexes.
func (s *Service) RemoveMapping(userID string) error {
	if s.kv == nil || userID == "" {
		return ErrMappingNotFound
	}
	m, err := s.loadMapping(mapKeyIDPrefix + userID)
	if err != nil {
		return err
	}
	_ = s.kv.KVDelete(mapKeyIDPrefix + userID)
	if m.MMEmail != "" {
		_ = s.kv.KVDelete(mapKeyEmailPrefix + m.MMEmail)
	}
	if m.GLPIUserID > 0 {
		_ = s.kv.KVDelete(mapKeyGLPIIDPrefix + strconv.Itoa(m.GLPIUserID))
	}
	if m.GLPILogin != "" {
		_ = s.kv.KVDelete(mapKeyLoginPrefix + m.GLPILogin)
	}
	return s.removeFromIndex(userID)
}

// AllMappings returns every stored mapping.
func (s *Service) AllMappings() ([]Mapping, error) {
	if s.kv == nil {
		return nil, nil
	}
	raw, _ := s.kv.KVGet(mapKeyIndex)
	var ids []string
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &ids)
	}
	out := make([]Mapping, 0, len(ids))
	for _, id := range ids {
		if m, err := s.loadMapping(mapKeyIDPrefix + id); err == nil {
			out = append(out, *m)
		}
	}
	return out, nil
}

// ClearCache clears the mapping cache and index. It does not touch the
// ownership mapping used by "My Tickets".
func (s *Service) ClearCache() error {
	mappings, err := s.AllMappings()
	if err != nil {
		return err
	}
	for _, m := range mappings {
		_ = s.RemoveMapping(m.MMUserID)
	}
	_ = s.kv.KVDelete(mapKeyIndex)
	return nil
}

func (s *Service) addToIndex(userID string) error {
	raw, _ := s.kv.KVGet(mapKeyIndex)
	var ids []string
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &ids)
	}
	for _, id := range ids {
		if id == userID {
			return nil
		}
	}
	ids = append(ids, userID)
	data, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	return appErrAsError(s.kv.KVSet(mapKeyIndex, data))
}

func (s *Service) removeFromIndex(userID string) error {
	raw, _ := s.kv.KVGet(mapKeyIndex)
	var ids []string
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &ids)
	}
	out := ids[:0]
	for _, id := range ids {
		if id != userID {
			out = append(out, id)
		}
	}
	data, err := json.Marshal(out)
	if err != nil {
		return err
	}
	return appErrAsError(s.kv.KVSet(mapKeyIndex, data))
}

// appErrAsError converts a *model.AppError to a plain error, avoiding the
// typed-nil interface trap when the KV operation succeeds.
func appErrAsError(appErr *model.AppError) error {
	if appErr != nil {
		return errors.New(appErr.Error())
	}
	return nil
}
