package glpi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// GLPI search-engine field IDs. These are the standard search option IDs shipped
// with GLPI; adjust here if a GLPI instance uses customized search options.
const (
	// Common to nearly all item types.
	fieldName = 1
	fieldID   = 2

	// Ticket search options.
	ticketFieldPriority   = 3
	ticketFieldRequester  = 4
	ticketFieldTechnician = 5
	ticketFieldStatus     = 12
	ticketFieldOpenDate   = 15
	ticketFieldDateMod    = 19

	// User search options.
	userFieldEmail = 5

	// Asset search options (Computer, Printer, Monitor, NetworkEquipment).
	assetFieldSerial = 5
	assetFieldUser   = 70
)

type searchCriterion struct {
	Link       string
	Field      string
	SearchType string
	Value      string
}

type searchQuery struct {
	ItemType     string
	Criteria     []searchCriterion
	ForceDisplay []int
	Sort         int
	Order        string
	Limit        int
}

type searchResponse struct {
	TotalCount int                      `json:"totalcount"`
	Count      int                      `json:"count"`
	Data       []map[string]interface{} `json:"data"`
}

// runSearch executes a query against the GLPI search engine (/search/{itemtype}).
func (c *Client) runSearch(ctx context.Context, q searchQuery) (*searchResponse, error) {
	values := url.Values{}

	for i, criterion := range q.Criteria {
		prefix := fmt.Sprintf("criteria[%d]", i)
		if i > 0 {
			link := criterion.Link
			if link == "" {
				link = "AND"
			}
			values.Set(prefix+"[link]", link)
		}
		values.Set(prefix+"[field]", criterion.Field)
		values.Set(prefix+"[searchtype]", criterion.SearchType)
		values.Set(prefix+"[value]", criterion.Value)
	}

	for i, field := range q.ForceDisplay {
		values.Set(fmt.Sprintf("forcedisplay[%d]", i), strconv.Itoa(field))
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 15
	}
	values.Set("range", "0-"+strconv.Itoa(limit-1))

	if q.Sort > 0 {
		values.Set("sort", strconv.Itoa(q.Sort))
		order := q.Order
		if order == "" {
			order = "DESC"
		}
		values.Set("order", order)
	}

	var result searchResponse
	err := c.doRequest(ctx, http.MethodGet, "/apirest.php/search/"+q.ItemType, values, nil, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// TicketFilter describes which tickets to look up.
type TicketFilter struct {
	RequesterID int
	AssigneeID  int
	TitleQuery  string
	Limit       int
}

// TicketSummary is a compact ticket row returned by the search engine.
type TicketSummary struct {
	ID       int
	Name     string
	Status   int
	Priority int
	Opened   string
}

// SearchTickets queries tickets by requester, assignee, and/or title.
func (c *Client) SearchTickets(ctx context.Context, filter TicketFilter) ([]TicketSummary, int, error) {
	var criteria []searchCriterion

	if filter.RequesterID > 0 {
		criteria = append(criteria, searchCriterion{
			Field:      strconv.Itoa(ticketFieldRequester),
			SearchType: "equals",
			Value:      strconv.Itoa(filter.RequesterID),
		})
	}
	if filter.AssigneeID > 0 {
		criteria = append(criteria, searchCriterion{
			Field:      strconv.Itoa(ticketFieldTechnician),
			SearchType: "equals",
			Value:      strconv.Itoa(filter.AssigneeID),
		})
	}
	if filter.TitleQuery != "" {
		criteria = append(criteria, searchCriterion{
			Field:      strconv.Itoa(fieldName),
			SearchType: "contains",
			Value:      filter.TitleQuery,
		})
	}
	if len(criteria) == 0 {
		// Match everything: id > 0.
		criteria = append(criteria, searchCriterion{
			Field:      strconv.Itoa(fieldID),
			SearchType: "morethan",
			Value:      "0",
		})
	}

	result, err := c.runSearch(ctx, searchQuery{
		ItemType: "Ticket",
		Criteria: criteria,
		ForceDisplay: []int{
			fieldID, fieldName, ticketFieldStatus, ticketFieldPriority, ticketFieldOpenDate,
		},
		Sort:  ticketFieldDateMod,
		Order: "DESC",
		Limit: filter.Limit,
	})
	if err != nil {
		return nil, 0, err
	}

	summaries := make([]TicketSummary, 0, len(result.Data))
	for _, row := range result.Data {
		summaries = append(summaries, TicketSummary{
			ID:       asInt(row[strconv.Itoa(fieldID)]),
			Name:     asString(row[strconv.Itoa(fieldName)]),
			Status:   asInt(row[strconv.Itoa(ticketFieldStatus)]),
			Priority: asInt(row[strconv.Itoa(ticketFieldPriority)]),
			Opened:   asString(row[strconv.Itoa(ticketFieldOpenDate)]),
		})
	}
	return summaries, result.TotalCount, nil
}

// FindUserIDByEmail resolves a GLPI user ID from an email address.
func (c *Client) FindUserIDByEmail(ctx context.Context, email string) (int, error) {
	if email == "" {
		return 0, &ConfigError{Message: "email is empty"}
	}

	result, err := c.runSearch(ctx, searchQuery{
		ItemType: "User",
		Criteria: []searchCriterion{
			{
				Field:      strconv.Itoa(userFieldEmail),
				SearchType: "contains",
				Value:      email,
			},
		},
		ForceDisplay: []int{fieldID, fieldName},
		Limit:        1,
	})
	if err != nil {
		return 0, err
	}

	if len(result.Data) == 0 {
		return 0, &NotFoundError{Message: "no GLPI user found for email " + email}
	}

	id := asInt(result.Data[0][strconv.Itoa(fieldID)])
	if id == 0 {
		return 0, &NetworkError{Message: "GLPI user search returned a row without an ID"}
	}
	return id, nil
}

func asString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func asInt(v interface{}) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case string:
		parsed, err := strconv.Atoi(t)
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}
