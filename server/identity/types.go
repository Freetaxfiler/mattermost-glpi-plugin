package identity

import (
	"context"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/glpi"
	"github.com/mattermost/mattermost/server/public/model"
)

// Mode identifies how a Mattermost user's GLPI requester identity was resolved.
type Mode string

const (
	// ModeIntegration is the default: the GLPI integration account owns the
	// request and the Mattermost identity is preserved as ticket metadata.
	ModeIntegration Mode = "integration"
	// ModeMapped is the optional per-user mapping: the Mattermost user was
	// matched to a GLPI user by email.
	ModeMapped Mode = "mapped"
)

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
	GLPIUserID int // 0 => the integration account is the requester
	User       MMUser
}

// KVStore is the subset of the Mattermost plugin KV API used by the service.
type KVStore interface {
	KVGet(key string) ([]byte, *model.AppError)
	KVSet(key string, value []byte) *model.AppError
	KVSetWithExpiry(key string, value []byte, expireInSeconds int64) *model.AppError
}

// UserLookup finds a GLPI user id by email (Mode B). The default
// implementation delegates to the GLPI client; tests inject a stub.
type UserLookup interface {
	FindUserIDByEmail(ctx context.Context, email string) (int, error)
}

// FetchTicket retrieves a single GLPI ticket; injected by the caller so the
// identity service stays decoupled from the full GLPI client.
type FetchTicket func(ctx context.Context, id int) (*glpi.Ticket, error)
