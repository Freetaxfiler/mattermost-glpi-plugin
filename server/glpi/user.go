package glpi

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// User search options (standard GLPI search option IDs). fieldName(1) is the
// login/name, fieldID(2) the id, userFieldEmail(5) the email, and 3/4 are the
// realname and firstname respectively.
const (
	userFieldRealname  = 3
	userFieldFirstname = 4
)

// UserSummary is a compact GLPI user row from the search engine.
type UserSummary struct {
	ID        int    `json:"id"`
	Login     string `json:"name"`
	Realname  string `json:"realname"`
	Firstname string `json:"firstname"`
	Email     string `json:"email"`
}

// CreateUserRequest is the payload for creating a GLPI user account. ProfileID
// and EntityID are sent as the underscored input fields so GLPI links the
// default profile and entity in the same request.
type CreateUserRequest struct {
	Login     string `json:"name"`
	Firstname string `json:"firstname"`
	Realname  string `json:"realname"`
	Email     string `json:"email"`
	Language  string `json:"language,omitempty"`
	Timezone  string `json:"timezone,omitempty"`
	Active    int    `json:"is_active"`

	ProfileID int `json:"_profiles_id,omitempty"`
	EntityID  int `json:"_entities_id,omitempty"`
	Recursive int `json:"_is_recursive,omitempty"`
}

// userFull is the expanded GET /User/{id} payload used for profile discovery.
type userFull struct {
	ID         int         `json:"id"`
	Name       string      `json:"name"`
	Realname   string      `json:"realname"`
	Firstname  string      `json:"firstname"`
	Email      string      `json:"email"`
	ProfilesID interface{} `json:"_profiles_id"`
}

const userForceDisplay = "1,2,3,4,5"

// searchUsers runs a search against /search/User with the given criteria and
// parses compact user rows. limit 0 means the GLPI default.
func (c *Client) searchUsers(ctx context.Context, criteria []searchCriterion, limit, page int) ([]UserSummary, int, error) {
	if limit <= 0 {
		limit = 50
	}
	result, err := c.runSearch(ctx, searchQuery{
		ItemType:     "User",
		Criteria:     criteria,
		ForceDisplay: []int{fieldID, fieldName, userFieldRealname, userFieldFirstname, userFieldEmail},
		Limit:        limit,
		Page:         page,
	})
	if err != nil {
		return nil, 0, err
	}
	users := make([]UserSummary, 0, len(result.Data))
	for _, row := range result.Data {
		users = append(users, UserSummary{
			ID:        asInt(row[strconv.Itoa(fieldID)]),
			Login:     asString(row[strconv.Itoa(fieldName)]),
			Realname:  asString(row[strconv.Itoa(userFieldRealname)]),
			Firstname: asString(row[strconv.Itoa(userFieldFirstname)]),
			Email:     asString(row[strconv.Itoa(userFieldEmail)]),
		})
	}
	return users, result.TotalCount, nil
}

func userByCriteria(ctx context.Context, c *Client, criteria []searchCriterion) (*UserSummary, error) {
	users, _, err := c.searchUsers(ctx, criteria, 1, 1)
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, &NotFoundError{Message: "no GLPI user matches the criteria"}
	}
	return &users[0], nil
}

// FindUserByEmail resolves a GLPI user by exact email match.
func (c *Client) FindUserByEmail(ctx context.Context, email string) (*UserSummary, error) {
	if email == "" {
		return nil, &ConfigError{Message: "email is empty"}
	}
	return userByCriteria(ctx, c, []searchCriterion{
		{Field: strconv.Itoa(userFieldEmail), SearchType: "contains", Value: email},
	})
}

// FindUserByLogin resolves a GLPI user by exact login name.
func (c *Client) FindUserByLogin(ctx context.Context, login string) (*UserSummary, error) {
	if login == "" {
		return nil, &ConfigError{Message: "login is empty"}
	}
	return userByCriteria(ctx, c, []searchCriterion{
		{Field: strconv.Itoa(fieldName), SearchType: "equals", Value: login},
	})
}

// FindUserByName resolves a GLPI user by firstname and/or realname. When only
// one name part is supplied the match is made on whichever field is present.
func (c *Client) FindUserByName(ctx context.Context, firstname, lastname string) (*UserSummary, error) {
	var criteria []searchCriterion
	if lastname != "" {
		criteria = append(criteria, searchCriterion{Field: strconv.Itoa(userFieldRealname), SearchType: "contains", Value: lastname})
	}
	if firstname != "" {
		criteria = append(criteria, searchCriterion{Field: strconv.Itoa(userFieldFirstname), SearchType: "contains", Value: firstname})
	}
	if len(criteria) == 0 {
		return nil, &ConfigError{Message: "firstname and lastname are both empty"}
	}
	return userByCriteria(ctx, c, criteria)
}

// ListUsers returns a page of GLPI users (login, realname, firstname, email).
func (c *Client) ListUsers(ctx context.Context, limit, page int) ([]UserSummary, int, error) {
	return c.searchUsers(ctx, nil, limit, page)
}

// GetUserProfiles returns the profile names attached to a GLPI user via the
// default profile (GET /User/{id}?expand_dropdowns=true → _profiles_id). An
// empty slice is returned when the user has no default profile.
func (c *Client) GetUserProfiles(ctx context.Context, userID int) ([]string, error) {
	values := url.Values{}
	values.Set("expand_dropdowns", "true")
	var full userFull
	if err := c.doRequest(ctx, http.MethodGet, "/apirest.php/User/"+strconv.Itoa(userID), values, nil, &full); err != nil {
		return nil, err
	}
	switch v := full.ProfilesID.(type) {
	case map[string]interface{}:
		if name, ok := v["name"].(string); ok && name != "" {
			return []string{name}, nil
		}
	case string:
		if v != "" && v != "0" {
			return []string{v}, nil
		}
	case float64:
		if id := int(v); id > 0 {
			return []string{ProfileNameByID(id)}, nil
		}
	}
	return []string{}, nil
}

// ProfileNameByID maps the default GLPI seeded profile ids to their well-known
// names. Custom profile ids fall back to the numeric representation.
func ProfileNameByID(id int) string {
	switch id {
	case 1:
		return "Self-Service"
	case 2:
		return "Observer"
	case 3:
		return "Admin"
	case 4:
		return "Super-Admin"
	case 5:
		return "Technician"
	case 6:
		return "Hotliner"
	case 7:
		return "Read-Only"
	case 8:
		return "Manager"
	default:
		return strconv.Itoa(id)
	}
}

// CreateUser creates a GLPI user and links the default profile and entity.
// It returns the new user's GLPI id. Callers must guard against duplicates
// (by email/login) before calling.
func (c *Client) CreateUser(ctx context.Context, req CreateUserRequest) (int, error) {
	if req.ProfileID == 0 {
		req.ProfileID = 1 // Self-Service
	}
	var out struct {
		ID int `json:"id"`
	}
	if err := c.doRequest(ctx, http.MethodPost, "/apirest.php/User", nil, map[string]interface{}{"input": req}, &out); err != nil {
		return 0, err
	}
	return out.ID, nil
}
