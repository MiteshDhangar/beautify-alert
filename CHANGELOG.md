# Changelog

## 0.1.6

- Removed **Skip certificate check**. It turned off TLS certificate checks, so anyone on the network between Grafana and the SMTP server could read the SMTP password (gosec G402). Certificates are now always checked.
- New **Trusted certificate** setting for SMTP servers with a self-signed or private certificate: paste the server's certificate, or its CA, in PEM format.
- If **Skip certificate check** was on, the Configuration page and certificate errors say what to do instead.
- **Test connection** reuses the saved password only when the trusted certificate is also unchanged.

## 0.1.5

Security fixes from internal penetration testing. Upgrade recommended.

- Templates run under limits: 5 seconds, 1,000,000 loop iterations and template calls, 1 MB per value and 64 MB per render. Before, one preview could keep every CPU core busy for minutes or use gigabytes of memory. See [template limits](docs/templates.md#limits).
- Removed Sprig's slow cryptography functions (`genPrivateKey`, `bcrypt` and others). In HTML templates, `html` and `urlquery` can only end an output.
- Preview needs the Editor role, like the Templates page. At most 4 previews and test emails render at once.
- Test emails from Editors only go to their own address, the default recipients, a saved template's recipients or an allowed domain. Each organization can send 10 test emails a minute.
- **Test connection** only sends the saved SMTP password to the saved host, port and username with the same security. Before, an admin could send it to a server they run.
- New settings: allowed recipient domains, max recipients per email (default 50) and max emails per notification (default 20).
- SMTP errors no longer quote bytes from servers that are not SMTP servers, and remove control characters from server replies.
- Fix: "too many recipients" was reported as a wrong password and retried.
- Removed AI assistant files from the repository.

## 0.1.4

- Fix: the delivery log was shared by all Grafana organizations; each organization now has its own.
- New docs: getting started, configuration with SMTP settings for common providers, routing, creating templates, troubleshooting.
- New examples: a template for SQL rules that return one row per problem, and a provisioning file.
- New screenshots, issue and pull request templates, code of conduct.

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
