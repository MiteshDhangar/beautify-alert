# Roadmap

Each release works on its own. Order can change with feedback: open a [discussion](https://github.com/MiteshDhangar/beautify-alert/discussions) or upvote one.

## 0.1: HTML emails (this release)

- HTML templates per alert, picked with a label; per-template and per-alert recipients
- Live preview, test emails, starter templates
- SMTP with STARTTLS, TLS or none; connection test
- One-click setup of the service account and contact point
- Delivery log with resend; email threading; CSS inlining; YAML provisioning

## 0.2: Any email provider

Microsoft 365 no longer accepts password login for SMTP, and Grafana's SMTP settings have no OAuth option ([grafana/grafana#112802](https://github.com/grafana/grafana/issues/112802)).

- Microsoft 365 through the Microsoft Graph API (client credentials)
- Gmail with OAuth; SMTP with XOAUTH2
- Amazon SES, SendGrid, Mailgun, Postmark
- Template import and export as JSON; a default time zone per template
- Paste a real alert payload to preview with your own data

## 0.3: Data in emails

Grafana does not add panel images to emails that use custom templates, and screenshots need the image renderer service.

- Tables of query results in the email
- Small charts drawn by the plugin, without the image renderer
- Optional CSV attachment of the alert's data
- Panel screenshots when the image renderer is installed

## 0.4: Chat messages

- The same editor and preview for Slack Block Kit, Microsoft Teams Adaptive Cards and Discord embeds

## 1.0: Actions and noise control

- Signed, expiring links to silence or acknowledge an alert from the email
- Quiet hours per recipient
- Rate limits per template
- Multiple Grafana organizations
- A delivery log stored in the Grafana database, shared by all Grafana servers

## Not planned

- Scheduled PDF or CSV reports of dashboards. Grafana Enterprise has this, and the Grafana plugin policy does not allow duplicating it.
- Usage tracking. The plugin collects nothing about its users.
