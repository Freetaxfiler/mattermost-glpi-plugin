package glpi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthCheckReturnsVersion(t *testing.T) {
	var initCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apirest.php/initSession":
			initCalls++
			if got := r.Header.Get("App-Token"); got != "app-token" {
				t.Fatalf("expected App-Token header, got %q", got)
			}
			if got := r.Header.Get("Authorization"); got != "user_token user-token" {
				t.Fatalf("expected user token auth header, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"session_token":"abc123"}`))
		case "/apirest.php/getGlpiConfig":
			// getGlpiConfig is the only health-check endpoint: GLPI 11 has no
			// /apirest.php/healthcheck predefined endpoint and rejects it with
			// ERROR_RESOURCE_NOT_FOUND_NOR_COMMONDBTM.
			if got := r.Header.Get("Session-Token"); got != "abc123" {
				t.Fatalf("expected Session-Token header, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"10.0.17"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "app-token", "user-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := client.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("HealthCheck returned error: %v", err)
	}
	if result.Version != "10.0.17" {
		t.Fatalf("expected version 10.0.17, got %q", result.Version)
	}
	if initCalls != 1 {
		t.Fatalf("expected exactly one initSession call, got %d", initCalls)
	}
}

func TestHealthCheckReturnsAuthenticationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid token"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "app-token", "user-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.HealthCheck(ctx)
	if err == nil {
		t.Fatal("expected authentication error")
	}

	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError, got %T", err)
	}
}
