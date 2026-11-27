# Troubleshooting

Start with **Beautify Alert, Delivery log**. Every email the plugin tried to send is there, with its result:

| Result | Meaning |
| --- | --- |
| Sent | The SMTP server accepted the email. |
| Sent, with warnings | Sent, but something needs attention. Hover or click **View**: a recipient was skipped, the template failed and the built-in one was used, or not every alert fit in the email. |
| Skipped | Not sent on purpose: every alert was resolved and the template has **Resolved-only emails** turned off. |
| Failed | Not sent. The error says why. Fix it, then click **Resend**. |

The log keeps the last 500 emails per organization and is cleared when Grafana restarts.

## No email, and nothing in the delivery log

The alert never reached the plugin.

1. **Is the alert routed to Beautify Alert?** Check the rule's contact point, or the notification policy that matches it (Alerting, Notification policies). The policy's matchers must match the alert's labels.
2. **Did Grafana send it?** Open Alerting, Contact points, **Beautify Alert**, and click **Test**. If Grafana reports an error:
   - `401 Unauthorized`: the token is wrong or was deleted. On the Setup page, click **Reconnect with a new token**.
   - `403 Forbidden`: the service account needs the Editor role.
   - `404 Not Found`: the plugin is not enabled, or the URL is wrong. It must end with `/api/plugins/prudentbee939-beautifyalert-app/resources/alert`.
   - Connection refused or timeout: the Grafana server cannot reach the URL. Use `http://localhost:3000/...` (or your sub-path) on the Setup page.
3. **Grafana's log:** look for `Notify for alerts failed` lines with `Beautify Alert`.
4. **Grafana's notification history** (Alerting, History, Notifications) shows each attempt, if enabled.

## "Failed" in the delivery log

| Error contains | Fix |
| --- | --- |
| `connection refused`, `i/o timeout` | The SMTP host or port is wrong, or a firewall blocks it. AWS EC2 blocks outgoing port 25 by default; use 587. The plugin already retried twice. |
| `does not support STARTTLS` | Use **TLS** for port 465, or **None** for a local relay without TLS. |
| `offered no login method` | The server offers no login on this connection. Use STARTTLS or TLS, or clear the username if the server needs no login. |
| `did not answer like an SMTP server` | The host and port reach something that is not an SMTP server, such as a database or web server. Check both. |
| `refused this many recipients` | The provider limits recipients per email. Lower **Max recipients per email** on the Configuration page. |
| `outside the allowed domains` | The recipient's domain is not in **Allowed recipient domains**. |
| `over the limit of N emails per notification` | One notification named more recipient lists than **Max emails per notification**. Raise it, or group recipients into fewer lists. |
| `rejected the username or password`, `535` | Wrong login. Gmail and Microsoft 365 need an app password; SendGrid's username is `apikey`. |
| `certificate`, `x509` | The server's certificate does not match the host name or is self-signed. Use the host name on the certificate, or turn on **Skip certificate check** for a trusted internal server. |
| `no valid recipients` | Every address was invalid. Check the `email_to` label, the template's recipients and the default recipients. |
| `no recipients` | Nobody to send to. Set default recipients on the Configuration page. |
| `550`, `553`, `sender rejected` | The SMTP provider does not allow this from address. Verify the address or domain with the provider. |

**Test connection** on the Configuration page checks the server and login without sending. To test a different host, port, username or security setting, type the password again: the saved password is only sent to the saved server.

Other errors:

| Error contains | Fix |
| --- | --- |
| `template went over a render limit` | The template loops too much or builds very large values. See [template limits](templates.md#limits). |
| `test emails from editors can only go to` | Send the test to yourself, the default recipients or a saved template's recipients, or ask an admin to add the domain to **Allowed recipient domains**. |
| `test emails in the last` | Each organization can send 10 test emails a minute. Wait and try again. |
| `too many previews and test emails are rendering` | Several people are previewing at once. Try again in a few seconds. |

## The email looks wrong

- **Styles missing in Gmail or Outlook:** keep **Inline CSS for email clients** on, and use tables for layout. See [styling for email clients](creating-templates.md#styling-for-email-clients).
- **"[Message clipped]" in Gmail:** the email is over about 102 KB. Lower **Alerts per email** on the Configuration page, or make the template smaller.
- **A field is empty:** the label or annotation does not exist on that alert. Labels are case-sensitive: `.Labels.Severity` is not `.Labels.severity`.
- **The default layout was used instead of mine:** the alert named a template that does not exist (check spelling), or your template failed on real data. The delivery log shows which, as a warning.
- **Firing and resolved emails are not grouped in Gmail:** Gmail also needs the subjects to match. Leave the status out of the subject if you want them in one conversation.

## Resolved emails at the end of a time window

If a rule's query only returns rows between, say, 22:20 and 22:30, every alert resolves at 22:30 and you get a resolved email for problems nobody fixed. Open the template and turn off **Resolved-only emails**.

## Saving templates fails

- **"Grafana stores at most 64 KB per plugin":** delete templates you no longer use, or shorten long ones. The Templates page shows storage used.
- **Save is disabled:** the preview shows an error; fix it first. Hover the button for the reason.
- **Only Grafana admins can save templates:** Editors can preview and send tests, but only admins can save.

## Settings disappear after a restart

Settings in a [provisioning file](provisioning.md) replace what was saved in the UI each time Grafana starts. Edit the file instead, or remove the plugin from it.

## Still stuck?

[Open an issue](https://github.com/MiteshDhangar/beautify-alert/issues/new/choose) with your Grafana version, the plugin version, the error from the delivery log, and the template if it is a template problem. Remove passwords and tokens first.
