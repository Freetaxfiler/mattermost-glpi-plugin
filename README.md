# Mattermost GLPI Plugin

GLPI IT support inside Mattermost. Employees create, track, and update GLPI
tickets, look up their assets, and search the knowledge base without leaving
Mattermost. Targets **Mattermost Team Edition 10.11.x** and **GLPI 11.x**
(REST API).

## Features

- `/glpi status` — connectivity + GLPI version check
- `/glpi create` — interactive dialog: subject, description, priority, urgency, category
- `/glpi my` / `/glpi assigned` — your requested / assigned tickets
- `/glpi search <text>` — search tickets by title
- `/glpi view <id>` — full ticket details
- `/glpi comment|private <id> <text>` — public / private follow-ups
- `/glpi update <id> priority|urgency|status|title <value>` — update a ticket
- `/glpi close <id> [solution]` — record a solution and close
- `/glpi reopen <id>` / `/glpi delete <id>`
- `/glpi attach <id>` — attach your latest uploaded file in the channel to a ticket
- `/glpi assets [type] [search]` — computers, printers, monitors, network, software, licenses
- `/glpi kb <text>` — knowledge base search
- `/glpi admin` — diagnostics (system admins only)
- Webhook endpoint for GLPI notifications → channel post and/or DM to the requester

- `/glpi timeline <id> [page]` — paginated follow-ups, solutions, validations, and ticket history

## Build

Requires Go (see `go.mod` for the version).

```bash
make dist      # builds server binary and packages dist/mattermost-glpi-plugin-<version>.tar.gz
make test      # runs unit tests
```

Upload the tarball in **System Console > Plugins > Plugin Management**.

## Configuration

In **System Console > Plugins > GLPI**:

| Setting | Description |
|---|---|
| GLPI Server URL | e.g. `https://glpi.example.com` |
| App Token | GLPI Setup > General > API |
| User Token | API user's preferences > Remote access keys |
| Default Entity ID | numeric entity for new tickets (optional) |
| Default Category ID | numeric ITIL category for new tickets (optional) |
| Notification Channel ID | channel that receives webhook notifications (optional) |
| Webhook Secret | shared secret for the webhook endpoint (generated) |
| Enable User Mapping | Mode B: attribute tickets to the GLPI user matching the requester's email (default: off) |

The Mattermost **Site URL** must be set (System Console > Web Server) for the
create-ticket dialog to work.

### Enterprise identity layer

The plugin routes all user↔GLPI identity lookups through a centralized layer
(`server/identity/`) with **two operating modes**:

- **Mode A (default):** every API request runs under the GLPI integration
  account. The real Mattermost employee is preserved as HTML metadata appended
  to the ticket description (user ID, username, display name, email, team,
  channel). Ticket creation **never fails** because an employee has no GLPI
  account.
- **Mode B (`Enable User Mapping` in System Console):** the plugin looks up the
  mapped GLPI user for each Mattermost user and files tickets under that
  requester. Discovery priority is **email → username → display name**.
  Unmatched users fall back to Mode A automatically.

Mappings are **persistent** in plugin KV and indexed by Mattermost user ID,
email, GLPI user ID, and GLPI login. When mapping is enabled, "My Tickets"
reads the requester's mapped tickets; when it is not, it reads the
identity-service ownership record so the feature works without GLPI accounts.

The mapped GLPI user's profiles determine the plugin **role**
(`employee` / `technician` / `supervisor` / `manager` / `administrator`), which
drives the visible UI: the "Assign to me" technician action is hidden for
employees.

> **Note:** in the default Mode A, role detection is inactive and the "Assign
> to me" action is hidden for non-admins. Enable *Map Mattermost Users* and give
> technicians a GLPI profile (`Technician`, `Hotliner`, `Read-Only`, …) to
> restore role-driven UI.

### Admin user-mapping page

Mattermost **System Admins** get an **Admin** tab in the sidebar:

- **Mapped users** table (Mattermost user ↔ GLPI login, profiles, role, last sync)
- **Unmapped users** with a *Provision GLPI Account* action (creates the GLPI
  user, never duplicates)
- **Duplicate emails** detection
- **Sync Users** (re-runs automatic discovery for every unmapped user),
  **Clear Cache**, and **Refresh**

### Notifications from GLPI

Configure a GLPI webhook (GLPI 11: Setup > Webhooks) to POST JSON to:

```
<mattermost-site-url>/plugins/com.ntas.glpi/webhook?secret=<webhook secret>
```

The payload is parsed leniently; recognized keys: `event`, `ticket_id` (or
`items_id`/`id`), `title`/`name`, `status`, `url`, `requester_email`. If
`requester_email` matches a Mattermost account, that user receives a DM from
the GLPI bot; the notification channel (if configured) also gets a post.

### GLPI search field IDs

Ticket/user/asset lookups use GLPI's standard search option IDs (see
`server/glpi/search.go`). If your GLPI instance customizes search options,
adjust the constants there.

### Ticket timelines

`/glpi timeline <id> [page]` combines the visible GLPI follow-ups, solutions,
validation records, and history records in newest-first order. Pagination is
derived from GLPI's REST `Content-Range` headers. The plugin never bypasses
GLPI access control: private events are rendered only for Mattermost system
administrators. This conservative restriction prevents a shared GLPI API token
from exposing private information before technician role mapping is configured.

### Solution workflow

With solution text, `/glpi close <id> <solution>` records a GLPI solution and
lets GLPI 11 apply the configured solved, requester-approval, rejection, and
automatic-closing workflow. Use `/glpi close <id>` only for a direct closure
permitted by the active GLPI lifecycle profile.
