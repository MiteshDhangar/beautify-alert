# Changelog

## 0.1.0 (unreleased)

First release.

- HTML email templates, picked per alert with the `email_template` label
- Recipients from the `email_to` label, the template, or the default list
- Template editor with live preview, sample firing and resolved alerts, and test emails
- Starter templates: cards, compact table, summary, blank
- SMTP delivery with STARTTLS, TLS or no encryption, and a connection test
- Setup page that creates the service account, token and webhook contact point
- Delivery log with error details, email view and resend
- Threading headers that group the emails of one alert group
- CSS inlining for email clients
- Settings and templates provisioning from YAML
