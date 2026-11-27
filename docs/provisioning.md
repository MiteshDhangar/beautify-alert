# Provisioning settings and templates

Grafana can set app plugin settings from a YAML file at startup, so you can keep templates in Git and deploy them like any other configuration.

Put a file in Grafana's `provisioning/plugins/` directory:

```yaml
apiVersion: 1

apps:
  - type: prudentbee939-beautifyalert-app
    org_id: 1
    disabled: false
    jsonData:
      delivery:
        provider: smtp
        fromAddress: alerts@example.com
        fromName: Grafana Alerts
        smtp:
          host: smtp.example.com
          port: 587
          security: starttls   # starttls, tls or none
          username: grafana
      defaultRecipients: ops@example.com
      routing:
        templateLabel: email_template
        recipientsLabel: email_to
      threading: true
      allowedRecipientDomains: example.com   # optional; empty allows any domain
      templates:
        - name: payments
          description: Finance team
          recipients: finance@example.com
          subject: 'Payments: {{ len .Alerts.Firing }} failing'
          html: |
            <h2>{{ .CommonLabels.alertname }}</h2>
            {{ range .Alerts }}<p>{{ .Annotations.summary }}</p>{{ end }}
    secureJsonData:
      smtpPassword: $SMTP_PASSWORD
```

Grafana expands `$SMTP_PASSWORD` from the environment. A complete example is in [examples/provisioning](../examples/provisioning/beautify-alert.yaml).

Grafana stores at most 64 KB of plugin settings, including provisioned ones. Templates in a provisioning file are stored as written (not compressed), so keep large template sets in the UI, or keep them short. Grafana rejects settings over the limit, provisioned ones included.

Grafana applies this file each time it starts and replaces the settings saved in the UI. Edits made in the UI last until the next restart, so treat the file as the source of truth.

The contact point and its service account can be provisioned too, with Grafana's [alerting provisioning](https://grafana.com/docs/grafana/latest/alerting/set-up/provision-alerting-resources/file-provisioning/) files. Use a webhook integration with the URL `<grafana>/api/plugins/prudentbee939-beautifyalert-app/resources/alert` and a Bearer token of an Editor service account.
