package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebhookHandlerDoesNotClaimMalformedPayload(t *testing.T) {
	var dedupeCalls int
	handler := &WebhookHandler{
		Secret: "test-secret",
		Notify: func(WebhookEvent) {},
		Dedupe: func(string) (bool, error) {
			dedupeCalls++
			return false, nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader("not-json"))
	req.Header.Set("X-GLPI-Secret", "test-secret")
	res := httptest.NewRecorder()
	handler.HandleWebhook(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d", res.Code)
	}
	if dedupeCalls != 0 {
		t.Fatalf("malformed payload must not be deduplicated, got %d calls", dedupeCalls)
	}
}

func TestWebhookHandlerReturnsSuccessForReplay(t *testing.T) {
	var notifications int
	handler := &WebhookHandler{
		Secret: "test-secret",
		Notify: func(WebhookEvent) { notifications++ },
		Dedupe: func(string) (bool, error) { return true, nil },
	}

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"event":"ticket.updated","ticket_id":42}`))
	req.Header.Set("X-GLPI-Secret", "test-secret")
	res := httptest.NewRecorder()
	handler.HandleWebhook(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected success for replay, got %d", res.Code)
	}
	if notifications != 0 {
		t.Fatalf("replayed payload must not notify, got %d notifications", notifications)
	}
}
