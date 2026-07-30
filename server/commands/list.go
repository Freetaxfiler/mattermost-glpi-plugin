package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/glpi"
	"github.com/mattermost/mattermost/server/public/model"
)

const listLimit = 15

func executeMyTickets(ctx context.Context, p PluginExecutor, config *ConfigView, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	glpiUserID, errResp := resolveGLPIUser(p, args.UserId)
	if errResp != nil {
		return errResp, nil
	}
	return listTickets(ctx, p, config, glpi.TicketFilter{RequesterID: glpiUserID, Limit: listLimit}, "Tickets you requested")
}

func executeAssignedTickets(ctx context.Context, p PluginExecutor, config *ConfigView, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	glpiUserID, errResp := resolveGLPIUser(p, args.UserId)
	if errResp != nil {
		return errResp, nil
	}
	return listTickets(ctx, p, config, glpi.TicketFilter{AssigneeID: glpiUserID, Limit: listLimit}, "Tickets assigned to you")
}

func executeSearchTickets(ctx context.Context, p PluginExecutor, config *ConfigView, rest []string) (*model.CommandResponse, *model.AppError) {
	// rate-limit interactive text searches per-user to avoid abuse
	if !checkAndRecordUserRate(ctx) {
		return responseText("You're searching too frequently. Please wait a moment and try again."), nil
	}

	query := strings.TrimSpace(strings.Join(rest, " "))
	if query == "" {
		return responseText("Usage: `/glpi search <text>`"), nil
	}
	return listTickets(ctx, p, config, glpi.TicketFilter{TitleQuery: query, Limit: listLimit}, fmt.Sprintf("Tickets matching `%s`", query))
}

func resolveGLPIUser(p PluginExecutor, mattermostUserID string) (int, *model.CommandResponse) {
	glpiUserID, err := p.GetGLPIUserID(mattermostUserID)
	if err != nil {
		var notFound *glpi.NotFoundError
		if errors.As(err, &notFound) {
			return 0, responseText("Your Mattermost email address does not match any GLPI user. Ask your GLPI administrator to check your account email.")
		}
		return 0, responseText(fmt.Sprintf("Could not resolve your GLPI account: %v", err))
	}
	return glpiUserID, nil
}

func listTickets(ctx context.Context, p PluginExecutor, config *ConfigView, filter glpi.TicketFilter, title string) (*model.CommandResponse, *model.AppError) {
	client, errResp := clientOrError(p)
	if errResp != nil {
		return errResp, nil
	}

	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	tickets, total, err := client.SearchTickets(ctx, filter)
	if err != nil {
		return friendlyError("Searching tickets", err), nil
	}

	if len(tickets) == 0 {
		return responseText(fmt.Sprintf("### %s\nNo tickets found.", title)), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("### %s\n", title))
	b.WriteString("| ID | Title | Status | Priority | Opened |\n")
	b.WriteString("|---:|---|---|---|---|\n")
	for _, ticket := range tickets {
		name := ticket.Name
		if len(name) > 60 {
			name = name[:60] + "…"
		}
		b.WriteString(fmt.Sprintf(
			"| [%d](%s) | %s | %s | %s | %s |\n",
			ticket.ID,
			ticketURL(config, ticket.ID),
			escapePipes(name),
			glpi.StatusLabel(ticket.Status),
			glpi.PriorityLabel(ticket.Priority),
			ticket.Opened,
		))
	}
	if total > len(tickets) {
		b.WriteString(fmt.Sprintf("\nShowing %d of %d tickets. Refine with `/glpi search <text>` or open GLPI for the full list.", len(tickets), total))
	}
	b.WriteString("\nUse `/glpi view <id>` for details.")
	return responseText(b.String()), nil
}

func escapePipes(text string) string {
	return strings.ReplaceAll(text, "|", "\\|")
}
