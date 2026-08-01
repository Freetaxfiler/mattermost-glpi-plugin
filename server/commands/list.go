package commands

import (
	"context"
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
	if glpiUserID > 0 {
		return listTickets(ctx, p, config, glpi.TicketFilter{RequesterID: glpiUserID, Limit: listLimit}, "Tickets you requested")
	}
	// No individual GLPI account (integration mode): fall back to the
	// identity-service ownership mapping so "My Tickets" never fails.
	tickets, total, err := p.GetMyTickets(args.UserId)
	if err != nil {
		return responseText(fmt.Sprintf("Could not load your tickets: %v", err)), nil
	}
	return renderTicketList("Tickets you requested", tickets, total, config), nil
}

func executeAssignedTickets(ctx context.Context, p PluginExecutor, config *ConfigView, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	glpiUserID, errResp := resolveGLPIUser(p, args.UserId)
	if errResp != nil {
		return errResp, nil
	}
	if glpiUserID <= 0 {
		// A user without an individual GLPI account cannot be an assignee.
		return responseText("### Tickets assigned to you\nNo tickets assigned."), nil
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

// resolveGLPIUser returns the user's GLPI user id (0 when no individual GLPI
// account exists). A 0 result is not an error: callers must fall back.
func resolveGLPIUser(p PluginExecutor, mattermostUserID string) (int, *model.CommandResponse) {
	glpiUserID, err := p.GetGLPIUserID(mattermostUserID)
	if err != nil {
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

	return renderTicketList(title, tickets, total, config), nil
}

// renderTicketList renders a ticket summary table (shared by GLPI searches and
// the identity-service ownership fallback).
func renderTicketList(title string, tickets []glpi.TicketSummary, total int, config *ConfigView) *model.CommandResponse {
	if len(tickets) == 0 {
		return responseText(fmt.Sprintf("### %s\nNo tickets found.", title))
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
	return responseText(b.String())
}

func escapePipes(text string) string {
	return strings.ReplaceAll(text, "|", "\\|")
}
