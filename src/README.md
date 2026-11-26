# Beautify Alert

Send Grafana alerts as designed HTML emails.

Grafana's email contact point escapes HTML in notification templates, so every alert email looks the same. Beautify Alert renders alerts with HTML templates you write, picks a template per alert with a label, and sends them through your SMTP server. It works with any Grafana alert rule and any SMTP provider.

## Features

- HTML and CSS templates in Go template syntax, with CSS inlined for email clients
- A template per alert, picked with the `email_template` label
- Recipients per alert (`email_to` label), per template, or a default list
- Live preview with firing and resolved sample alerts, and test emails
- Starter templates: cards, compact table, summary, blank
- One-click setup of the service account and webhook contact point
- Delivery log with errors, warnings, email view and resend
- Safe delivery: invalid addresses skipped, broken templates fall back to the built-in layout, short SMTP failures retried
- Query failures name the rule and show the error instead of `DatasourceError`
- Large alert groups capped per email so Gmail does not cut them off
- Resolved-only emails can be turned off per template, for time-window rules
- Templates provisioned from YAML; settings and log per organization

## Requirements

- Grafana 13.2 or later, self-hosted (OSS or Enterprise)
- An SMTP server reachable from Grafana (Gmail, Amazon SES, SendGrid, Mailgun, Postmark, Microsoft 365 or your own relay)

## Getting started

1. Open the plugin's **Configuration** tab, enter your SMTP server and from address, click **Test connection**, then **Save settings**.
2. Open **Beautify Alert, Setup** and click **Create contact point**.
3. Pick the **Beautify Alert** contact point in an alert rule or a notification policy.
4. Create templates on the **Templates** page and add `email_template=<name>` to alert rules.

## Documentation

- [Getting started](https://github.com/MiteshDhangar/beautify-alert/blob/main/docs/getting-started.md)
- [Configuration and SMTP providers](https://github.com/MiteshDhangar/beautify-alert/blob/main/docs/configuration.md)
- [Routing alerts](https://github.com/MiteshDhangar/beautify-alert/blob/main/docs/routing.md)
- [Creating templates](https://github.com/MiteshDhangar/beautify-alert/blob/main/docs/creating-templates.md)
- [Template reference](https://github.com/MiteshDhangar/beautify-alert/blob/main/docs/templates.md)
- [Troubleshooting](https://github.com/MiteshDhangar/beautify-alert/blob/main/docs/troubleshooting.md)
- [Report an issue](https://github.com/MiteshDhangar/beautify-alert/issues)
