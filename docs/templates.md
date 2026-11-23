# Template reference

A template has a subject and an HTML body. Both are [Go templates](https://pkg.go.dev/text/template). The data is the same as in Grafana's own notification templates, so most Grafana template code works unchanged.

The HTML body is rendered with Go's `html/template`: your markup is kept and every value from an alert is escaped. An annotation that contains `<script>` shows as text.

## Notification data

At the top level, `.` is the notification:

| Field | Type | Description |
| --- | --- | --- |
| `.Status` | string | `firing` if any alert in this email is firing, else `resolved` |
| `.Alerts` | list of alerts | Every alert in this email |
| `.Alerts.Firing` | list of alerts | The firing alerts |
| `.Alerts.Resolved` | list of alerts | The resolved alerts |
| `.CommonLabels` | labels | Labels every alert in this email shares |
| `.CommonAnnotations` | labels | Annotations every alert in this email shares |
| `.GroupLabels` | labels | Labels the notification policy groups by |
| `.ExternalURL` | string | Link to Grafana |
| `.Receiver` | string | Contact point name |
| `.GroupKey` | string | Identifies the alert group |
| `.Title`, `.Message` | string | Grafana's default title and message for the webhook |

When one notification is split into several emails (different templates or recipients), `.Status`, `.CommonLabels` and `.CommonAnnotations` describe only the alerts in that email.

## Alert data

Inside `{{ range .Alerts }}`, `.` is one alert:

| Field | Type | Description |
| --- | --- | --- |
| `.Status` | string | `firing` or `resolved` |
| `.Labels` | labels | All labels, for example `.Labels.severity` |
| `.Annotations` | labels | All annotations, for example `.Annotations.summary` |
| `.StartsAt`, `.EndsAt` | time | When the alert started and ended |
| `.Values` | map of numbers | Values of the rule's expressions, by reference ID |
| `.ValueString` | string | Labels and values of the reduced expressions |
| `.DashboardURL`, `.PanelURL` | string | Links, when the rule is linked to a dashboard and panel |
| `.GeneratorURL` | string | Link to the alert rule |
| `.SilenceURL` | string | Link to create a silence |
| `.Fingerprint` | string | Unique ID of the alert |
| `.ImageURL` | string | Panel screenshot URL, when Grafana has screenshots set up |

Labels (`.Labels`, `.CommonLabels`, ...) also have:

| Method | Example |
| --- | --- |
| `.SortedPairs` | `{{ range .Labels.SortedPairs }}{{ .Name }}={{ .Value }}{{ end }}` |
| `.Names` | `{{ join ", " .Labels.Names }}` |
| `.Remove "key" ...` | `{{ range (.Labels.Remove "email_template" "email_to").SortedPairs }}...{{ end }}` |

## Functions

Every [Sprig](https://masterminds.github.io/sprig/) function is available except `env`, `expandenv` and `getHostByName`. For example `upper`, `default`, `join`, `trunc`, `date`, `list`.

Added by Beautify Alert:

| Function | Example | Result |
| --- | --- | --- |
| `statusColor` | `{{ statusColor .Status }}` | `#dc2626` when firing, `#16a34a` otherwise |
| `severityColor` | `{{ severityColor .Labels.severity }}` | Red for critical, orange for high, amber for warning, blue for info, gray otherwise |
| `formatTime` | `{{ formatTime .StartsAt "02 Jan 2006 15:04 MST" "Europe/Berlin" }}` | `23 Nov 2026 13:30 CET`. The time zone is optional. |
| `since` | `{{ since .StartsAt }}` | `12m`, `3h5m`, `2d4h` |
| `humanize` | `{{ humanize 1234.5 }}` | `1.23k` |
| `safeHTML` | `{{ safeHTML .Annotations.html_note }}` | Inserts the value without escaping. Only use it for content you trust. |

The layout argument of `formatTime` is a [Go time layout](https://pkg.go.dev/time#pkg-constants): write the date `Mon Jan 2 15:04:05 MST 2006` the way you want dates to look.

## Examples

Different text for firing and resolved:

```html
{{ if eq .Status "firing" }}
  <h2>{{ len .Alerts.Firing }} alerts need attention</h2>
{{ else }}
  <h2>All clear</h2>
{{ end }}
```

A table of alerts:

```html
<table>
  {{ range .Alerts }}
  <tr>
    <td style="color: {{ statusColor .Status }}">{{ .Status }}</td>
    <td>{{ .Labels.instance }}</td>
    <td>{{ .Annotations.summary }}</td>
    <td>{{ formatTime .StartsAt "15:04" }}</td>
  </tr>
  {{ end }}
</table>
```

Values with units:

```html
{{ range $ref, $value := .Values }}{{ $ref }}: {{ humanize $value }}%{{ end }}
```

## Email client tips

- Leave **Inline CSS for email clients** on. Many clients drop `<style>` blocks; inlining copies the rules into `style` attributes. Media queries stay in `<style>`.
- Use tables for layout. Outlook for Windows does not support flexbox or grid.
- Use full `https://` URLs for images. The plugin does not attach files yet.
- Gmail groups emails into a conversation only when the subjects match, besides the `References` header. Keep the status out of the subject if you want firing and resolved emails grouped in Gmail.

## Limits

- A rendered email can be at most 5 MB.
- Template errors show in the preview. When an alert fails to render, the delivery log records the error and Grafana retries the notification.
