# Changelog

## 0.1.2

- Build with Go 1.26.11 (fixes GO-2026-6859 reported by govulncheck)
- Keep TLS certificate checks on unless the admin turns on "Skip certificate check" (gosec G402)

## 0.1.1

- Plugin ID is now `prudentbee939-beautifyalert-app`, matching the grafana.com organization
- Use `backend.UserFromContext` (fixes the backend lint check)
- Fill in the copyright line of the license

## 0.1.0

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
