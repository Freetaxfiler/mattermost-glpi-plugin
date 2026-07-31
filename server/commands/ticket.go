package commands

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/glpi"
	"github.com/mattermost/mattermost/server/public/model"
)

const commandTimeout = 15 * time.Second

func ticketURL(config *ConfigView, id int) string {
	return strings.TrimRight(config.GLPIURL, "/") + "/front/ticket.form.php?id=" + strconv.Itoa(id)
}

func parseTicketID(rest []string, usage string) (int, *model.CommandResponse) {
	if len(rest) < 1 {
		return 0, responseText(usage)
	}
	id, err := strconv.Atoi(rest[0])
	if err != nil || id <= 0 {
		return 0, responseText(fmt.Sprintf("`%s` is not a valid ticket ID.\n\n%s", rest[0], usage))
	}
	return id, nil
}

func clientOrError(p PluginExecutor) (glpi.GLPIClient, *model.CommandResponse) {
	client := p.GetGLPIClient()
	if client == nil {
		return nil, responseText("GLPI client is not initialized. Check the plugin configuration and run `/glpi status`.")
	}
	return client, nil
}

func friendlyError(action string, err error) *model.CommandResponse {
	var notFound *glpi.NotFoundError
	var authErr *glpi.AuthError
	switch {
	case errors.As(err, &notFound):
		return responseText(fmt.Sprintf("%s failed: %s", action, notFound.Error()))
	case errors.As(err, &authErr):
		return responseText(fmt.Sprintf("%s failed: GLPI rejected the configured tokens. Ask an administrator to verify the plugin settings.", action))
	default:
		// Log the full error details server-side, return a sanitised message to the user.
		return responseText(fmt.Sprintf("%s failed with an unexpected error. Contact your system administrator if the problem persists.", action))
	}
}

func executeViewTicket(ctx context.Context, p PluginExecutor, config *ConfigView, rest []string) (*model.CommandResponse, *model.AppError) {
	id, errResp := parseTicketID(rest, "Usage: `/glpi view <ticket id>`")
	if errResp != nil {
		return errResp, nil
	}

	client, errResp := clientOrError(p)
	if errResp != nil {
		return errResp, nil
	}

	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	ticket, err := client.GetTicket(ctx, id)
	if err != nil {
		return friendlyError("Fetching the ticket", err), nil
	}

	content := glpi.StripHTML(ticket.Content)
	if len(content) > 1500 {
		content = content[:1500] + "…"
	}

	message := fmt.Sprintf(
		"### Ticket #%d — %s\n"+
			"| Field | Value |\n|---|---|\n"+
			"| Status | %s |\n"+
			"| Priority | %s |\n"+
			"| Urgency | %s |\n"+
			"| Impact | %s |\n"+
			"| Opened | %s |\n"+
			"| Last update | %s |\n\n"+
			"**Description**\n%s\n\n[Open in GLPI](%s)",
		ticket.ID,
		ticket.Name,
		glpi.StatusLabel(ticket.Status),
		glpi.PriorityLabel(ticket.Priority),
		glpi.PriorityLabel(ticket.Urgency),
		glpi.PriorityLabel(ticket.Impact),
		ticket.Date,
		ticket.DateMod,
		content,
		ticketURL(config, ticket.ID),
	)
	return responseText(message), nil
}

func executeFollowup(ctx context.Context, p PluginExecutor, rest []string, isPrivate bool) (*model.CommandResponse, *model.AppError) {
	label := "comment"
	if isPrivate {
		label = "private"
	}
	usage := fmt.Sprintf("Usage: `/glpi %s <ticket id> <text>`", label)

	id, errResp := parseTicketID(rest, usage)
	if errResp != nil {
		return errResp, nil
	}
	if len(rest) < 2 {
		return responseText(usage), nil
	}
	text := strings.TrimSpace(strings.Join(rest[1:], " "))
	if text == "" {
		return responseText(usage), nil
	}

	client, errResp := clientOrError(p)
	if errResp != nil {
		return errResp, nil
	}

	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	if err := client.AddFollowup(ctx, id, text, isPrivate); err != nil {
		return friendlyError("Adding the follow-up", err), nil
	}

	visibility := "public"
	if isPrivate {
		visibility = "private"
	}
	return responseText(fmt.Sprintf("✅ Added a %s follow-up to ticket #%d.", visibility, id)), nil
}

func executeUpdateTicket(ctx context.Context, p PluginExecutor, rest []string) (*model.CommandResponse, *model.AppError) {
	usage := "Usage: `/glpi update <ticket id> <priority|urgency|status|title> <value>`\n" +
		"- priority / urgency: 1 (very low) … 5 (very high)\n" +
		"- status: new, processing, planned, pending, solved, closed\n" +
		"- title: the new ticket title"

	id, errResp := parseTicketID(rest, usage)
	if errResp != nil {
		return errResp, nil
	}
	if len(rest) < 3 {
		return responseText(usage), nil
	}

	field := strings.ToLower(rest[1])
	value := strings.TrimSpace(strings.Join(rest[2:], " "))

	input := map[string]interface{}{}
	switch field {
	case "priority", "urgency":
		level, err := strconv.Atoi(value)
		if err != nil || level < 1 || level > 6 {
			return responseText(fmt.Sprintf("`%s` is not a valid %s (use 1-5).\n\n%s", value, field, usage)), nil
		}
		input[field] = level

	case "status":
		status, ok := statusFromName(value)
		if !ok {
			return responseText(fmt.Sprintf("`%s` is not a valid status.\n\n%s", value, usage)), nil
		}
		input["status"] = status

	case "title":
		if value == "" {
			return responseText(usage), nil
		}
		input["name"] = value

	default:
		return responseText(fmt.Sprintf("`%s` is not an updatable field.\n\n%s", field, usage)), nil
	}

	client, errResp := clientOrError(p)
	if errResp != nil {
		return errResp, nil
	}

	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	if err := client.UpdateTicket(ctx, id, input); err != nil {
		return friendlyError("Updating the ticket", err), nil
	}
	return responseText(fmt.Sprintf("✅ Ticket #%d updated (%s → %s).", id, field, value)), nil
}

