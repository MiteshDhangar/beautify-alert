# Getting started

This guide takes you from nothing to your first designed alert email in about 10 minutes. You need:

- Grafana 13.2 or later, self-hosted (OSS or Enterprise), and the Admin role in it.
- An SMTP server Grafana can reach: your company's mail relay, or a service such as Gmail, Amazon SES, SendGrid or Mailgun. See [SMTP providers](configuration.md#smtp-providers).

## Install

Plugin ID: `prudentbee939-beautifyalert-app`.

### From the Grafana plugin catalog

Once the plugin is published in the catalog:

1. In Grafana, open **Administration, Plugins and data, Plugins**.
2. Search for **Beautify Alert** and click **Install**.

Or on the Grafana server:

```bash
grafana cli plugins install prudentbee939-beautifyalert-app
sudo systemctl restart grafana-server
```

### From a GitHub release

Use this before the catalog version is available, or on servers without access to grafana.com. Pick the version from the [releases page](https://github.com/MiteshDhangar/beautify-alert/releases).

```bash
sudo grafana cli \
  --pluginUrl https://github.com/MiteshDhangar/beautify-alert/releases/download/v0.1.3/prudentbee939-beautifyalert-app-0.1.3.zip \
  plugins install prudentbee939-beautifyalert-app
```

Release builds are not signed until the catalog review is done, so allow the plugin in `grafana.ini` and restart Grafana:

```ini
[plugins]
allow_loading_unsigned_plugins = prudentbee939-beautifyalert-app
```

### Docker

```bash
docker run -d -p 3000:3000 --name=grafana \
  -e "GF_PLUGINS_PREINSTALL=prudentbee939-beautifyalert-app@0.1.3@https://github.com/MiteshDhangar/beautify-alert/releases/download/v0.1.3/prudentbee939-beautifyalert-app-0.1.3.zip" \
  -e "GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=prudentbee939-beautifyalert-app" \
  grafana/grafana:latest
```

Once the plugin is in the catalog, `GF_PLUGINS_PREINSTALL=prudentbee939-beautifyalert-app` is enough.

### Check the install

Open **Administration, Plugins and data, Plugins, Beautify Alert**. If it is not enabled yet, click **Enable**. **Beautify Alert** then appears under **More apps** in the menu.

## 1. Configure email

Open the plugin's **Configuration** tab (Administration, Plugins and data, Plugins, Beautify Alert, Configuration).

1. **Sender:** the address emails come from, for example `alerts@yourcompany.com`, and a name such as `Grafana Alerts`.
2. **SMTP server:** host, port, security (STARTTLS for port 587, TLS for 465), username and password. Copy-paste values for common providers are in [configuration](configuration.md#smtp-providers).
3. **Default recipients:** who gets alerts that name no recipients, for example `oncall@yourcompany.com`.
4. Click **Test connection**. It connects and logs in without sending anything.
5. Click **Save settings**.

The password is stored encrypted by Grafana and is never shown again.

## 2. Connect Grafana alerting

Open **Beautify Alert, Setup** and click **Create contact point**. This creates:

- a service account `beautify-alert` with the Editor role,
- a token for it,
- a webhook contact point **Beautify Alert** that posts alerts to the plugin with that token.

The webhook URL defaults to Grafana's public address. If the Grafana server cannot reach its own public URL (for example behind a load balancer, a VPN or a login proxy), change it to one it can reach, usually:

```
http://localhost:3000/api/plugins/prudentbee939-beautifyalert-app/resources/alert
```

If Grafana runs under a sub-path, include it: `http://localhost:3000/grafana/api/plugins/...`.

To set this up by hand instead, see the "Set it up by hand" section on the Setup page.

## 3. Send your first email

1. Open **Alerting, Contact points**, edit **Beautify Alert**, and click **Test**, then **Send test notification**.
2. Check your inbox (the default recipients) for **[FIRING:1] TestAlert**.
3. Open **Beautify Alert, Delivery log** to see the email and its delivery result.

No email? See [troubleshooting](troubleshooting.md).

## 4. Route real alerts

Choose which alerts go through Beautify Alert. Either:

- **In an alert rule:** in **Configure notifications**, select the contact point **Beautify Alert**.
- **In a notification policy:** Alerting, Notification policies, add a child policy with a matcher such as `team = ops` and the contact point **Beautify Alert**.

Your other contact points keep working. To try it next to your current emails first, add the policy with **Continue matching** turned on, so both emails are sent.

More patterns: [routing alerts](routing.md).

## 5. Make it yours

1. Open **Beautify Alert, Templates** and click **New template**.
2. Name it, for example `payments`, pick a starter, and click **Create template**.
3. Edit the HTML on the left; the preview on the right updates as you type. Switch the preview between **Firing** and **Resolved**.
4. Click **Send test email**, then **Save template**.
5. Add the label `email_template=payments` to the alert rules that should use it.

Next: [creating templates](creating-templates.md).
