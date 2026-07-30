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

The Mattermost **Site URL** must be set (System Console > Web Server) for the
create-ticket dialog to work.

### User mapping

Commands like `/glpi my` match the Mattermost account email against GLPI user
emails, cached for one hour. Users must have the same email in both systems.

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