func statusFromName(name string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "new", "1":
		return glpi.StatusNew, true
	case "processing", "assigned", "2":
		return glpi.StatusProcessing, true
	case "planned", "3":
		return glpi.StatusPlanned, true
	case "pending", "waiting", "4":
		return glpi.StatusPending, true
	case "solved", "5":
		return glpi.StatusSolved, true
	case "closed", "6":
		return glpi.StatusClosed, true
	default:
		return 0, false
	}
}

func executeCloseTicket(ctx context.Context, p PluginExecutor, rest []string) (*model.CommandResponse, *model.AppError) {
	usage := "Usage: `/glpi close <ticket id> [solution text]`"

	id, errResp := parseTicketID(rest, usage)
	if errResp != nil {
		return errResp, nil
	}

	client, errResp := clientOrError(p)
	if errResp != nil {
		return errResp, nil
	}

	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	solution := strings.TrimSpace(strings.Join(rest[1:], " "))
	if solution != "" {
		// Record the solution only and let GLPI apply its own approval
		// workflow. Forcing a status update here would bypass GLPI's ticket
		// lifecycle (the ticket is moved to Solved/Closed by GLPI).
		if err := client.AddSolution(ctx, id, solution); err != nil {
			return friendlyError("Recording the solution", err), nil
		}
		return responseText(fmt.Sprintf("✅ Solution recorded for ticket #%d. GLPI will process the closure.", id)), nil
	}

	if err := client.UpdateTicket(ctx, id, map[string]interface{}{"status": glpi.StatusClosed}); err != nil {
		return friendlyError("Closing the ticket", err), nil
	}
	return responseText(fmt.Sprintf("✅ Ticket #%d closed.", id)), nil
}

func executeReopenTicket(ctx context.Context, p PluginExecutor, rest []string) (*model.CommandResponse, *model.AppError) {
	usage := "Usage: `/glpi reopen <ticket id>`"

	id, errResp := parseTicketID(rest, usage)
	if errResp != nil {
		return errResp, nil
	}

	client, errResp := clientOrError(p)
	if errResp != nil {
		return errResp, nil
	}

	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	// Fetch the current ticket to validate it can be reopened.
	ticket, err := client.GetTicket(ctx, id)
	if err != nil {
		return friendlyError("Fetching the ticket", err), nil
	}
	if ticket.Status != glpi.StatusSolved && ticket.Status != glpi.StatusClosed {
		return responseText(fmt.Sprintf("Ticket #%d is not solved or closed and cannot be reopened.", id)), nil
	}

	if err := client.UpdateTicket(ctx, id, map[string]interface{}{"status": glpi.StatusProcessing}); err != nil {
		return friendlyError("Reopening the ticket", err), nil
	}
	return responseText(fmt.Sprintf("✅ Ticket #%d reopened.", id)), nil
}

func executeDeleteTicket(ctx context.Context, p PluginExecutor, rest []string) (*model.CommandResponse, *model.AppError) {
	usage := "Usage: `/glpi delete <ticket id>`"

	id, errResp := parseTicketID(rest, usage)
	if errResp != nil {
		return errResp, nil
	}

	client, errResp := clientOrError(p)
	if errResp != nil {
		return errResp, nil
	}

	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	if err := client.DeleteTicket(ctx, id); err != nil {
		return friendlyError("Deleting the ticket", err), nil
	}
	return responseText(fmt.Sprintf("🗑️ Ticket #%d moved to the GLPI trash.", id)), nil
}

func executeAttach(ctx context.Context, p PluginExecutor, args *model.CommandArgs, rest []string) (*model.CommandResponse, *model.AppError) {
	usage := "Usage: `/glpi attach <ticket id>`\nUpload a file to this channel first, then run the command to attach it to the ticket."

	id, errResp := parseTicketID(rest, usage)
	if errResp != nil {
		return errResp, nil
	}

	client, errResp := clientOrError(p)
	if errResp != nil {
		return errResp, nil
	}

	filename, data, err := p.LatestFileAttachment(args.UserId, args.ChannelId)
	if err != nil {
		return responseText(fmt.Sprintf("Could not find a recent file you posted in this channel: %v\n\n%s", err, usage)), nil
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	documentID, err := client.UploadDocument(ctx, filename, data, id)
	if err != nil {
		return friendlyError("Uploading the attachment", err), nil
	}

	return responseText(fmt.Sprintf("📎 Attached `%s` to ticket #%d (document #%d).", filename, id, documentID)), nil
}

// VisibleTimelineEvents filters timeline events based on user role.
// Private events are only visible to system administrators to prevent
// exposing confidential information through a shared GLPI API account.
// Exported so the webapp REST timeline handler applies the same policy.
func VisibleTimelineEvents(events []glpi.TimelineEvent, isSystemAdmin bool) []glpi.TimelineEvent {
	if isSystemAdmin {
		return events
	}
	filtered := make([]glpi.TimelineEvent, 0, len(events))
	for _, e := range events {
		if !e.IsPrivate {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
