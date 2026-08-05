package main

import (
	"testing"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/identity"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/mock"
)

// newOwnershipPlugin builds a Plugin whose API mock answers HasPermissionTo
// with isAdmin, so IsSystemAdmin and ResolveRole behave deterministically.
func newOwnershipPlugin(isAdmin bool) (*Plugin, *plugintest.API) {
	api := &plugintest.API{}
	p := &Plugin{}
	p.API = api
	p.ownershipSvc = NewOwnershipService(p)
	api.On("HasPermissionTo", mock.Anything, model.PermissionManageSystem).Return(isAdmin)
	return p, api
}

func TestOwnershipService_SystemAdminCanViewEverything(t *testing.T) {
	p, api := newOwnershipPlugin(true)

	if !p.ownershipSvc.CanViewTicket("admin", 999) {
		t.Fatal("system admin should be able to view any ticket")
	}
	if !p.ownershipSvc.CanEditTicket("admin") {
		t.Fatal("system admin should be able to edit tickets")
	}
	if !p.ownershipSvc.CanAssignTicket("admin") {
		t.Fatal("system admin should be able to assign tickets")
	}
	if !p.ownershipSvc.CanAddPrivateNote("admin") {
		t.Fatal("system admin should be able to add private notes")
	}
	api.AssertExpectations(t)
}

func TestOwnershipService_EmployeeCannotEdit(t *testing.T) {
	p, api := newOwnershipPlugin(false)

	if p.ownershipSvc.CanEditTicket("emp1") {
		t.Fatal("employee should not be able to edit tickets")
	}
	if p.ownershipSvc.CanAssignTicket("emp1") {
		t.Fatal("employee should not be able to assign tickets")
	}
	if p.ownershipSvc.CanCloseTicket("emp1") {
		t.Fatal("employee should not be able to close tickets")
	}
	if p.ownershipSvc.CanAddPrivateNote("emp1") {
		t.Fatal("employee should not be able to add private notes")
	}
	api.AssertExpectations(t)
}

func TestOwnershipService_EmployeeCannotViewUnowned(t *testing.T) {
	p, api := newOwnershipPlugin(false)

	// No identity store → no ownership records → employee cannot view.
	if p.ownershipSvc.CanViewTicket("emp1", 123) {
		t.Fatal("employee with no ownership record should not view ticket 123")
	}
	api.AssertExpectations(t)
}

func TestOwnershipService_ResolveRole_AdminAlwaysAdmin(t *testing.T) {
	p, api := newOwnershipPlugin(true)

	role, _ := p.ownershipSvc.ResolveRole("admin")
	if role != identity.RoleAdministrator {
		t.Fatalf("expected administrator role, got %s", role)
	}
	api.AssertExpectations(t)
}

func TestOwnershipService_ResolveRole_UnmappedEmployee(t *testing.T) {
	p, api := newOwnershipPlugin(false)

	role, _ := p.ownershipSvc.ResolveRole("emp1")
	if role != identity.RoleEmployee {
		t.Fatalf("expected employee role, got %s", role)
	}
	api.AssertExpectations(t)
}

func TestOwnershipService_HasMappedGLPIUser(t *testing.T) {
	p, _ := newOwnershipPlugin(false)

	// With no identity service, GetGLPIUserID returns 0 → no GLPI account.
	if p.ownershipSvc.HasMappedGLPIUser("emp1") {
		t.Fatal("unmapped user should not have GLPI account")
	}
}
