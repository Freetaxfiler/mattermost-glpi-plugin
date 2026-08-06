package glpi

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// GLPI ticket status values.
const (
	StatusNew        = 1
	StatusProcessing = 2
	StatusPlanned    = 3
	StatusPending    = 4
	StatusSolved     = 5
	StatusClosed     = 6
)

// StatusLabel returns a human-readable label for a GLPI ticket status.
func StatusLabel(status int) string {
	switch status {
	case StatusNew:
		return "New"
	case StatusProcessing:
		return "Processing (assigned)"
	case StatusPlanned:
		return "Processing (planned)"
	case StatusPending:
		return "Pending"
	case StatusSolved:
		return "Solved"
	case StatusClosed:
		return "Closed"
	default:
		return "Unknown (" + strconv.Itoa(status) + ")"
	}
}

// PriorityLabel returns a human-readable label for a GLPI priority/urgency/impact value.
func PriorityLabel(priority int) string {
	switch priority {
	case 1:
		return "Very low"
	case 2:
		return "Low"
	case 3:
		return "Medium"
	case 4:
		return "High"
	case 5:
		return "Very high"
	case 6:
		return "Major"
	default:
		return "Unknown (" + strconv.Itoa(priority) + ")"
	}
}

// CreateTicketRequest represents the payload used to create a GLPI ticket.
type CreateTicketRequest struct {
	Name           string
	Content        string
	Priority       int
	Urgency        int
	Impact         int
	Type           int
	ITILCategoryID int
	EntityID       int
	RequesterID    int
	// AssetID + AssetType link the ticket to a GLPI asset via the standard
	// "associated element" fields (_items_id / _itemtype).
	AssetID   int
	AssetType string
}

// CreateTicketResponse represents the response returned by GLPI.
type CreateTicketResponse struct {
	ID int `json:"id"`
}

// Ticket represents a full GLPI ticket as returned by GET /Ticket/{id}.
type Ticket struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Content  string `json:"content"`
	Status   int    `json:"status"`
	Priority int    `json:"priority"`
	Urgency  int    `json:"urgency"`
	Impact   int    `json:"impact"`
	Date     string `json:"date"`
	DateMod  string `json:"date_mod"`
	// Extended fields resolved via expand_dropdowns=true. Each is the
	// human-readable dropdown name; the numeric id is retained on the *_id
	// fields when the API returns them.
	Requester  string `json:"requester,omitempty"`
	Assignee   string `json:"assignee,omitempty"`
	Category   string `json:"category,omitempty"`
	AssetName  string `json:"asset_name,omitempty"`
	AssetType  string `json:"asset_type,omitempty"`
	AssetID    int    `json:"asset_id,omitempty"`
	CategoryID int    `json:"category_id,omitempty"`
	RequesterID int   `json:"requester_id,omitempty"`
	AssigneeID int    `json:"assignee_id,omitempty"`
	// Calculated urgency = (urgency + priority) / 2 for convenience
	UrgencyCalc int `json:"urgency_calc,omitempty"`
	// GLPI URL for opening the ticket
	Link       string `json:"link,omitempty"`
}

// CreateTicket creates a new ticket in GLPI.
func (c *Client) CreateTicket(ctx context.Context, req CreateTicketRequest) (*CreateTicketResponse, error) {
	input := map[string]interface{}{
		"name":    strings.TrimSpace(req.Name),
		"content": strings.TrimSpace(req.Content),
	}

	if req.Priority > 0 {
		input["priority"] = req.Priority
	}
	if req.Urgency > 0 {
		input["urgency"] = req.Urgency
	}
	if req.Impact > 0 {
		input["impact"] = req.Impact
	}
	if req.Type > 0 {
		input["type"] = req.Type
	}
	if req.ITILCategoryID > 0 {
		input["itilcategories_id"] = req.ITILCategoryID
	}
	if req.EntityID > 0 {
		input["entities_id"] = req.EntityID
	}
	if req.RequesterID > 0 {
		input["_users_id_requester"] = req.RequesterID
	}
	if req.AssetID > 0 && strings.TrimSpace(req.AssetType) != "" {
		// Link the ticket to the associated GLPI asset. GLPI accepts the
		// itemtype either with or without the "Asset" suffix normalization;
		// use the type as provided by the caller.
		input["_items_id"] = req.AssetID
		input["_itemtype"] = strings.TrimSpace(req.AssetType)
	}

	var result CreateTicketResponse
	err := c.doRequest(ctx, http.MethodPost, "/apirest.php/Ticket", nil, map[string]interface{}{"input": input}, &result)
	if err != nil {
		return nil, err
	}

	if result.ID == 0 {
		return nil, &NetworkError{Message: "GLPI returned success but no ticket ID"}
	}

	return &result, nil
}

