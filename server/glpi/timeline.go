package glpi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const (
	defaultTimelinePageSize = 20
	maxTimelinePageSize     = 100
	maxTimelineFetchCount   = 1000
)

// TimelineKind identifies the GLPI object that produced a timeline event.
type TimelineKind string

const (
	TimelineFollowup   TimelineKind = "followup"
	TimelineSolution   TimelineKind = "solution"
	TimelineValidation TimelineKind = "validation"
	TimelineHistory    TimelineKind = "history"
)

// TimelinePageRequest selects a one-based page of a ticket timeline.
type TimelinePageRequest struct {
	Page    int
	PerPage int
}

// TimelineEvent is a normalized event returned from the GLPI ticket timeline.
// GLPI permissions determine which private events the API account can retrieve.
type TimelineEvent struct {
	ID        int
	Kind      TimelineKind
	Content   string
	Date      string
	AuthorID  int
	Author    string
	IsPrivate bool
	Status    string
}

// TimelinePage contains one ordered page and the total number of events
// reported by GLPI's Content-Range headers.
type TimelinePage struct {
	Events  []TimelineEvent
	Page    int
	PerPage int
	Total   int
	HasMore bool
}

type timelineSource struct {
	subItemType string
	kind        TimelineKind
	required    bool
}

var ticketTimelineSources = []timelineSource{
	{subItemType: "ITILFollowup", kind: TimelineFollowup, required: true},
	{subItemType: "ITILSolution", kind: TimelineSolution},
	{subItemType: "TicketValidation", kind: TimelineValidation},
	{subItemType: "Log", kind: TimelineHistory},
}

// GetTicketTimeline retrieves a stable, newest-first view over the standard
// legacy REST sub-items that make up a GLPI ticket timeline. Callers must
// enforce Mattermost visibility rules before rendering private events because
// the GLPI client uses a shared API account.
func (c *Client) GetTicketTimeline(ctx context.Context, ticketID int, request TimelinePageRequest) (*TimelinePage, error) {
	if ticketID <= 0 {
		return nil, &ConfigError{Message: "ticket ID must be positive"}
	}

	page, perPage, err := normalizeTimelinePageRequest(request)
	if err != nil {
		return nil, err
	}

	offset := (page - 1) * perPage
	// Fetch everything needed to construct this page from every source. GLPI
	// paginates each sub-item collection independently, so fetching only one
	// source page would skip events when the collections are merged.
	fetchCount := offset + perPage
	events := make([]TimelineEvent, 0, fetchCount)
	total := 0
	totalKnown := true

	for _, source := range ticketTimelineSources {
		sourceEvents, sourceTotal, sourceTotalKnown, err := c.getTicketTimelineSource(ctx, ticketID, source, fetchCount)
		if err != nil {
			var notFound *NotFoundError
			if !source.required && isAsNotFound(err, &notFound) {
				// Some GLPI profiles or deployments do not expose every optional
				// timeline sub-item. A supported follow-up timeline remains useful.
				continue
			}
			return nil, err
		}
		events = append(events, sourceEvents...)
		if sourceTotalKnown {
			total += sourceTotal
		} else {
			totalKnown = false
		}
	}

	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Date != events[j].Date {
			return events[i].Date > events[j].Date
		}
		if events[i].ID != events[j].ID {
			return events[i].ID > events[j].ID
		}
		return events[i].Kind < events[j].Kind
	})

	if offset >= len(events) {
		return &TimelinePage{
			Events:  []TimelineEvent{},
			Page:    page,
			PerPage: perPage,
			Total:   timelineTotal(total, totalKnown, len(events)),
			HasMore: totalKnown && total > offset,
		}, nil
	}

	end := offset + perPage
	if end > len(events) {
		end = len(events)
	}
	pageEvents := append([]TimelineEvent(nil), events[offset:end]...)
	return &TimelinePage{
		Events:  pageEvents,
		Page:    page,
		PerPage: perPage,
		Total:   timelineTotal(total, totalKnown, len(events)),
		HasMore: totalKnown && total > end,
	}, nil
}

func normalizeTimelinePageRequest(request TimelinePageRequest) (int, int, error) {
	page := request.Page
	if page == 0 {
		page = 1
	}
	if page < 1 {
		return 0, 0, &ConfigError{Message: "timeline page must be at least 1"}
	}

	perPage := request.PerPage
	if perPage == 0 {
		perPage = defaultTimelinePageSize
	}
	if perPage < 1 || perPage > maxTimelinePageSize {
		return 0, 0, &ConfigError{Message: fmt.Sprintf("timeline page size must be between 1 and %d", maxTimelinePageSize)}
	}
	if page > maxTimelineFetchCount/perPage {
		return 0, 0, &ConfigError{Message: fmt.Sprintf("timeline page exceeds the maximum accessible range of %d events", maxTimelineFetchCount)}
	}
	return page, perPage, nil
}

func (c *Client) getTicketTimelineSource(ctx context.Context, ticketID int, source timelineSource, fetchCount int) ([]TimelineEvent, int, bool, error) {
	query := url.Values{}
	query.Set("range", "0-"+strconv.Itoa(fetchCount-1))
	query.Set("order", "DESC")

	rows := []map[string]interface{}{}
	metadata := &ResponseMetadata{}
	path := "/apirest.php/Ticket/" + strconv.Itoa(ticketID) + "/" + source.subItemType
	if err := c.doRequestWithResponse(ctx, http.MethodGet, path, query, nil, &rows, metadata); err != nil {
		return nil, 0, false, err
	}

	events := make([]TimelineEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, timelineEventFromRow(source.kind, row))
	}

	total, known := contentRangeTotal(metadata.Header.Get("Content-Range"))
	if !known {
		total = len(events)
	}
	return events, total, known, nil
}

func timelineEventFromRow(kind TimelineKind, row map[string]interface{}) TimelineEvent {
	date := firstTimelineString(row, "date", "date_creation", "date_mod")
	content := firstTimelineString(row, "content", "new_value", "name", "message")
	if content == "" {
		content = "GLPI ticket history updated"
	}

	return TimelineEvent{
		ID:        asInt(row["id"]),
		Kind:      kind,
		Content:   content,
		Date:      date,
		AuthorID:  firstTimelineInt(row, "users_id", "users_id_editor", "user_id"),
		Author:    firstTimelineString(row, "user_name", "users_name", "user", "name_user"),
		IsPrivate: timelineBool(row["is_private"]),
		Status:    firstTimelineString(row, "status", "status_name"),
	}
}

func firstTimelineString(row map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(asString(row[key])); value != "" {
			return value
		}
	}
	return ""
}

func firstTimelineInt(row map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		if value := asInt(row[key]); value > 0 {
			return value
		}
	}
	return 0
}

func timelineBool(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed != 0
	case string:
		return typed == "1" || strings.EqualFold(typed, "true")
	default:
		return false
	}
}

func contentRangeTotal(header string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(header), "/")
	if len(parts) != 2 || parts[1] == "*" {
		return 0, false
	}
	total, err := strconv.Atoi(parts[1])
	if err != nil || total < 0 {
		return 0, false
	}
	return total, true
}

func timelineTotal(total int, known bool, fetched int) int {
	if known {
		return total
	}
	return fetched
}
