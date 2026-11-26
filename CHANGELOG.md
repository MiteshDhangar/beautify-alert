# Changelog

## 0.1.3

Found in pre-release testing with a real MySQL alert rule.

- Templates are stored compressed. Grafana limits plugin settings to 64 KB, which allowed only about 5 to 15 templates; compressed, about 50 fit. The Templates page shows storage used and explains the limit instead of failing with a 400 error.
- Invalid addresses in `email_to` or template recipients are skipped with a warning; valid recipients still get the email.
- A template that fails while rendering real alerts falls back to the built-in template, so the email is still sent. Templates with a preview error cannot be saved.
- Grafana's webhook does not retry, so the SMTP sender now retries refused connections, timeouts and temporary replies twice.
- `DatasourceError` and `DatasourceNoData` alerts name the rule ("Disk usage (query failed)") and show the error. New template fields: `.Name`, `.Problem`, `.Error`, `.FiringCount`, `.ResolvedCount`.
- New setting: alerts per email (default 40), so large groups stay under Gmail's 102 KB cut-off; the rest are counted as "and N more".
- New per-template switch: resolved-only emails, for rules whose query only returns rows in a time window.
- The delivery log shows warnings and skipped emails.
- Clearer SMTP errors for login, STARTTLS and wrong passwords.
- Renaming a template asks for confirmation, because alert rules using the old name fall back to the default.

If you saved your own copy of a starter template, create a new template from the starter to get the error-alert handling.

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
