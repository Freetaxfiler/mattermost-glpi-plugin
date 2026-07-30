# Changelog

## Unreleased

- Added paginated `/glpi timeline <ticket id> [page]` output that merges GLPI
  follow-ups, solutions, validations, and ticket history using the legacy REST
  API collection pagination headers.
- Made webhook fingerprint deduplication atomic across Mattermost cluster
  nodes, and ensure malformed webhook bodies are rejected before deduplication.
- Use cryptographically strong GLPI request correlation IDs.
- Respect GLPI's solution approval lifecycle: adding a solution now leaves the
  solved/approval transition to GLPI instead of forcibly closing the ticket.
## 0.2.0

- Ticket management commands: `my`, `assigned`, `search`, `view`, `update`, `close`, `reopen`, `delete`
- Public and private follow-ups (`comment`, `private`) and solutions on close
- Attachments: `/glpi attach <id>` uploads the user's latest channel file to the ticket
- Assets (`/glpi assets`) and knowledge base (`/glpi kb`) search
- `/glpi admin` diagnostics for system admins; `/glpi help` with full command table
- Slash-command autocomplete for all subcommands
- GLPI webhook endpoint (`/plugins/com.ntas.glpi/webhook`) with shared-secret validation; posts to a configured channel and DMs the requester via the GLPI bot
- Mattermost↔GLPI user mapping by email with 1-hour KV cache
- Dialog gained urgency + category fields; requester, default entity, and default category applied to new tickets
- Dialog callback URL now derived from the Mattermost Site URL (previously hardcoded)
- GLPI client: thread-safe session handling, automatic re-authentication on expired sessions, shared request layer, typed not-found errors
- New settings: Default Category ID, Notification Channel ID, Webhook Secret
- Unit tests for search parsing, user lookup, and HTML stripping

## 0.1.0

- Initial skeleton: configuration, GLPI session handling, health check, `/glpi status`, create-ticket dialog
