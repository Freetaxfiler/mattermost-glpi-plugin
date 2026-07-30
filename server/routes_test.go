package main

import (
	"bytes"
	"sync"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

type memoryWebhookFingerprintStore struct {
	mu      sync.Mutex
	values  map[string][]byte
	expires map[string]int64
}

func newMemoryWebhookFingerprintStore() *memoryWebhookFingerprintStore {
	return &memoryWebhookFingerprintStore{
		values:  map[string][]byte{},
		expires: map[string]int64{},
	}
}

func (s *memoryWebhookFingerprintStore) KVCompareAndSet(key string, oldValue, newValue []byte) (bool, *model.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, found := s.values[key]
	if !found {
		current = nil
	}
	if !bytes.Equal(current, oldValue) {
		return false, nil
	}
	s.values[key] = append([]byte(nil), newValue...)
	return true, nil
}

func (s *memoryWebhookFingerprintStore) KVSetWithExpiry(key string, value []byte, expireInSeconds int64) *model.AppError {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = append([]byte(nil), value...)
	s.expires[key] = expireInSeconds
	return nil
}

func (s *memoryWebhookFingerprintStore) KVDelete(key string) *model.AppError {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, key)
	delete(s.expires, key)
	return nil
}

func TestClaimWebhookFingerprintIsAtomicAndExpires(t *testing.T) {
	store := newMemoryWebhookFingerprintStore()
	seen, err := claimWebhookFingerprint(store, "abc", 600)
	if err != nil || seen {
		t.Fatalf("first claim = seen:%t err:%v, want unseen without error", seen, err)
	}

	seen, err = claimWebhookFingerprint(store, "abc", 600)
	if err != nil || !seen {
		t.Fatalf("second claim = seen:%t err:%v, want seen without error", seen, err)
	}
	if got := store.expires["glpi_webhook_abc"]; got != 600 {
		t.Fatalf("expiry = %d, want 600", got)
	}
}
