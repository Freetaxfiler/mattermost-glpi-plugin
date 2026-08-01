package commands

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Freetaxfiler/mattermost-glpi-plugin/server/glpi"
	"github.com/mattermost/mattermost/server/public/model"
)

// PluginVersion is injected at build time via -ldflags.
var PluginVersion = "0.2.0"

const Trigger = "glpi"

// ConfigView is the subset of plugin configuration needed by the command layer.
type ConfigView struct {
	GLPIURL               string
	AppToken              string
	UserToken             string
	DefaultEntity         string
	DefaultCategory       string
	NotificationChannelID string
	EnableDebug           bool
}

// PluginExecutor exposes the minimal plugin surface needed by commands.
type PluginExecutor interface {
	GetConfiguration() *ConfigView
	GetGLPIClient() glpi.GLPIClient

	OpenCreateTicketDialog(args *model.CommandArgs) error

	// GetGLPIUserID resolves the GLPI user ID for a Mattermost user (by email).
	// Returns 0 when no individual GLPI user exists (integration mode).
	GetGLPIUserID(mattermostUserID string) (int, error)

	// GetMyTickets returns the Mattermost user's owned tickets from the
	// identity-service ownership mapping (used when no GLPI user exists).
	GetMyTickets(mattermostUserID string) ([]glpi.TicketSummary, int, error)

	// IsSystemAdmin reports whether the Mattermost user is a system admin.
	IsSystemAdmin(mattermostUserID string) bool

	// LatestFileAttachment returns the most recent file the user posted in the
	// given channel (filename and content).
	LatestFileAttachment(mattermostUserID, channelID string) (string, []byte, error)
}

// GetCommand returns the slash command definition for the GLPI plugin.
func GetCommand() *model.Command {
	return &model.Command{
		Trigger:          Trigger,
		DisplayName:      "GLPI",
		Description:      "Interact with GLPI IT support without leaving Mattermost",
		AutoComplete:     true,
		AutoCompleteDesc: "Manage GLPI tickets, assets, and knowledge base",
		AutoCompleteHint: "[command]",
		AutocompleteData: buildAutocomplete(),
	}
}

func buildAutocomplete() *model.AutocompleteData {
	root := model.NewAutocompleteData(Trigger, "[command]", "Interact with GLPI IT support")

	help := model.NewAutocompleteData("help", "", "Show all available GLPI commands")
	root.AddCommand(help)

	status := model.NewAutocompleteData("status", "", "Check GLPI connectivity and version")
	root.AddCommand(status)

	create := model.NewAutocompleteData("create", "", "Open a dialog to create a new ticket")
	root.AddCommand(create)

	my := model.NewAutocompleteData("my", "", "List tickets you requested")
	root.AddCommand(my)

	assigned := model.NewAutocompleteData("assigned", "", "List tickets assigned to you")
	root.AddCommand(assigned)

	search := model.NewAutocompleteData("search", "[text]", "Search tickets by title")
	search.AddTextArgument("Text to search for in ticket titles", "[text]", "")
	root.AddCommand(search)

	view := model.NewAutocompleteData("view", "[ticket id]", "View a ticket's details")
	view.AddTextArgument("Ticket ID", "[ticket id]", "")
	root.AddCommand(view)

	comment := model.NewAutocompleteData("comment", "[ticket id] [text]", "Add a public follow-up to a ticket")
	comment.AddTextArgument("Ticket ID followed by the follow-up text", "[ticket id] [text]", "")
	root.AddCommand(comment)

	private := model.NewAutocompleteData("private", "[ticket id] [text]", "Add a private follow-up to a ticket")
	private.AddTextArgument("Ticket ID followed by the follow-up text", "[ticket id] [text]", "")
	root.AddCommand(private)

	update := model.NewAutocompleteData("update", "[ticket id] [field] [value]", "Update a ticket (priority, urgency, status, title)")
	update.AddTextArgument("Ticket ID, field (priority|urgency|status|title), and new value", "[ticket id] [field] [value]", "")
	root.AddCommand(update)

	closeCmd := model.NewAutocompleteData("close", "[ticket id] [solution]", "Close a ticket, optionally recording a solution")
	closeCmd.AddTextArgument("Ticket ID and optional solution text", "[ticket id] [solution]", "")
	root.AddCommand(closeCmd)

	reopen := model.NewAutocompleteData("reopen", "[ticket id]", "Reopen a closed or solved ticket")
	reopen.AddTextArgument("Ticket ID", "[ticket id]", "")
	root.AddCommand(reopen)

	deleteCmd := model.NewAutocompleteData("delete", "[ticket id]", "Move a ticket to the GLPI trash")
	deleteCmd.AddTextArgument("Ticket ID", "[ticket id]", "")
	root.AddCommand(deleteCmd)

	attach := model.NewAutocompleteData("attach", "[ticket id]", "Attach your most recent file in this channel to a ticket")
	attach.AddTextArgument("Ticket ID", "[ticket id]", "")
	root.AddCommand(attach)

	assets := model.NewAutocompleteData("assets", "[type] [search]", "List your assets (computers, printers, monitors, network, software, licenses)")
	assets.AddTextArgument("Asset type and optional name search", "[type] [search]", "")
	root.AddCommand(assets)

	kb := model.NewAutocompleteData("kb", "[text]", "Search the GLPI knowledge base")
	kb.AddTextArgument("Text to search for", "[text]", "")
	root.AddCommand(kb)

	admin := model.NewAutocompleteData("admin", "", "Show plugin diagnostics (system admins only)")
	root.AddCommand(admin)

	return root
}

