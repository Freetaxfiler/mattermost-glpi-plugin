package glpi

import (
	"context"
	"net/http"
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

// GetTicket retrieves a single ticket by ID.
func (c *Client) GetTicket(ctx context.Context, id int) (*Ticket, error) {
	var ticket Ticket
	err := c.doRequest(ctx, http.MethodGet, "/apirest.php/Ticket/"+strconv.Itoa(id), nil, nil, &ticket)
	if err != nil {
		return nil, err
	}
	return &ticket, nil
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
