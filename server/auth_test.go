package main

import (
	"net/http/httptest"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
)

func TestAuthenticateResolvesServerInjectedHeader(t *testing.T) {
	api := &plugintest.API{}
	p := &Plugin{}
	p.API = api

	api.On("GetUser", "user1").
		Return(&model.User{Id: "user1", Username: "alice", Email: "alice@example.com", Roles: "system_user"}, nil).Once()
	api.On("HasPermissionTo", "user1", model.PermissionManageSystem).Return(false, false).Once()

	r := httptest.NewRequest("GET", "/api/v1/user", nil)
	r.Header.Set("Mattermost-User-Id", "user1")

	cu, err := p.authenticate(r)
	if err != nil {
		t.Fatalf("authenticate returned error: %v", err)
	}
	if cu == nil {
		t.Fatal("authenticate returned nil user")
	}
	if cu.UserID != "user1" || cu.Username != "alice" || cu.Email != "alice@example.com" {
		t.Fatalf("unexpected user context: %+v", cu)
	}
	if cu.IsSystemAdmin {
		t.Fatal("expected IsSystemAdmin=false for system_user roles")
	}
	api.AssertExpectations(t)
}

func TestAuthenticateRejectsMissingHeader(t *testing.T) {
	api := &plugintest.API{}
	p := &Plugin{}
	p.API = api

	cu, err := p.authenticate(httptest.NewRequest("GET", "/api/v1/user", nil))
	if err == nil {
		t.Fatal("expected error for missing Mattermost-User-Id header")
	}
	if cu != nil {
		t.Fatalf("expected nil user, got %+v", cu)
	}
	api.AssertExpectations(t)
}

func TestAuthenticateRejectsUnknownUser(t *testing.T) {
	api := &plugintest.API{}
	p := &Plugin{}
	p.API = api

	api.On("GetUser", "ghost").
		Return(nil, model.NewAppError("GetUser", "app.user.get.app_error", nil, "no user", httpStatusInternal)).Once()

	r := httptest.NewRequest("GET", "/api/v1/user", nil)
	r.Header.Set("Mattermost-User-Id", "ghost")

	if _, err := p.authenticate(r); err == nil {
		t.Fatal("expected error for unknown user")
	}
	api.AssertExpectations(t)
}

func TestAuthenticateRejectsBot(t *testing.T) {
	api := &plugintest.API{}
	p := &Plugin{}
	p.API = api

	api.On("GetUser", "bot1").
		Return(&model.User{Id: "bot1", Username: "glpi", IsBot: true}, nil).Once()

	r := httptest.NewRequest("GET", "/api/v1/user", nil)
	r.Header.Set("Mattermost-User-Id", "bot1")

	if _, err := p.authenticate(r); err == nil {
		t.Fatal("expected error for bot session")
	}
	api.AssertExpectations(t)
}

func TestAuthenticateFlagsSystemAdmin(t *testing.T) {
	api := &plugintest.API{}
	p := &Plugin{}
	p.API = api

	api.On("GetUser", "admin1").
		Return(&model.User{Id: "admin1", Username: "root", Email: "root@example.com", Roles: "system_user system_admin"}, nil).Once()
	api.On("HasPermissionTo", "admin1", model.PermissionManageSystem).Return(true, false).Once()

	r := httptest.NewRequest("GET", "/api/v1/user", nil)
	r.Header.Set("Mattermost-User-Id", "admin1")

	cu, err := p.authenticate(r)
	if err != nil {
		t.Fatalf("authenticate returned error: %v", err)
	}
	if !cu.IsSystemAdmin {
		t.Fatal("expected IsSystemAdmin=true for system_admin roles")
	}
	api.AssertExpectations(t)
}

// httpStatusInternal is a placeholder status used only by the mock AppError.
const httpStatusInternal = 500
