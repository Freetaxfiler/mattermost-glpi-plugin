package identity

import (
	"context"
	"strings"
)

// ResolveRole maps a GLPI user's profiles to an effective role. An unmapped or
// unavailable user defaults to employee; errors are returned for diagnostics.
func (s *Service) ResolveRole(ctx context.Context, glpiUserID int) (Role, []string, error) {
	if glpiUserID <= 0 || s.users == nil {
		return RoleEmployee, nil, nil
	}
	profiles, err := s.users.GetUserProfiles(ctx, glpiUserID)
	if err != nil {
		return RoleEmployee, profiles, err
	}
	return HighestRole(profiles), profiles, nil
}

// HighestRole returns the most-privileged role among the given profile names.
func HighestRole(profiles []string) Role {
	best := RoleEmployee
	for _, p := range profiles {
		r := MapProfileName(p)
		if roleRank(r) > roleRank(best) {
			best = r
		}
	}
	return best
}

// MapProfileName maps a GLPI profile name to a plugin role. Unknown profiles
// default to employee so employees never gain privileged capabilities by
// omission. Matching is case-insensitive substring based on the well-known
// seeded GLPI profile names.
func MapProfileName(name string) Role {
	n := strings.ToLower(strings.TrimSpace(name))
	switch {
	case n == "":
		return RoleEmployee
	case strings.Contains(n, "super-admin"), strings.Contains(n, "super admin"),
		n == "admin", strings.Contains(n, "administrator"):
		return RoleAdministrator
	case strings.Contains(n, "manager"):
		return RoleManager
	case strings.Contains(n, "supervisor"):
		return RoleSupervisor
	case strings.Contains(n, "technician"), strings.Contains(n, "hotliner"),
		strings.Contains(n, "read-only"), strings.Contains(n, "readonly"):
		return RoleTechnician
	default:
		return RoleEmployee
	}
}

func roleRank(r Role) int {
	switch r {
	case RoleAdministrator:
		return 5
	case RoleManager:
		return 4
	case RoleSupervisor:
		return 3
	case RoleTechnician:
		return 2
	default:
		return 1
	}
}
