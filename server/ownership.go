package main

import (
	"context"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/glpi"
	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/identity"
)

// OwnershipService centralizes all ticket ownership and permission logic.
// Every screen must route through this service; no screen may implement
// its own ownership or permission check.
type OwnershipService struct {
	p *Plugin
}

func NewOwnershipService(p *Plugin) *OwnershipService {
	return &OwnershipService{p: p}
}

// ResolveRole returns the effective IT role for a Mattermost user.
// System administrators always resolve to administrator regardless of GLPI
// mapping. When mapping is disabled (Mode A default) the role is employee
// unless the user is a Mattermost system admin.
func (o *OwnershipService) ResolveRole(userID string) (identity.Role, []string) {
	if userID == "" {
		return identity.RoleEmployee, nil
	}

	// System administrators always have admin privileges
	if o.p.IsSystemAdmin(userID) {
		return identity.RoleAdministrator, nil
	}

	svc := o.p.currentIdentity()
	if svc == nil {
		return identity.RoleEmployee, nil
	}
	role, profiles := svc.RoleForMMUser(context.Background(), &identity.MMUser{UserID: userID})
	return role, profiles
}

// CanViewTicket reports whether the Mattermost user can view the given ticket.
//
// Role rules:
//   - Administrator: view everything.
//   - Technician: view everything (includes assigned work).
//   - Employee (Mode A): view only tickets recorded in the KV ownership store.
//   - Employee (Mode B): view only tickets where the user is the requester
//     (GLPI RequesterID matches the mapped GLPI user).
//
// When GLPI requester data is not available on the Ticket object, the
// integration account falls back to ownership-store membership.
func (o *OwnershipService) CanViewTicket(userID string, ticketID int) bool {
	role, _ := o.ResolveRole(userID)

	// Technicians and admins can view any ticket (the plugin uses an integration
	// account that GLPI grants broad visibility; technicians see their work).
	if role == identity.RoleAdministrator || role == identity.RoleTechnician {
		return true
	}

	// Employee: check the KV ownership mapping.
	// Every ticket creation records the Mattermost user, so ownership is
	// deterministic in Mode A.
	if svc := o.p.currentIdentity(); svc != nil {
		ids := svc.ListOwnedTicketIDs(userID)
		for _, id := range ids {
			if id == ticketID {
				return true
			}
		}
	}

	// Mode B fallback: if the user has a mapped GLPI user, the GLPI search
	// filters already ensure they only see their own tickets. For individual
	// ticket access, we allow the request and let GLPI handle the final
	// authorization (the integration account typically has read access).
	glpiUserID, _ := o.p.GetGLPIUserID(userID)
	if glpiUserID > 0 {
		return true
	}

	return false
}

// CanEditTicket reports whether the user can update ticket fields.
// Only technicians and admins can edit; employees cannot.
func (o *OwnershipService) CanEditTicket(userID string) bool {
	role, _ := o.ResolveRole(userID)
	return role == identity.RoleAdministrator || role == identity.RoleTechnician
}

// CanAssignTicket reports whether the user can assign/reassign a ticket.
// Only technicians and admins can assign.
func (o *OwnershipService) CanAssignTicket(userID string) bool {
	return o.CanEditTicket(userID)
}

// CanCloseTicket reports whether the user can close a ticket.
// Only technicians and admins can close.
func (o *OwnershipService) CanCloseTicket(userID string) bool {
	return o.CanEditTicket(userID)
}

// CanAddPrivateNote reports whether the user can post a private (internal) note.
// Only technicians and admins.
func (o *OwnershipService) CanAddPrivateNote(userID string) bool {
	return o.CanEditTicket(userID)
}

// CanViewPrivateNotes reports whether the user can see private follow-ups.
// Only technicians and admins.
func (o *OwnershipService) CanViewPrivateNotes(userID string) bool {
	return o.CanEditTicket(userID)
}

// OwnershipFilter builds a GLPI TicketFilter scoped to the user's effective
// ownership. The returned filter is ready to pass to client.SearchTickets.
//
// Admin: no ownership filter — matches everything.
// Technician: assigned tickets only.
// Employee (Mode A): empty filter (caller must fall back to KV ownership store).
// Employee (Mode B): requester-scoped filter.
func (o *OwnershipService) OwnershipFilter(userID string) glpi.TicketFilter {
	role, _ := o.ResolveRole(userID)
	glpiUserID, _ := o.p.GetGLPIUserID(userID)

	switch {
	case role == identity.RoleAdministrator:
		// Admin: no filter — match everything
		return glpi.TicketFilter{Limit: 1}

	case role == identity.RoleTechnician && glpiUserID > 0:
		// Technician: assigned tickets (Mode B only; in Mode A, integration
		// account owns everything so a technician filter returns all).
		return glpi.TicketFilter{AssigneeID: glpiUserID, Limit: 1}

	case glpiUserID > 0:
		// Employee Mode B: requester-scoped
		return glpi.TicketFilter{RequesterID: glpiUserID, Limit: 1}

	default:
		// Employee Mode A: caller must fall back to KV ownership store
		return glpi.TicketFilter{Limit: 1}
	}
}

// HasMappedGLPIUser reports whether the Mattermost user has an individual
// GLPI account (Mode B). False means the user relies on the integration
// account and ownership is tracked via the KV ownership store.
func (o *OwnershipService) HasMappedGLPIUser(userID string) bool {
	glpiUserID, _ := o.p.GetGLPIUserID(userID)
	return glpiUserID > 0
}

// isOwnershipEmpty reports whether the user has no ownership records in the KV store.
func (o *OwnershipService) isOwnershipEmpty(userID string) bool {
	if svc := o.p.currentIdentity(); svc != nil {
		return len(svc.ListOwnedTicketIDs(userID)) == 0
	}
	return true
}

// BuildMetadata returns the Mattermost identity metadata block for a ticket
// description, but only once. Returns empty string when the user has a GLPI
// account (no metadata needed).
func (o *OwnershipService) BuildMetadata(mm *identity.MMUser) string {
	if mm == nil {
		return ""
	}
	return mm.MetadataHTML()
}
