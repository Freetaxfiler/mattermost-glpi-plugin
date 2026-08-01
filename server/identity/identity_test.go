package identity

import (
	"context"
	"strings"
	"testing"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/glpi"
	"github.com/mattermost/mattermost/server/public/model"
)

// stubKV implements KVStore in memory.
type stubKV struct {
	data map[string][]byte
	ttl  map[string]int64
}

func newStubKV() *stubKV { return &stubKV{data: map[string][]byte{}, ttl: map[string]int64{}} }

func (k *stubKV) KVGet(key string) ([]byte, *model.AppError)         { return k.data[key], nil }
func (k *stubKV) KVSet(key string, value []byte) *model.AppError      { k.data[key] = value; return nil }
func (k *stubKV) KVSetWithExpiry(key string, value []byte, ttl int64) *model.AppError {
	k.data[key] = value
	k.ttl[key] = ttl
	return nil
}

// stubLookup implements UserLookup.
type stubLookup struct {
	ids map[string]int
}

func (l *stubLookup) FindUserIDByEmail(_ context.Context, email string) (int, error) {
	return l.ids[email], nil
}

func TestResolveRequesterModeAAlwaysIntegration(t *testing.T) {
	svc := New(newStubKV(), nil, false)
	r := svc.ResolveRequester(context.Background(), &MMUser{Email: "a@b.c"})
	if r.Mode != ModeIntegration || r.GLPIUserID != 0 {
		t.Fatalf("expected integration, got %+v", r)
	}
}

func TestResolveRequesterModeBMapsByEmail(t *testing.T) {
	svc := New(newStubKV(), &stubLookup{ids: map[string]int{"a@b.c": 42}}, true)
	r := svc.ResolveRequester(context.Background(), &MMUser{Email: "a@b.c"})
	if r.Mode != ModeMapped || r.GLPIUserID != 42 {
		t.Fatalf("expected mapped 42, got %+v", r)
	}
}

func TestResolveRequesterModeBFallsBackOnMissing(t *testing.T) {
	svc := New(newStubKV(), &stubLookup{ids: map[string]int{}}, true)
	r := svc.ResolveRequester(context.Background(), &MMUser{Email: "nobody@b.c"})
	if r.Mode != ModeIntegration || r.GLPIUserID != 0 {
		t.Fatalf("expected integration fallback, got %+v", r)
	}
}

func TestResolveRequesterNilUserReturnsIntegration(t *testing.T) {
	svc := New(newStubKV(), &stubLookup{ids: map[string]int{"a@b.c": 42}}, true)
	r := svc.ResolveRequester(context.Background(), nil)
	if r.Mode != ModeIntegration || r.GLPIUserID != 0 {
		t.Fatalf("expected integration, got %+v", r)
	}
}

func TestRecordAndListOwnedTickets(t *testing.T) {
	svc := New(newStubKV(), nil, false)
	svc.RecordTicketOwnership("u1", 1)
	svc.RecordTicketOwnership("u1", 2)
	svc.RecordTicketOwnership("u1", 3)
	ids := svc.ListOwnedTicketIDs("u1")
	if len(ids) != 3 || ids[0] != 3 || ids[2] != 1 {
		t.Fatalf("unexpected owned ids (want newest first): %v", ids)
	}
	if got := svc.ListOwnedTicketIDs("u2"); len(got) != 0 {
		t.Fatalf("expected empty for u2, got %v", got)
	}
}

func TestListOwnedTicketsFetchesAndPaginates(t *testing.T) {
	svc := New(newStubKV(), nil, false)
	for _, id := range []int{1, 2, 3, 4, 5} {
		svc.RecordTicketOwnership("u1", id)
	}
	fetch := func(_ context.Context, id int) (*glpi.Ticket, error) {
		return &glpi.Ticket{ID: id, Name: "t", Status: 1, Priority: 3, Date: "d"}, nil
	}
	list, total, err := svc.ListOwnedTickets(context.Background(), "u1", 1, 2, fetch)
	if err != nil {
		t.Fatalf("ListOwnedTickets returned error: %v", err)
	}
	if total != 5 || len(list) != 2 {
		t.Fatalf("expected total 5 page of 2, got total=%d len=%d", total, len(list))
	}
	if list[0].ID != 5 { // newest first
		t.Fatalf("expected newest id 5 first, got %d", list[0].ID)
	}
}

func TestMetadataHTMLEscapesValues(t *testing.T) {
	m := &MMUser{UserID: "<script>", Username: "bob", DisplayName: "Bob", Email: "b@b.c"}
	out := m.MetadataHTML()
	if out == "" {
		t.Fatal("expected metadata output")
	}
	if strings.Contains(out, "<script>") {
		t.Fatalf("metadata must escape HTML, got %q", out)
	}
	if !strings.Contains(out, "Mattermost Metadata") {
		t.Fatalf("expected marker, got %q", out)
	}
}