// GetTicket retrieves a single ticket by ID, expanding dropdown relations
// (requester, assignee, category, linked asset) to human-readable names.
func (c *Client) GetTicket(ctx context.Context, id int) (*Ticket, error) {
	values := url.Values{}
	values.Set("expand_dropdowns", "true")

	var raw map[string]interface{}
	if err := c.doRequest(ctx, http.MethodGet, "/apirest.php/Ticket/"+strconv.Itoa(id), values, nil, &raw); err != nil {
		return nil, err
	}

	ticket := &Ticket{
		ID:        asInt(raw["id"]),
		Name:      asString(raw["name"]),
		Content:   asString(raw["content"]),
		Status:    asInt(raw["status"]),
		Priority:  asInt(raw["priority"]),
		Urgency:   asInt(raw["urgency"]),
		Impact:    asInt(raw["impact"]),
		Date:      asString(firstKnown(raw, "date", "date_creation")),
		DateMod:   asString(raw["date_mod"]),
		Requester: dropdownName(raw["_users_id_requester"]),
		Assignee:  dropdownName(raw["_users_id_assign"]),
		Category:  dropdownName(raw["itilcategories_id"]),
	}
	if ticket.Requester == "" {
		ticket.Requester = dropdownName(raw["users_id_requester"])
	}
	if ticket.Assignee == "" {
		ticket.Assignee = dropdownName(raw["users_id_assign"])
	}
	ticket.RequesterID = dropdownID(raw["_users_id_requester"])
	ticket.AssigneeID = dropdownID(raw["_users_id_assign"])
	ticket.CategoryID = dropdownID(raw["itilcategories_id"])

	// Calculated urgency = (urgency + priority) / 2 for convenience
	if ticket.Urgency > 0 && ticket.Priority > 0 {
		ticket.UrgencyCalc = (ticket.Urgency + ticket.Priority) / 2
	} else if ticket.Urgency > 0 {
		ticket.UrgencyCalc = ticket.Urgency
	} else if ticket.Priority > 0 {
		ticket.UrgencyCalc = ticket.Priority
	}

	// Linked asset (associated element): _items_id is the asset id and
	// _itemtype its GLPI item type.
	if itemID := dropdownID(raw["_items_id"]); itemID > 0 {
		ticket.AssetID = itemID
		ticket.AssetName = dropdownName(raw["_items_id"])
		ticket.AssetType = asString(raw["_itemtype"])
	}
	return ticket, nil
}

// UpdateTicket applies the given input fields to a ticket.
func (c *Client) UpdateTicket(ctx context.Context, id int, input map[string]interface{}) error {
	if input == nil {
		input = map[string]interface{}{}
	}
	input["id"] = id
	payload := map[string]interface{}{"input": input}
	return c.doRequest(ctx, http.MethodPut, "/apirest.php/Ticket/"+strconv.Itoa(id), nil, payload, nil)
}

// DeleteTicket moves a ticket to the GLPI trash bin.
func (c *Client) DeleteTicket(ctx context.Context, id int) error {
	return c.doRequest(ctx, http.MethodDelete, "/apirest.php/Ticket/"+strconv.Itoa(id), nil, nil, nil)
}

