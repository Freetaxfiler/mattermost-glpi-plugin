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

func (k *stubKV) KVGet(key string) ([]byte, *model.AppError)     { return k.data[key], nil }
func (k *stubKV) KVSet(key string, value []byte) *model.AppError { k.data[key] = value; return nil }
func (k *stubKV) KVSetWithExpiry(key string, value []byte, ttl int64) *model.AppError {
	k.data[key] = value
	k.ttl[key] = ttl
	return nil
}
func (k *stubKV) KVDelete(key string) *model.AppError {
	delete(k.data, key)
	return nil
}

// stubUsers implements UserStore in memory.
type stubUsers struct {
	byEmail  map[string]*glpi.UserSummary
	byLogin  map[string]*glpi.UserSummary
	byName   map[string]*glpi.UserSummary
	profiles map[int][]string
	created  []glpi.CreateUserRequest
}

func newStubUsers() *stubUsers {
	return &stubUsers{
		byEmail:  map[string]*glpi.UserSummary{},
		byLogin:  map[string]*glpi.UserSummary{},
		byName:   map[string]*glpi.UserSummary{},
		profiles: map[int][]string{},
	}
}

func (u *stubUsers) FindUserByEmail(_ context.Context, email string) (*glpi.UserSummary, error) {
	if s, ok := u.byEmail[email]; ok {
		return s, nil
	}
	return nil, &glpi.NotFoundError{Message: "no user for email " + email}
}
func (u *stubUsers) FindUserByLogin(_ context.Context, login string) (*glpi.UserSummary, error) {
	if s, ok := u.byLogin[login]; ok {
		return s, nil
	}
	return nil, &glpi.NotFoundError{Message: "no user for login " + login}
}
func (u *stubUsers) FindUserByName(_ context.Context, first, last string) (*glpi.UserSummary, error) {
	if s, ok := u.byName[first+"|"+last]; ok {
		return s, nil
	}
	return nil, &glpi.NotFoundError{Message: "no user for name"}
}
func (u *stubUsers) GetUserProfiles(_ context.Context, id int) ([]string, error) {
	return u.profiles[id], nil
}
func (u *stubUsers) ListUsers(context.Context, int, int) ([]glpi.UserSummary, int, error) {
	return nil, 0, nil
}
func (u *stubUsers) CreateUser(_ context.Context, req glpi.CreateUserRequest) (int, error) {
	u.created = append(u.created, req)
	return 900 + len(u.created), nil
}

func TestResolveRequesterModeAAlwaysIntegration(t *testing.T) {
	svc := New(newStubKV(), nil, false)
	r := svc.ResolveRequester(context.Background(), &MMUser{UserID: "u1", Email: "a@b.c"})
	if r.Mode != ModeIntegration || r.GLPIUserID != 0 {
		t.Fatalf("expected integration, got %+v", r)
	}
}

func TestResolveRequesterModeBDiscoverAndMap(t *testing.T) {
	users := newStubUsers()
	users.byEmail["a@b.c"] = &glpi.UserSummary{ID: 42, Login: "bob", Email: "a@b.c"}
	svc := New(newStubKV(), users, true)
	r := svc.ResolveRequester(context.Background(), &MMUser{UserID: "u1", Username: "bob", Email: "a@b.c"})
	if r.Mode != ModeMapped || r.GLPIUserID != 42 {
		t.Fatalf("expected mapped 42, got %+v", r)
	}
	// Second call must hit the permanent mapping without a second discovery.
	users.byEmail["a@b.c"] = nil
	r2 := svc.ResolveRequester(context.Background(), &MMUser{UserID: "u1", Username: "bob", Email: "a@b.c"})
	if r2.Mode != ModeMapped || r2.GLPIUserID != 42 {
		t.Fatalf("expected cached mapped 42, got %+v", r2)
	}
}

func TestResolveRequesterModeBFallsBackOnMissing(t *testing.T) {
	users := newStubUsers()
	svc := New(newStubKV(), users, true)
	r := svc.ResolveRequester(context.Background(), &MMUser{UserID: "u1", Username: "ghost", Email: "nobody@b.c"})
	if r.Mode != ModeIntegration || r.GLPIUserID != 0 {
		t.Fatalf("expected integration fallback, got %+v", r)
	}
}

func TestResolveRequesterNilUserReturnsIntegration(t *testing.T) {
	svc := New(newStubKV(), newStubUsers(), true)
	r := svc.ResolveRequester(context.Background(), nil)
	if r.Mode != ModeIntegration || r.GLPIUserID != 0 {
		t.Fatalf("expected integration, got %+v", r)
	}
}

