package identity

import (
	"context"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/glpi"
	"github.com/mattermost/mattermost/server/public/model"
)

// Mode identifies how a Mattermost user's GLPI requester identity was resolved.
type Mode string

const (
	ModeIntegration Mode = "integration" // GLPI integration account owns the request
	ModeMapped      Mode = "mapped"      // mapped to an individual GLPI user
)

// Role is the effective IT role derived from a GLPI user's profiles.
type Role string

const (
	RoleEmployee      Role = "employee"
	RoleTechnician    Role = "technician"
	RoleSupervisor    Role = "supervisor"
	RoleManager       Role = "manager"
	RoleAdministrator Role = "administrator"
)

// Mapping is the permanent Mattermost ↔ GLPI user mapping stored in KV.
type Mapping struct {
	MMUserID      string   `json:"mm_user_id"`
	MMUsername    string   `json:"mm_username"`
	MMEmail       string   `json:"mm_email"`
	MMDisplayName string   `json:"mm_display_name"`
	GLPIUserID    int      `json:"glpi_user_id"`
	GLPILogin     string   `json:"glpi_login"`
	GLPIFullName  string   `json:"glpi_full_name"`
	GLPIMail      string   `json:"glpi_email"`
	Profiles      []string `json:"profiles"`
	Role          string   `json:"role"`
	SyncStatus    string   `json:"sync_status"` // mapped | unmapped | duplicate
	LastSync      int64    `json:"last_sync"`
}

// MMUser is the Mattermost identity captured on every ticket.
type MMUser struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	TeamID      string `json:"team_id,omitempty"`
	Team        string `json:"team,omitempty"`
	ChannelID   string `json:"channel_id,omitempty"`
	Channel     string `json:"channel,omitempty"`
}

// Requester is the resolved GLPI requester identity for a Mattermost user.
type Requester struct {
	Mode       Mode
	GLPIUserID int // 0 => integration account
	User       MMUser
}

// KVStore is the subset of the Mattermost plugin KV API used by the service.
type KVStore interface {
	KVGet(key string) ([]byte, *model.AppError)
	KVSet(key string, value []byte) *model.AppError
	KVSetWithExpiry(key string, value []byte, expireInSeconds int64) *model.AppError
	KVDelete(key string) *model.AppError
}

// UserStore provides read/write access to GLPI users for mapping resolution
// and admin provisioning. It is satisfied by the glpi.GLPIClient interface
// (once the plugin has connected).
type UserStore interface {
	FindUserByEmail(ctx context.Context, email string) (*glpi.UserSummary, error)
	FindUserByLogin(ctx context.Context, login string) (*glpi.UserSummary, error)
	FindUserByName(ctx context.Context, firstname, lastname string) (*glpi.UserSummary, error)
	GetUserProfiles(ctx context.Context, userID int) ([]string, error)
	ListUsers(ctx context.Context, limit, page int) ([]glpi.UserSummary, int, error)
	CreateUser(ctx context.Context, req glpi.CreateUserRequest) (int, error)
}

// FetchTicket retrieves a single GLPI ticket; injected by the caller.
type FetchTicket func(ctx context.Context, id int) (*glpi.Ticket, error)
