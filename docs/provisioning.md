# Provisioning settings and templates

Grafana can set app plugin settings from a YAML file at startup, so you can keep templates in Git and deploy them like any other configuration.

Put a file in Grafana's `provisioning/plugins/` directory:

```yaml
apiVersion: 1

apps:
  - type: miteshdhangar76-beautifyalert-app
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

Grafana expands `$SMTP_PASSWORD` from the environment.

Grafana applies this file each time it starts and replaces the settings saved in the UI. Edits made in the UI last until the next restart, so treat the file as the source of truth.

The contact point and its service account can be provisioned too, with Grafana's [alerting provisioning](https://grafana.com/docs/grafana/latest/alerting/set-up/provision-alerting-resources/file-provisioning/) files. Use a webhook integration with the URL `<grafana>/api/plugins/miteshdhangar76-beautifyalert-app/resources/alert` and a Bearer token of an Editor service account.
