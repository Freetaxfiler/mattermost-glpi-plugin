package glpi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newSearchTestServer(t *testing.T, expectPath string, payload string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apirest.php/initSession":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"session_token":"abc123"}`))
		case expectPath:
			if got := r.Header.Get("Session-Token"); got != "abc123" {
				t.Fatalf("expected Session-Token header, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte(payload))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
}

func TestSearchTicketsParsesRows(t *testing.T) {
	payload := `{"totalcount":2,"count":2,"data":[` +
		`{"1":"Printer broken","2":41,"12":2,"3":"3","15":"2026-07-01 10:00:00"},` +
		`{"1":"VPN down","2":"42","12":"1","3":4,"15":"2026-07-02 09:30:00"}]}`
	server := newSearchTestServer(t, "/apirest.php/search/Ticket", payload)
	defer server.Close()

	client, err := NewClient(server.URL, "app-token", "user-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tickets, total, err := client.SearchTickets(ctx, TicketFilter{RequesterID: 7, Limit: 10})
	if err != nil {
		t.Fatalf("SearchTickets returned error: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(tickets) != 2 {
		t.Fatalf("expected 2 tickets, got %d", len(tickets))
	}
	if tickets[0].ID != 41 || tickets[0].Name != "Printer broken" || tickets[0].Status != 2 || tickets[0].Priority != 3 {
		t.Fatalf("unexpected first ticket: %+v", tickets[0])
	}
	if tickets[1].ID != 42 || tickets[1].Status != 1 || tickets[1].Priority != 4 {
		t.Fatalf("unexpected second ticket: %+v", tickets[1])
	}
}

func TestFindUserIDByEmailNotFound(t *testing.T) {
	server := newSearchTestServer(t, "/apirest.php/search/User", `{"totalcount":0,"count":0,"data":[]}`)
	defer server.Close()

	client, err := NewClient(server.URL, "app-token", "user-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.FindUserIDByEmail(ctx, "nobody@example.com")
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if _, ok := err.(*NotFoundError); !ok {
		t.Fatalf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestStripHTML(t *testing.T) {
	input := "<p>Hello &amp; welcome</p><ul><li>one</li><li>two</li></ul>"
	got := StripHTML(input)
	expected := "Hello & welcome\none\ntwo"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}
