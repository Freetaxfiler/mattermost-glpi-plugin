package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/glpi"
	"github.com/mattermost/mattermost/server/public/model"
)

func TestVisibleTimelineEvents(t *testing.T) {
	events := []glpi.TimelineEvent{
		{ID: 1, Kind: glpi.TimelineFollowup},
		{ID: 2, Kind: glpi.TimelineFollowup, IsPrivate: true},
	}

	userEvents := VisibleTimelineEvents(events, false)
	if len(userEvents) != 1 || userEvents[0].ID != 1 {
		t.Fatalf("regular users must not receive private timeline events: %+v", userEvents)
	}

	adminEvents := VisibleTimelineEvents(events, true)
	if len(adminEvents) != 2 {
		t.Fatalf("system administrators should receive all timeline events: %+v", adminEvents)
	}
}

func TestCloseTicketWithSolutionLetsGLPIApplyApprovalWorkflow(t *testing.T) {
	client := &workflowTestClient{ticket: &glpi.Ticket{ID: 17, Status: glpi.StatusProcessing}}
	response, appErr := executeCloseTicket(context.Background(), workflowTestExecutor{client: client}, []string{"17", "Replaced", "the", "disk"})
	if appErr != nil {
		t.Fatalf("executeCloseTicket returned app error: %v", appErr)
	}
	if client.solutionCalls != 1 {
		t.Fatalf("solution calls = %d, want 1", client.solutionCalls)
	}
	if len(client.updates) != 0 {
		t.Fatalf("solution workflow must not force a ticket status update: %+v", client.updates)
	}
	if !strings.Contains(response.Text, "Solution recorded") {
		t.Fatalf("unexpected response: %q", response.Text)
	}
}

func TestCloseTicketWithoutSolutionRequestsDirectClose(t *testing.T) {
	client := &workflowTestClient{ticket: &glpi.Ticket{ID: 18, Status: glpi.StatusProcessing}}
	response, appErr := executeCloseTicket(context.Background(), workflowTestExecutor{client: client}, []string{"18"})
	if appErr != nil {
		t.Fatalf("executeCloseTicket returned app error: %v", appErr)
	}
	if client.solutionCalls != 0 {
		t.Fatalf("solution calls = %d, want 0", client.solutionCalls)
	}
	if len(client.updates) != 1 || client.updates[0]["status"] != glpi.StatusClosed {
		t.Fatalf("direct close must request status closed: %+v", client.updates)
	}
	if !strings.Contains(response.Text, "closed") {
		t.Fatalf("unexpected response: %q", response.Text)
	}
}

func TestReopenTicketRejectsActiveTickets(t *testing.T) {
	client := &workflowTestClient{ticket: &glpi.Ticket{ID: 19, Status: glpi.StatusProcessing}}
	response, appErr := executeReopenTicket(context.Background(), workflowTestExecutor{client: client}, []string{"19"})
	if appErr != nil {
		t.Fatalf("executeReopenTicket returned app error: %v", appErr)
	}
	if len(client.updates) != 0 {
		t.Fatalf("active ticket must not be updated: %+v", client.updates)
	}
	if !strings.Contains(response.Text, "cannot be reopened") {
		t.Fatalf("unexpected response: %q", response.Text)
	}
}

type workflowTestExecutor struct {
	client glpi.GLPIClient
}

func (e workflowTestExecutor) GetConfiguration() *ConfigView { return &ConfigView{} }
func (e workflowTestExecutor) GetGLPIClient() glpi.GLPIClient { return e.client }
func (e workflowTestExecutor) OpenCreateTicketDialog(*model.CommandArgs) error { return nil }
func (e workflowTestExecutor) GetGLPIUserID(string) (int, error) { return 0, nil }
func (e workflowTestExecutor) GetMyTickets(string) ([]glpi.TicketSummary, int, error) { return nil, 0, nil }
func (e workflowTestExecutor) IsSystemAdmin(string) bool { return false }
func (e workflowTestExecutor) LatestFileAttachment(string, string) (string, []byte, error) {
	return "", nil, nil
}

type workflowTestClient struct {
	ticket        *glpi.Ticket
	solutionCalls int
	updates       []map[string]interface{}
}

func (c *workflowTestClient) HealthCheck(context.Context) (*glpi.HealthCheckResponse, error) {
	return &glpi.HealthCheckResponse{}, nil
}
func (c *workflowTestClient) KillSession(context.Context) error { return nil }
func (c *workflowTestClient) CreateTicket(context.Context, glpi.CreateTicketRequest) (*glpi.CreateTicketResponse, error) {
	return &glpi.CreateTicketResponse{}, nil
}
func (c *workflowTestClient) GetTicket(context.Context, int) (*glpi.Ticket, error) { return c.ticket, nil }
func (c *workflowTestClient) UpdateTicket(_ context.Context, _ int, input map[string]interface{}) error {
	copyInput := make(map[string]interface{}, len(input))
	for key, value := range input {
		copyInput[key] = value
	}
	c.updates = append(c.updates, copyInput)
	return nil
}
func (c *workflowTestClient) DeleteTicket(context.Context, int) error { return nil }
func (c *workflowTestClient) AddFollowup(context.Context, int, string, bool) error { return nil }
func (c *workflowTestClient) AddSolution(context.Context, int, string) error {
	c.solutionCalls++
	return nil
}
func (c *workflowTestClient) SearchTickets(context.Context, glpi.TicketFilter) ([]glpi.TicketSummary, int, error) {
	return nil, 0, nil
}
func (c *workflowTestClient) GetTicketTimeline(context.Context, int, glpi.TimelinePageRequest) (*glpi.TimelinePage, error) {
	return &glpi.TimelinePage{}, nil
}
func (c *workflowTestClient) FindUserIDByEmail(context.Context, string) (int, error) { return 0, nil }
func (c *workflowTestClient) SearchAssets(context.Context, glpi.AssetFilter) ([]glpi.AssetSummary, int, error) {
	return nil, 0, nil
}
func (c *workflowTestClient) SearchKnowledge(context.Context, string, int, int, int) ([]glpi.KnowledgeSummary, int, error) {
	return nil, 0, nil
}
func (c *workflowTestClient) SearchKnowledgeBaseCategories(context.Context, int) ([]glpi.KnowbaseCategorySummary, int, error) {
	return nil, 0, nil
}
func (c *workflowTestClient) UploadDocument(context.Context, string, []byte, int) (int, error) { return 0, nil }
func (c *workflowTestClient) SearchITILCategories(context.Context, string, int) ([]glpi.CategorySummary, int, error) {
	return nil, 0, nil
}
func (c *workflowTestClient) GetKnowbaseItem(context.Context, int) (*glpi.KnowledgeArticle, error) { return nil, nil }
func (c *workflowTestClient) GetAsset(context.Context, string, int) (*glpi.AssetDetail, error) { return nil, nil }
func (c *workflowTestClient) ListTicketDocuments(context.Context, int) ([]glpi.DocumentInfo, error) { return nil, nil }
func (c *workflowTestClient) GetDocumentContent(context.Context, int) ([]byte, string, error) { return nil, "", nil }
