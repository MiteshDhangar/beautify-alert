# Architecture

## Overview

```
┌──────────────┐  labels   ┌───────────────────┐  POST + Bearer token   ┌────────────────────────────┐   SMTP   ┌───────┐
│ Alert rule   │──────────►│ Grafana alerting  │───────────────────────►│ Beautify Alert backend (Go)│─────────►│ Inbox │
└──────────────┘           │ webhook contact   │  /api/plugins/<id>/    │ routing ► render ► deliver │          └───────┘
                           │ point             │  resources/alert       └─────────────▲──────────────┘
                           └───────────────────┘                                      │ preview, test, log
                                                                         ┌────────────┴───────────────┐
                                                                         │ Plugin pages (React)       │
                                                                         │ Templates, Delivery log,   │
                                                                         │ Setup, Configuration       │
                                                                         └────────────────────────────┘
```

Grafana plugins cannot add contact point types, so alerts reach the plugin through Grafana's built-in webhook contact point. The webhook calls the plugin's resource endpoint on the same Grafana server. Grafana authenticates the call with a service account token and passes the caller's org role to the plugin.

Grafana starts the backend as a separate process and creates a new app instance each time the plugin settings are saved, so the backend always works with current settings and never watches for changes.

## Backend packages

| Package | Responsibility |
| --- | --- |
| `pkg/alert` | The webhook payload, which is also the template data. Sample data for previews. |
| `pkg/config` | Settings stored in the plugin's `jsonData` (`secureJsonData` for the SMTP password), defaults and validation. Templates are stored gzip-compressed in `templatesGz`. |
| `pkg/routing` | Splits one notification into emails: which template each alert uses and who receives it. |
| `pkg/templates` | Renders subject and HTML, inlines CSS, builds the plain-text part. Holds the starter templates. |
| `pkg/delivery` | The `Sender` interface and the SMTP sender. Threading headers. |
| `pkg/history` | The in-memory delivery log. |
| `pkg/notify` | Orchestration: plan, render, send, record. |
| `pkg/plugin` | Grafana wiring: app instance, HTTP routes, role checks. |

Dependencies point one way: `plugin` uses `notify`, which uses `routing`, `templates`, `delivery` and `history`, which use `alert` and `config`.

## HTTP endpoints

All under `/api/plugins/prudentbee939-beautifyalert-app/resources/`. Grafana handles authentication.

| Endpoint | Minimum role | Purpose |
| --- | --- | --- |
| `POST /alert` | Editor | Webhook target. Returns 502 when any email fails, which Grafana records as a failed notification. |
| `GET /starters` | Viewer | Starter templates and sample data |
| `POST /preview` | Editor | Renders an unsaved template |
| `POST /test` | Editor | Sends an unsaved template with sample data, to known addresses only for Editors, 10 per minute per organization |
| `GET /history`, `GET /history/{id}` | Editor | Delivery log |
| `POST /history/{id}/resend` | Editor | Sends a logged email again |
| `POST /verify` | Admin | Connects and logs in to the SMTP server without sending |

Saving settings and templates uses Grafana's plugin settings API (`POST /api/plugins/<id>/settings`), which requires the Admin role. The Setup page creates the service account and contact point through Grafana's HTTP API with the signed-in admin's session.

## Adding an email provider

1. Add a provider name and its settings to `pkg/config` (`Delivery`), secrets in `secureJsonData`.
2. Implement `delivery.Sender` (`Send` and `Verify`) in `pkg/delivery`, and return it from `delivery.New`.
3. Add the provider's fields to the Configuration page.

Callers do not change.

## Delivery rules

For each planned email:

1. Recipients that are not valid email addresses, or are outside the allowed recipient domains, are skipped and listed as a warning. At most *Max recipients per email* remain. If none is left, the email fails.
2. At most *Alerts per email* alerts are listed (default 40, firing first). The rest are counted in `.TruncatedAlerts`; `.FiringCount` and `.ResolvedCount` still count every alert.
3. If the template fails while rendering, the email is rendered with the built-in template and the error is recorded as a warning, so the alert still reaches people.
4. If the template has **Resolved-only emails** turned off and every alert in the email is resolved, nothing is sent and the log shows it as skipped.
5. One notification sends at most *Max emails per notification* emails. The rest are recorded as one failed log entry.

## Security

- Values from alerts are escaped by `html/template`. `safeHTML` is the only way to insert raw HTML, and templates are written by admins.
- Sprig functions that read the environment, reach the network or run slow cryptography are removed.
- Every render runs under limits (`pkg/templates/guard.go`): a tick at the start of each loop iteration and template call counts steps and checks a 5 second deadline, and every function is wrapped to refuse oversized arguments and results. See [template limits](templates.md#limits).
- At most 4 previews and test emails render at once across the plugin; alert emails do not wait for them.
- Viewers cannot preview or send email through the plugin. Test emails from Editors only go to known addresses, and each organization can send 10 a minute.
- **Test connection** sends the saved password only to the saved host, port and username with the same security, so it cannot be used to send the password elsewhere.
- SMTP errors never quote bytes from a server that is not an SMTP server, and control characters in server replies are removed.
- The preview frame is sandboxed without scripts or same-origin access.
- Request bodies are limited to 10 MB, rendered emails to 5 MB.
- The SMTP password is stored encrypted by Grafana and is never returned to the browser or written to logs.

## Known limits

- The delivery log is in memory: the last 500 emails per Grafana server, cleared on restart. With several Grafana servers behind a load balancer, each keeps its own log.
- Grafana limits a plugin's `jsonData` to 65,535 bytes on every database (`maxJSONDataBytes` in Grafana's plugin settings service). Templates are saved gzip-compressed, which fits about 50 typical templates. The Templates page shows how much is used and refuses to save past the limit with a clear message.
- Grafana's webhook contact point does not retry a failed call. The SMTP sender retries failures that may pass on a second try (refused connection, timeout, temporary 4xx reply) twice, after 1 and 3 seconds, within Grafana's 30 second webhook timeout. Wrong passwords, rejected recipients and TLS errors are not retried. Use **Resend** in the delivery log for anything that still failed.
