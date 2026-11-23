# Beautify Alert

Send Grafana alerts as designed HTML emails.

Grafana's email contact point escapes HTML in notification templates, so alert emails all share one layout. Beautify Alert renders each alert with an HTML template you write and sends it through your SMTP server.

## Features

- HTML and CSS templates in Go template syntax, with CSS inlined for email clients
- A template per alert, picked with the `email_template` label
- Recipients per alert (`email_to` label) or per template
- Live preview with firing and resolved sample alerts, and test emails
- Starter templates: cards, compact table, summary, blank
- One-click setup of the service account and webhook contact point
- Delivery log with errors, email view and resend
- Firing and resolved emails threaded together in mail clients
- Templates provisioned from YAML

## Requirements

- Grafana 13.2 or later, self-hosted
- An SMTP server reachable from Grafana

## Getting started

1. Open the plugin's **Configuration** tab, enter your SMTP server and from address, click **Test connection**, then **Save settings**.
2. Open **Beautify Alert, Setup** and click **Create contact point**.
3. Pick the **Beautify Alert** contact point in an alert rule or a notification policy.
4. Create templates on the **Templates** page and add `email_template=<name>` to alert rules.

## Documentation

- [README and setup](https://github.com/MiteshDhangar/beautify-alert#readme)
- [Template reference](https://github.com/MiteshDhangar/beautify-alert/blob/main/docs/templates.md)
- [Report an issue](https://github.com/MiteshDhangar/beautify-alert/issues)
