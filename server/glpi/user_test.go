package glpi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestFindUserByEmailParsesRow(t *testing.T) {
	client := newTestClient(t, sessionHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apirest.php/search/User" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totalcount":1,"count":1,"data":[{"2":42,"1":"bob","3":"Smith","4":"Bob","5":"bob@x.c"}]}`))
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	u, err := client.FindUserByEmail(ctx, "bob@x.c")
	if err != nil {
		t.Fatalf("FindUserByEmail: %v", err)
	}
	if u.ID != 42 || u.Login != "bob" || u.Email != "bob@x.c" || u.Realname != "Smith" || u.Firstname != "Bob" {
		t.Fatalf("unexpected user: %+v", u)
	}
}

func TestFindUserByLoginNotFound(t *testing.T) {
	client := newTestClient(t, sessionHandler(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totalcount":0,"count":0,"data":[]}`))
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.FindUserByLogin(ctx, "ghost"); err == nil {
		t.Fatal("expected NotFoundError for unknown login")
	} else {
		var nf *NotFoundError
		if !errors.As(err, &nf) {
			t.Fatalf("expected NotFoundError, got %T", err)
		}
	}
}

func TestFindUserByNameUsesRealnameAndFirstname(t *testing.T) {
	client := newTestClient(t, sessionHandler(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totalcount":1,"count":1,"data":[{"2":9,"1":"jane","3":"Doe","4":"Jane","5":"j@x.c"}]}`))
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	u, err := client.FindUserByName(ctx, "Jane", "Doe")
	if err != nil || u.ID != 9 {
		t.Fatalf("FindUserByName: %+v %v", u, err)
	}
}

func TestGetUserProfilesExpandedProfilesID(t *testing.T) {
	client := newTestClient(t, sessionHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apirest.php/User/42" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("expand_dropdowns") != "true" {
			t.Fatalf("expected expand_dropdowns=true")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42,"name":"bob","_profiles_id":{"id":5,"name":"Technician"}}`))
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	profiles, err := client.GetUserProfiles(ctx, 42)
	if err != nil {
		t.Fatalf("GetUserProfiles: %v", err)
	}
	if len(profiles) != 1 || profiles[0] != "Technician" {
		t.Fatalf("unexpected profiles %v", profiles)
	}
}

func TestGetUserProfilesNumericIDFallback(t *testing.T) {
	client := newTestClient(t, sessionHandler(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42,"_profiles_id":5}`))
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	profiles, err := client.GetUserProfiles(ctx, 42)
	if err != nil {
		t.Fatalf("GetUserProfiles: %v", err)
	}
	if len(profiles) != 1 || profiles[0] != "Technician" { // profile id 5 → Technician
		t.Fatalf("unexpected profiles %v", profiles)
	}
}

func TestListUsersParsesRows(t *testing.T) {
	client := newTestClient(t, sessionHandler(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totalcount":2,"count":2,"data":[{"2":1,"1":"a","3":"A","4":"","5":""},{"2":2,"1":"b","3":"B","4":"Bee","5":"b@x"}]}`))
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	users, total, err := client.ListUsers(ctx, 10, 1)
	if err != nil || total != 2 || len(users) != 2 {
		t.Fatalf("ListUsers: %d/%d %v", len(users), total, err)
	}
}

func TestCreateUserPostsInputAndParsesID(t *testing.T) {
	var gotBody map[string]interface{}
	client := newTestClient(t, sessionHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/apirest.php/User" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":123}`))
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id, err := client.CreateUser(ctx, CreateUserRequest{
		Login: "jdoe", Firstname: "Jane", Realname: "Doe", Email: "j@x.c",
		ProfileID: 5, EntityID: 0, Recursive: 1, Active: 1,
	})
	if err != nil || id != 123 {
		t.Fatalf("CreateUser: id=%d err=%v", id, err)
	}
	input, ok := gotBody["input"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected wrapped input, got %v", gotBody)
	}
	if input["name"] != "jdoe" || input["email"] != "j@x.c" {
		t.Fatalf("unexpected input %v", input)
	}
}
