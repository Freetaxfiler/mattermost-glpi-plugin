package glpi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestGetTicketTimelineMergesSourcesAndPaginates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apirest.php/initSession":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"session_token":"abc123"}`))
			return
		case "/apirest.php/Ticket/12/ITILFollowup":
			assertTimelineRequest(t, r, 3)
			w.Header().Set("Content-Range", "0-2/3")
			_, _ = w.Write([]byte(`[
				{"id":10,"content":"Newest follow-up","date":"2026-07-05 09:00:00","is_private":0,"user_name":"Ava"},
				{"id":9,"content":"<p>Older follow-up</p>","date":"2026-07-02 09:00:00","is_private":1,"user_name":"Bea"},
				{"id":8,"content":"First follow-up","date":"2026-07-01 09:00:00"}
			]`))
			return
		case "/apirest.php/Ticket/12/ITILSolution":
			assertTimelineRequest(t, r, 3)
			w.Header().Set("Content-Range", "0-0/1")
			_, _ = w.Write([]byte(`[{"id":7,"content":"Solution","date_creation":"2026-07-04 09:00:00"}]`))
			return
		case "/apirest.php/Ticket/12/TicketValidation":
			assertTimelineRequest(t, r, 3)
			w.Header().Set("Content-Range", "0-0/1")
			_, _ = w.Write([]byte(`[{"id":6,"status":"accepted","date":"2026-07-03 09:00:00"}]`))
			return
		case "/apirest.php/Ticket/12/Log":
			assertTimelineRequest(t, r, 3)
			w.Header().Set("Content-Range", "0-0/1")
			_, _ = w.Write([]byte(`[{"id":5,"new_value":"Status changed","date_mod":"2026-07-01 08:00:00"}]`))
			return
		default:
			t.Fatalf("unexpected request path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "app-token", "user-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	timeline, err := client.GetTicketTimeline(ctx, 12, TimelinePageRequest{Page: 2, PerPage: 2})
	if err != nil {
		t.Fatalf("GetTicketTimeline returned error: %v", err)
	}

	if timeline.Page != 2 || timeline.PerPage != 2 || timeline.Total != 6 || !timeline.HasMore {
		t.Fatalf("unexpected timeline metadata: %+v", timeline)
	}
	if len(timeline.Events) != 2 {
		t.Fatalf("expected two events, got %d", len(timeline.Events))
	}
	if timeline.Events[0].Kind != TimelineValidation || timeline.Events[0].Date != "2026-07-03 09:00:00" {
		t.Fatalf("unexpected first event: %+v", timeline.Events[0])
	}
	if timeline.Events[1].Kind != TimelineFollowup || !timeline.Events[1].IsPrivate || timeline.Events[1].Author != "Bea" {
		t.Fatalf("unexpected second event: %+v", timeline.Events[1])
	}
}

func TestGetTicketTimelineRejectsInvalidPaging(t *testing.T) {
	client, err := NewClient("https://glpi.example.test", "app-token", "user-token", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = client.GetTicketTimeline(context.Background(), 0, TimelinePageRequest{})
	if err == nil {
		t.Fatal("expected an invalid ticket ID error")
	}
	_, err = client.GetTicketTimeline(context.Background(), 1, TimelinePageRequest{Page: 1, PerPage: 101})
	if err == nil {
		t.Fatal("expected an invalid page-size error")
	}
	_, err = client.GetTicketTimeline(context.Background(), 1, TimelinePageRequest{Page: 1001, PerPage: 1})
	if err == nil {
		t.Fatal("expected an inaccessible page error")
	}
}

func assertTimelineRequest(t *testing.T, r *http.Request, expectedRangeEnd int) {
	t.Helper()
	if got := r.Header.Get("Session-Token"); got != "abc123" {
		t.Fatalf("expected session token, got %q", got)
	}
	if got, want := r.URL.Query().Get("range"), "0-"+strconv.Itoa(expectedRangeEnd); got != want {
		t.Fatalf("expected range %q, got %q", want, got)
	}
}
