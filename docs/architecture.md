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
| `pkg/config` | Settings stored in the plugin's `jsonData` (`secureJsonData` for the SMTP password), defaults and validation. |
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
| `POST /alert` | Editor | Webhook target. Returns 502 when any email fails, so Grafana retries. |
| `GET /starters` | Viewer | Starter templates and sample data |
| `POST /preview` | Viewer | Renders an unsaved template |
| `POST /test` | Editor | Sends an unsaved template with sample data |
| `GET /history`, `GET /history/{id}` | Editor | Delivery log |
| `POST /history/{id}/resend` | Editor | Sends a logged email again |
| `POST /verify` | Admin | Connects and logs in to the SMTP server without sending |

Saving settings and templates uses Grafana's plugin settings API (`POST /api/plugins/<id>/settings`), which requires the Admin role. The Setup page creates the service account and contact point through Grafana's HTTP API with the signed-in admin's session.

## Adding an email provider

1. Add a provider name and its settings to `pkg/config` (`Delivery`), secrets in `secureJsonData`.
2. Implement `delivery.Sender` (`Send` and `Verify`) in `pkg/delivery`, and return it from `delivery.New`.
3. Add the provider's fields to the Configuration page.

Callers do not change.

## Security

- Values from alerts are escaped by `html/template`. `safeHTML` is the only way to insert raw HTML, and templates are written by admins.
- Sprig functions that read the environment (`env`, `expandenv`) or the network (`getHostByName`) are removed.
- Viewers cannot send email through the plugin.
- The preview frame is sandboxed without scripts or same-origin access.
- Request bodies are limited to 10 MB, rendered emails to 5 MB.
- The SMTP password is stored encrypted by Grafana and is never returned to the browser or written to logs.

## Known limits

- The delivery log is in memory: the last 500 emails per Grafana server, cleared on restart. With several Grafana servers behind a load balancer, each keeps its own log.
- Templates live in the plugin's `jsonData`. With a MySQL database, the column may limit total template size to about 64 KB. This has not been measured yet.