// AddFollowup adds a public or private follow-up to a ticket.
func (c *Client) AddFollowup(ctx context.Context, ticketID int, content string, isPrivate bool) error {
	private := 0
	if isPrivate {
		private = 1
	}

	payload := map[string]interface{}{
		"input": map[string]interface{}{
			"itemtype":   "Ticket",
			"items_id":   ticketID,
			"content":    strings.TrimSpace(content),
			"is_private": private,
		},
	}
	return c.doRequest(ctx, http.MethodPost, "/apirest.php/ITILFollowup", nil, payload, nil)
}

// UpdateFollowup edits the content/visibility of an existing ITILFollowup.
// GLPI supports PUT on the ITILFollowup item type.
func (c *Client) UpdateFollowup(ctx context.Context, followupID int, content string, isPrivate bool) error {
	private := 0
	if isPrivate {
		private = 1
	}
	payload := map[string]interface{}{
		"input": map[string]interface{}{
			"id":         followupID,
			"content":    strings.TrimSpace(content),
			"is_private": private,
		},
	}
	return c.doRequest(ctx, http.MethodPut, "/apirest.php/ITILFollowup/"+strconv.Itoa(followupID), nil, payload, nil)
}

// DeleteFollowup removes an ITILFollowup. GLPI supports DELETE on the
// ITILFollowup item type.
func (c *Client) DeleteFollowup(ctx context.Context, followupID int) error {
	return c.doRequest(ctx, http.MethodDelete, "/apirest.php/ITILFollowup/"+strconv.Itoa(followupID), nil, nil, nil)
}

// AddSolution attaches a solution to a ticket (moves it to Solved in GLPI).
func (c *Client) AddSolution(ctx context.Context, ticketID int, content string) error {
	payload := map[string]interface{}{
		"input": map[string]interface{}{
			"itemtype": "Ticket",
			"items_id": ticketID,
			"content":  strings.TrimSpace(content),
		},
	}
	return c.doRequest(ctx, http.MethodPost, "/apirest.php/ITILSolution", nil, payload, nil)
}

// TicketTemplateSummary is a compact ticket template row used for the picker.
type TicketTemplateSummary struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// TicketTemplate is a full ticket template used to prefill the creation form.
type TicketTemplate struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	CategoryID int    `json:"category_id"`
	Type       int    `json:"type"`
}

// SearchTicketTemplates lists GLPI ticket templates by name.
func (c *Client) SearchTicketTemplates(ctx context.Context, limit int) ([]TicketTemplateSummary, int, error) {
	if limit <= 0 {
		limit = 50
	}

	result, err := c.runSearch(ctx, searchQuery{
		ItemType:     "TicketTemplate",
		ForceDisplay: []int{fieldID, fieldName},
		Sort:         fieldName,
		Order:        "ASC",
		Limit:        limit,
	})
	if err != nil {
		return nil, 0, err
	}

	templates := make([]TicketTemplateSummary, 0, len(result.Data))
	for _, row := range result.Data {
		templates = append(templates, TicketTemplateSummary{
			ID:   asInt(row[strconv.Itoa(fieldID)]),
			Name: asString(row[strconv.Itoa(fieldName)]),
		})
	}
	return templates, result.TotalCount, nil
}

// GetTicketTemplate retrieves a single ticket template with its category
// resolved to a numeric ID so the creation form can be prefilled.
func (c *Client) GetTicketTemplate(ctx context.Context, id int) (*TicketTemplate, error) {
	values := url.Values{}
	values.Set("expand_dropdowns", "true")

	var raw map[string]interface{}
	if err := c.doRequest(ctx, http.MethodGet, "/apirest.php/TicketTemplate/"+strconv.Itoa(id), values, nil, &raw); err != nil {
		return nil, err
	}

	return &TicketTemplate{
		ID:         asInt(raw["id"]),
		Name:       asString(raw["name"]),
		Content:    asString(raw["content"]),
		CategoryID: dropdownID(raw["ticketcategories_id"]),
		Type:       asInt(raw["type"]),
	}, nil
}
