# Creating templates

A template is an email: a one-line **subject** and an **HTML body**. Both are [Go templates](https://pkg.go.dev/text/template), the same language as Grafana's notification templates. This guide builds a template step by step; every field and function is listed in the [template reference](templates.md).

## The editor

Open **Beautify Alert, Templates**.

- **New template** asks for a name and a starting point: **Cards** (the default layout), **Compact table**, **Summary** (big counts, for stakeholders) or **Blank**.
- The **preview** on the right renders the template with sample alerts as you type. Switch between **Firing** and **Resolved**.
- **Insert field** adds common fields at the cursor, and the editor suggests them as you type `{{`.
- **Send test email** sends the current, unsaved template with sample data. It goes to the address you type, or to the template's recipients.
- **Save template** is disabled while the preview shows an error.

The sample alerts are a CPU alert. If your rule has other labels (for example `order_id`), those fields show empty in the preview but fill in for real alerts.

## How data reaches the template

Grafana groups alerts into notifications. Each email gets:

- `.` = the notification: `.Name`, `.Status`, `.FiringCount`, `.CommonLabels`, `.ExternalURL`, and the list `.Alerts`.
- Inside `{{ range .Alerts }} ... {{ end }}`, `.` = one alert: `.Status`, `.Labels`, `.Annotations`, `.StartsAt`, `.Values`, `.DashboardURL` and more.

Values from alerts are HTML-escaped automatically. An annotation that contains `<b>` shows as text.

## Step by step: an incident email

### 1. A heading and a count

```html
<h1>{{ .Name }}</h1>
<p>{{ .FiringCount }} firing, {{ .ResolvedCount }} resolved</p>
```

`.Name` is the alert rule's name. For Grafana's error alerts it reads "Disk usage (query failed)" instead of `DatasourceError`.

### 2. One block per alert

```html
{{ range .Alerts }}
  <h3>{{ .Annotations.summary | default .Name }}</h3>
  <p>{{ .Annotations.description }}</p>
  <p>Instance: {{ .Labels.instance }}, started {{ since .StartsAt }} ago</p>
{{ end }}
```

`default` is a [Sprig](https://masterminds.github.io/sprig/) function: it shows the alert name when there is no summary.

### 3. Colors by status and severity

```html
{{ range .Alerts }}
  <div style="border-left: 4px solid {{ statusColor .Status }}; padding: 8px 12px;">
    <span style="color: {{ severityColor .Labels.severity }}; font-weight: 700;">{{ .Labels.severity | upper }}</span>
    {{ .Annotations.summary }}
  </div>
{{ end }}
```

`statusColor` is red for firing and green for resolved. `severityColor` maps critical, high, warning, info and similar values to red, orange, amber and blue.

### 4. Different content when resolved

```html
{{ if eq .Status "firing" }}
  <h2 style="color: #dc2626;">Action needed</h2>
{{ else }}
  <h2 style="color: #16a34a;">All clear</h2>
{{ end }}
```

The subject can do the same:

```
{{ if eq .Status "firing" }}[Action needed]{{ else }}[Resolved]{{ end }} {{ .Name }}
```

### 5. Links

```html
{{ range .Alerts }}
  {{ with .DashboardURL }}<a href="{{ . }}">Open dashboard</a>{{ end }}
  {{ with .GeneratorURL }}<a href="{{ . }}">View rule</a>{{ end }}
  {{ with .Annotations.runbook_url }}<a href="{{ . }}">Runbook</a>{{ end }}
  {{ if eq .Status "firing" }}{{ with .SilenceURL }}<a href="{{ . }}">Silence</a>{{ end }}{{ end }}
{{ end }}
```

`{{ with X }}...{{ end }}` only shows the link when the value is set. `.DashboardURL` is set when the rule is linked to a dashboard.

### 6. Times in your time zone

```html
{{ formatTime .StartsAt "02 Jan 2006 15:04 MST" "Europe/Berlin" }}
```

The layout is written as the date `Mon Jan 2 15:04:05 MST 2006` should look. The third argument is an [IANA time zone](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones) name and is optional (UTC otherwise).

### 7. Query values

```html
{{ range $ref, $value := .Values }}{{ $ref }}: {{ humanize $value }}{{ end }}
```

`.Values` holds the result of each expression of the rule, for example `A: 94.2`. `humanize` writes 1234 as `1.23k`.

### 8. When the query fails

```html
{{ range .Alerts }}
  {{ if eq .Problem "query-error" }}
    <p style="color: #b91c1c;">The check could not run: {{ .Error }}</p>
  {{ else if eq .Problem "no-data" }}
    <p>The query returned no data.</p>
  {{ end }}
{{ end }}
```

### 9. Long alert groups

```html
{{ if .TruncatedAlerts }}<p>And {{ .TruncatedAlerts }} more alerts.</p>{{ end }}
```

The plugin lists at most *Alerts per email* alerts (default 40, set on the Configuration page) so Gmail does not cut the email off. `.FiringCount` and `.ResolvedCount` still count all of them.

## Styling for email clients

Email clients are not browsers. These rules keep a template looking right in Gmail, Outlook and Apple Mail:

- **Put CSS in one `<style>` block** and leave **Inline CSS for email clients** on. The plugin copies the rules into `style` attributes before sending.
- **Use tables for layout.** Outlook for Windows ignores flexbox and grid. Use `<table role="presentation">` for columns.
- **Set a width:** `max-width: 600px` on the main container is the usual email width.
- **Use full `https://` URLs for images**, such as a logo on your website. The plugin does not attach files.
- **Use web-safe fonts** with fallbacks: `-apple-system, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif`.
- **Keep it small.** Gmail cuts off emails larger than about 102 KB.

## A branded header

```html
<table role="presentation" width="100%" cellpadding="0" cellspacing="0">
  <tr>
    <td style="background-color: #0f172a; padding: 16px 24px;">
      <img src="https://example.com/logo-white.png" alt="Example Inc." height="28">
    </td>
  </tr>
</table>
```

## Use it

Save the template, then add its name as a label on the alert rules that should use it: `email_template=<name>`. See [routing alerts](routing.md).

Ready-made templates to copy: [examples](../examples/).
