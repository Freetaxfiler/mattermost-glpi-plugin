package glpi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestClient spins up a GLPI API mock with a session endpoint and returns a
// configured client pointed at it.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, "app-token", "user-token", server.Client())
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	return client
}

func sessionHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/apirest.php/initSession" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"session_token":"abc123"}`))
			return
		}
		next(w, r)
	}
}

func TestSearchITILCategoriesParsesRows(t *testing.T) {
	client := newTestClient(t, sessionHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apirest.php/search/ITILCategory" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totalcount":2,"count":2,"data":[{"2":1,"1":"Helpdesk"},{"2":2,"1":"Network"}]}`))
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	categories, total, err := client.SearchITILCategories(ctx, "", 15)
	if err != nil {
		t.Fatalf("SearchITILCategories returned error: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(categories) != 2 || categories[0].ID != 1 || categories[0].Name != "Helpdesk" {
		t.Fatalf("unexpected categories: %+v", categories)
	}
}

func TestGetKnowbaseItemParsesArticle(t *testing.T) {
	client := newTestClient(t, sessionHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apirest.php/KnowbaseItem/5" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":5,"name":"Reset password","answer":"<p>Press the button</p>","knowbaseitemcategories_id":{"id":2,"name":"FAQ"},"date":"2024-01-01","date_mod":"2024-02-01"}`))
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	article, err := client.GetKnowbaseItem(ctx, 5)
	if err != nil {
		t.Fatalf("GetKnowbaseItem returned error: %v", err)
	}
	if article.Subject != "Reset password" {
		t.Fatalf("unexpected subject %q", article.Subject)
	}
	if article.Content != "<p>Press the button</p>" {
		t.Fatalf("unexpected content %q", article.Content)
	}
	if article.Category != "FAQ" {
		t.Fatalf("expected dropdown category FAQ, got %q", article.Category)
	}
}

func TestGetAssetResolvesDropdowns(t *testing.T) {
	client := newTestClient(t, sessionHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apirest.php/Computer/3" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":3,"name":"PC-001","serial":"SN123","otherserial":"INV-1","manufacturers_id":{"id":1,"name":"Dell"},"models_id":{"id":2,"name":"OptiPlex"},"locations_id":{"id":5,"name":"Room 101"},"users_id":{"id":9,"name":"Alice"},"users_id_tech":{"id":10,"name":"Bob"},"states_id":{"id":1,"name":"In use"},"warranty_date":"2025-01-01","notes":"<p>note</p>"}`))
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	asset, err := client.GetAsset(ctx, "Computer", 3)
	if err != nil {
		t.Fatalf("GetAsset returned error: %v", err)
	}
	if asset.Manufacturer != "Dell" || asset.Model != "OptiPlex" {
		t.Fatalf("unexpected asset dropdowns: %+v", asset)
	}
	if asset.Location != "Room 101" || asset.User != "Alice" {
		t.Fatalf("unexpected asset relations: %+v", asset)
	}
	if asset.Serial != "SN123" {
		t.Fatalf("unexpected serial %q", asset.Serial)
	}
}

func TestSearchTicketsAppliesStatusAndPriorityCriteria(t *testing.T) {
	client := newTestClient(t, sessionHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apirest.php/search/Ticket" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		q := r.URL.Query()
		if got := q.Get("criteria[1][field]"); got != "12" {
			t.Fatalf("expected status criterion at [1], got field %q", got)
		}
		if got := q.Get("criteria[1][value]"); got != "5" {
			t.Fatalf("expected status value 5, got %q", got)
		}
		if got := q.Get("criteria[2][field]"); got != "3" {
			t.Fatalf("expected priority criterion at [2], got field %q", got)
		}
		if got := q.Get("criteria[2][value]"); got != "4" {
			t.Fatalf("expected priority morethan 4, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totalcount":1,"count":1,"data":[{"2":77,"1":"Disk issue","12":5,"3":5,"15":"2024-01-01 10:00:00"}]}`))
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tickets, total, err := client.SearchTickets(ctx, TicketFilter{
		RequesterID:     10,
		Status:          5,
		PriorityAtLeast: 5,
		Limit:           15,
	})
	if err != nil {
		t.Fatalf("SearchTickets returned error: %v", err)
	}
	if total != 1 || len(tickets) != 1 || tickets[0].ID != 77 {
		t.Fatalf("unexpected tickets: %+v total=%d", tickets, total)
	}
	if tickets[0].Status != 5 {
		t.Fatalf("unexpected status %d", tickets[0].Status)
	}
}

func TestListTicketDocumentsListsDocuments(t *testing.T) {
	client := newTestClient(t, sessionHandler(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/apirest.php/Ticket/7/Document_Item":
			_, _ = w.Write([]byte(`[{"documents_id":1},{"documents_id":2}]`))
		case "/apirest.php/Document/1":
			_, _ = w.Write([]byte(`{"id":1,"name":"spec.pdf","filename":"spec.pdf","mime":"application/pdf","file_size":1234}`))
		case "/apirest.php/Document/2":
			_, _ = w.Write([]byte(`{"id":2,"name":"photo.png","filename":"photo.png","mime":"image/png","file_size":5678}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	docs, err := client.ListTicketDocuments(ctx, 7)
	if err != nil {
		t.Fatalf("ListTicketDocuments returned error: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(docs))
	}
	if docs[0].Name != "spec.pdf" || docs[0].MimeType != "application/pdf" {
		t.Fatalf("unexpected document: %+v", docs[0])
	}
	if docs[1].Size != 5678 {
		t.Fatalf("unexpected document size: %+v", docs[1])
	}
}

func TestCreateTicketIncludesAssetLinkage(t *testing.T) {
	var received map[string]interface{}
	client := newTestClient(t, sessionHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apirest.php/Ticket" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewDecoder(r.Body).Decode(&received)
		_, _ = w.Write([]byte(`{"id":55}`))
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := client.CreateTicket(ctx, CreateTicketRequest{
		Name:      "Monitor flickers",
		Content:   "Screen flickering every 5 minutes",
		AssetID:   42,
		AssetType: "Monitor",
	})
	if err != nil || result.ID != 55 {
		t.Fatalf("CreateTicket: id=%d err=%v", result.ID, err)
	}

	input, ok := received["input"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected input map in request body, got %v", received)
	}

	if got := fmt.Sprintf("%v", input["_items_id"]); got != "42" {
		t.Fatalf("expected _items_id=42, got %v", got)
	}
	if got := fmt.Sprintf("%v", input["_itemtype"]); got != "Monitor" {
		t.Fatalf("expected _itemtype=Monitor, got %v", got)
	}
}
