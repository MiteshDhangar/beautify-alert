# Configuration

All settings are on the plugin's **Configuration** tab: Administration, Plugins and data, Plugins, Beautify Alert, Configuration. Only Grafana admins can change them. Each Grafana organization has its own settings.

## Sender

| Setting | Description |
| --- | --- |
| From address | Required. The address emails come from. Many providers only send from addresses or domains you have verified with them. |
| From name | Shown next to the address, for example `Grafana Alerts`. |
| Reply-to | Optional. Where replies go, for example the on-call team's address. |

## SMTP server

Beautify Alert has its own SMTP settings. It does not use the `[smtp]` section of `grafana.ini`.

| Setting | Description |
| --- | --- |
| Host | The SMTP server, for example `smtp.gmail.com`. |
| Port | Usually 587 with STARTTLS or 465 with TLS. The port changes with the security setting when it still has the default value. |
| Security | **STARTTLS** upgrades the connection to TLS and fails if the server does not support it. **TLS** connects with TLS from the start (port 465). **None** sends unencrypted, for local relays only. |
| Username, Password | Leave the username empty for servers that need no login. The password is stored encrypted. |
| Trusted certificate | Optional. For a server with a self-signed or private certificate, paste its certificate, or the CA that issued it, in PEM format. It is trusted in addition to the system's certificates. Certificates are always checked; there is no option to skip the check. |

**Test connection** connects, starts TLS and logs in, without sending an email. Errors say what to change, for example "the server does not support STARTTLS. Use TLS for port 465".

### SMTP providers

| Provider | Host | Port and security | Username | Password | Notes |
| --- | --- | --- | --- | --- | --- |
| Gmail / Google Workspace | `smtp.gmail.com` | 587 STARTTLS or 465 TLS | Full email address | An [app password](https://support.google.com/accounts/answer/185833) (needs 2-Step Verification) | About 2,000 emails a day. The from address must be the account or a verified alias. |
| Microsoft 365 | `smtp.office365.com` | 587 STARTTLS | Mailbox address | Mailbox password | SMTP AUTH must be enabled for the mailbox. Microsoft is retiring password login for SMTP; OAuth support is on the [roadmap](../ROADMAP.md). Until then, use a relay or another provider if your tenant blocks it. |
| Amazon SES | `email-smtp.<region>.amazonaws.com` | 587 STARTTLS or 465 TLS | SES SMTP username | SES SMTP password | Create SMTP credentials in the SES console (they are not your IAM access keys). The from address must be a verified identity. New accounts are in the sandbox and can only send to verified addresses. |
| SendGrid | `smtp.sendgrid.net` | 587 STARTTLS | `apikey` (literally) | A SendGrid API key | The from address must be a verified sender. |
| Mailgun | `smtp.mailgun.org` (EU: `smtp.eu.mailgun.org`) | 587 STARTTLS | `postmaster@<your domain>` | The domain's SMTP password | |
| Postmark | `smtp.postmarkapp.com` | 587 STARTTLS | Server API token | Server API token | |
| Local relay (Postfix, Exchange relay) | Relay host, for example `localhost` | 25 None, or 587 STARTTLS | Empty, unless the relay needs a login | | Use **None** only when the relay is on the same host or network. |

## Recipients and routing

| Setting | Default | Description |
| --- | --- | --- |
| Default recipients | | Who gets an email when neither the alert nor its template names recipients. Separate addresses with commas. |
| Template label | `email_template` | The alert label (or annotation) that picks a template. |
| Recipients label | `email_to` | The alert label (or annotation) that overrides recipients. |
| Alerts per email | 40 | The most alerts listed in one email. The rest are counted as "and N more". Gmail cuts off emails larger than about 102 KB, which is roughly 40 to 50 alerts with the default template. |
| Thread emails | On | Firing and resolved emails of one alert group get the same `References` header, so mail clients can show them as one conversation. Gmail also needs the subjects to match. |

How these combine: [routing alerts](routing.md).

## Limits

| Setting | Default | Description |
| --- | --- | --- |
| Allowed recipient domains | Any domain | Every email, alert or test, only goes to addresses in these domains, for example `example.com, example.org`. Subdomains are not included; list each one. Other recipients are skipped with a warning in the delivery log. |
| Max recipients per email | 50 | Extra recipients are skipped with a warning. Many providers refuse more than 50 to 100 per email. |
| Max emails per notification | 20 | One Grafana notification becomes one email per template and recipient list. When alerts in one group name more lists than this, the rest are not sent and the delivery log says so. |

Set allowed recipient domains if alert rules are written by people who should not be able to email outside your company: anyone who can edit alert rules can set the `email_to` label.

## Template settings

Each template on the **Templates** page has:

| Setting | Description |
| --- | --- |
| Name | What alerts put in the template label, for example `email_template=payments`. Lowercase letters, digits, `-` and `_`. |
| Description | Shown in the template list. |
| Recipients | Used when an alert has no recipients label. Empty: the default recipients. |
| Resolved-only emails | Turn off to skip emails in which every alert is resolved. Use it for rules whose query only returns rows during a time window: when the window ends, every alert resolves at once, and that email would wrongly say the problem is fixed. Skipped emails appear in the delivery log. |
| Subject | A one-line Go template. |
| HTML | The email body, a Go template. See [creating templates](creating-templates.md). |
| Inline CSS for email clients | On by default. Copies `<style>` rules into `style` attributes, because many email clients ignore `<style>`. |

Grafana stores at most 64 KB of settings per plugin. Templates are saved compressed, which fits about 50 typical templates. The Templates page shows how much is used.

## Permissions

| Action | Minimum Grafana role |
| --- | --- |
| See templates, preview | Editor |
| Send test emails, see the delivery log, resend | Editor. Editors can send test emails only to their own address, the default recipients, a saved template's recipients or an allowed recipient domain. Admins can send them to any address the allowed domains permit. Each organization can send 10 test emails a minute. |
| Change settings and templates, test the SMTP connection, Setup page | Admin |
| Webhook (the contact point's token) | Editor service account |