// ExecuteCommand executes the /glpi slash command and returns an ephemeral response.
func ExecuteCommand(p PluginExecutor, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	config := p.GetConfiguration()
	if config == nil {
		return responseText("GLPI configuration is unavailable."), nil
	}

	// generate a short correlation id for this command execution
	reqID := fmt.Sprintf("cmd-%d-%d", time.Now().Unix(), time.Now().UnixNano()%100000)
	baseCtx := context.WithValue(context.Background(), "request_id", reqID)
	// attach user id for per-user rate limiting and auditing
	baseCtx = context.WithValue(baseCtx, "user_id", args.UserId)

	commandText := strings.TrimSpace(args.Command)
	commandParts := strings.Fields(commandText)

	subcommand := ""
	if len(commandParts) > 1 {
		subcommand = strings.ToLower(commandParts[1])
	}
	var rest []string
	if len(commandParts) > 2 {
		rest = commandParts[2:]
	}

	switch subcommand {
	case "", "help":
		return responseText(helpText(config)), nil

	case "status":
		return executeStatus(baseCtx, p, config)

	case "create":
		if err := p.OpenCreateTicketDialog(args); err != nil {
			return responseText(fmt.Sprintf("Unable to open dialog:\n%s", err.Error())), nil
		}
		return &model.CommandResponse{}, nil

	case "my":
		return executeMyTickets(baseCtx, p, config, args)

	case "assigned":
		return executeAssignedTickets(baseCtx, p, config, args)

	case "search":
		return executeSearchTickets(baseCtx, p, config, rest)

	case "view":
		return executeViewTicket(baseCtx, p, config, rest)

	case "comment":
		return executeFollowup(baseCtx, p, rest, false)

	case "private":
		return executeFollowup(baseCtx, p, rest, true)

	case "update":
		return executeUpdateTicket(baseCtx, p, rest)

	case "close":
		return executeCloseTicket(baseCtx, p, rest)

	case "reopen":
		return executeReopenTicket(baseCtx, p, rest)

	case "delete":
		return executeDeleteTicket(baseCtx, p, rest)

	case "attach":
		return executeAttach(baseCtx, p, args, rest)

	case "assets":
		return executeAssets(baseCtx, p, args, rest)

	case "kb":
		return executeKnowledge(baseCtx, p, rest)

	case "admin":
		return executeAdmin(baseCtx, p, config, args)

	default:
		return responseText(fmt.Sprintf("Unknown command `%s`.\n\n%s", subcommand, helpText(config))), nil
	}
}

func helpText(config *ConfigView) string {
	var b strings.Builder
	b.WriteString("### GLPI commands\n")
	b.WriteString("| Command | Description |\n")
	b.WriteString("|---|---|\n")
	b.WriteString("| `/glpi status` | Check GLPI connectivity and version |\n")
	b.WriteString("| `/glpi create` | Create a new ticket via a dialog |\n")
	b.WriteString("| `/glpi my` | List tickets you requested |\n")
	b.WriteString("| `/glpi assigned` | List tickets assigned to you |\n")
	b.WriteString("| `/glpi search <text>` | Search tickets by title |\n")
	b.WriteString("| `/glpi view <id>` | View a ticket's details |\n")
	b.WriteString("| `/glpi comment <id> <text>` | Add a public follow-up |\n")
	b.WriteString("| `/glpi private <id> <text>` | Add a private follow-up |\n")
	b.WriteString("| `/glpi update <id> <priority\\|urgency\\|status\\|title> <value>` | Update a ticket field |\n")
	b.WriteString("| `/glpi close <id> [solution]` | Close a ticket, optionally with a solution |\n")
	b.WriteString("| `/glpi reopen <id>` | Reopen a ticket |\n")
	b.WriteString("| `/glpi delete <id>` | Move a ticket to the trash |\n")
	b.WriteString("| `/glpi attach <id>` | Attach your latest file in this channel to a ticket |\n")
	b.WriteString("| `/glpi assets [type] [search]` | List your assets (computers, printers, monitors, network, software, licenses) |\n")
	b.WriteString("| `/glpi kb <text>` | Search the knowledge base |\n")
	b.WriteString("| `/glpi admin` | Plugin diagnostics (system admins only) |\n")

	if strings.TrimSpace(config.GLPIURL) == "" {
		b.WriteString("\n:warning: GLPI is not configured yet. Set the GLPI URL, App Token, and User Token in **System Console > Plugins > GLPI**.")
	}
	b.WriteString(fmt.Sprintf("\nPlugin version: %s", PluginVersion))
	return b.String()
}

var (
	userRateMu       sync.Mutex
	userLastRequest  = map[string]time.Time{}
	userRateWindow   = 2 * time.Second // simple per-user window for heavy commands
	rateLimitCalls   int64
	rateLimitSweepMu sync.Mutex
)

func responseText(text string) *model.CommandResponse {
	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         text,
	}
}

// checkAndRecordUserRate returns true if the user is allowed to perform a heavy command,
// and records the current time as the last request time when allowed. Stale entries are
// pruned periodically (every 128 calls) to prevent unbounded map growth.
func checkAndRecordUserRate(ctx context.Context) bool {
	uid, _ := ctx.Value("user_id").(string)
	if uid == "" {
		return true
	}
	userRateMu.Lock()
	defer userRateMu.Unlock()

	// Periodic sweep: prune entries idle for more than 10 minutes.
	if rateLimitCalls%128 == 0 {
		cutoff := time.Now().Add(-10 * time.Minute)
		for uid, last := range userLastRequest {
			if last.Before(cutoff) {
				delete(userLastRequest, uid)
			}
		}
	}
	rateLimitCalls++

	last, ok := userLastRequest[uid]
	if ok && time.Since(last) < userRateWindow {
		return false
	}
	userLastRequest[uid] = time.Now()
	return true
}