func TestMappingStoreRoundTripAndIndexes(t *testing.T) {
	svc := New(newStubKV(), nil, false)
	m := &Mapping{
		MMUserID: "u1", MMUsername: "bob", MMEmail: "b@b.c", MMDisplayName: "Bob",
		GLPIUserID: 7, GLPILogin: "bob", GLPIMail: "b@b.c", Role: string(RoleEmployee), SyncStatus: "mapped", LastSync: 1,
	}
	if err := svc.SaveMapping(m); err != nil {
		t.Fatalf("SaveMapping: %v", err)
	}
	if got, err := svc.GetMappingByMMID("u1"); err != nil || got.GLPIUserID != 7 {
		t.Fatalf("by MMID: %v %v", got, err)
	}
	if got, err := svc.GetMappingByEmail("b@b.c"); err != nil || got.MMUserID != "u1" {
		t.Fatalf("by email: %v %v", got, err)
	}
	if got, err := svc.GetMappingByGLPIID(7); err != nil || got.MMUserID != "u1" {
		t.Fatalf("by glpi id: %v %v", got, err)
	}
	if got, err := svc.GetMappingByLogin("bob"); err != nil || got.MMUserID != "u1" {
		t.Fatalf("by login: %v %v", got, err)
	}
	if all, err := svc.AllMappings(); err != nil || len(all) != 1 {
		t.Fatalf("AllMappings: %v %v", all, err)
	}
	if err := svc.RemoveMapping("u1"); err != nil {
		t.Fatalf("RemoveMapping: %v", err)
	}
	if _, err := svc.GetMappingByMMID("u1"); err != ErrMappingNotFound {
		t.Fatalf("expected ErrMappingNotFound after removal, got %v", err)
	}
}

func TestDiscoveryPriorityEmailOverLogin(t *testing.T) {
	users := newStubUsers()
	users.byEmail["a@b.c"] = &glpi.UserSummary{ID: 42, Login: "bob", Email: "a@b.c"}
	users.byLogin["alice"] = &glpi.UserSummary{ID: 99, Login: "alice", Email: "x@y.z"}
	svc := New(newStubKV(), users, true)
	m, err := svc.DiscoverAndMap(context.Background(), &MMUser{UserID: "u1", Username: "alice", Email: "a@b.c"})
	if err != nil || m.GLPIUserID != 42 {
		t.Fatalf("email priority failed: %v %v", m, err)
	}
}

func TestRoleMapping(t *testing.T) {
	cases := map[string]Role{
		"Self-Service":    RoleEmployee,
		"Technician":      RoleTechnician,
		"Hotliner":        RoleTechnician,
		"Manager":         RoleManager,
		"Supervisor":      RoleSupervisor,
		"Super-Admin":     RoleAdministrator,
		"Admin":           RoleAdministrator,
		"Unknown Profile": RoleEmployee,
		"":                RoleEmployee,
	}
	for name, want := range cases {
		if got := MapProfileName(name); got != want {
			t.Errorf("MapProfileName(%q) = %s, want %s", name, got, want)
		}
	}
	if got := HighestRole([]string{"Technician", "Manager", "Super-Admin"}); got != RoleAdministrator {
		t.Errorf("HighestRole = %s, want administrator", got)
	}
}

func TestResolveRoleFromProfiles(t *testing.T) {
	users := newStubUsers()
	users.profiles[7] = []string{"Technician"}
	svc := New(newStubKV(), users, true)
	role, profiles, err := svc.ResolveRole(context.Background(), 7)
	if err != nil || role != RoleTechnician || len(profiles) != 1 {
		t.Fatalf("ResolveRole = %s %v %v", role, profiles, err)
	}
}

func TestProvisionUserNeverDuplicates(t *testing.T) {
	users := newStubUsers()
	svc := New(newStubKV(), users, true)
	mm := &MMUser{UserID: "u1", Username: "jane doe", DisplayName: "Jane Doe", Email: "jane@x.c"}
	m1, err := svc.ProvisionUser(context.Background(), mm, 5, 0)
	if err != nil || m1.GLPIUserID == 0 {
		t.Fatalf("ProvisionUser: %v %v", m1, err)
	}
	if len(users.created) != 1 {
		t.Fatalf("expected 1 created user, got %d", len(users.created))
	}
	m2, err := svc.ProvisionUser(context.Background(), mm, 5, 0)
	if err != nil || m2.GLPIUserID != m1.GLPIUserID {
		t.Fatalf("duplicate provision: %v %v", m2, err)
	}
	if len(users.created) != 1 {
		t.Fatalf("second provision must not create another user, got %d", len(users.created))
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
