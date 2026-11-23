package alert

import "time"

// Sample returns a realistic notification for previews and test emails:
// one firing alert and one resolved alert from the same rule.
func Sample(now time.Time) Notification {
	orgID := int64(1)
	return Notification{
		Receiver: "Beautify Alert",
		Status:   StatusFiring,
		Alerts: Alerts{
			{
				Status: StatusFiring,
				Labels: KV{
					"alertname": "High CPU usage",
					"instance":  "api-server-1:9100",
					"severity":  "critical",
					"team":      "platform",
				},
				Annotations: KV{
					"summary":     "CPU usage on api-server-1 is above 90%",
					"description": "CPU usage has been above 90% for the last 10 minutes. Check for runaway processes or scale out the service.",
					"runbook_url": "https://example.com/runbooks/high-cpu",
				},
				StartsAt:     now.Add(-12 * time.Minute).Truncate(time.Minute),
				GeneratorURL: "https://grafana.example.com/alerting/grafana/cpu-high/view",
				Fingerprint:  "8a3f1c2d9e4b5a67",
				SilenceURL:   "https://grafana.example.com/alerting/silence/new?matcher=alertname%3DHigh+CPU+usage",
				DashboardURL: "https://grafana.example.com/d/node-exporter",
				PanelURL:     "https://grafana.example.com/d/node-exporter?viewPanel=3",
				Values:       map[string]float64{"A": 94.2},
				ValueString:  "[ var='A' labels={instance=api-server-1:9100} value=94.2 ]",
				OrgID:        &orgID,
			},
			{
				Status: StatusResolved,
				Labels: KV{
					"alertname": "High CPU usage",
					"instance":  "api-server-2:9100",
					"severity":  "critical",
					"team":      "platform",
				},
				Annotations: KV{
					"summary":     "CPU usage on api-server-2 is back to normal",
					"description": "CPU usage dropped below 90%.",
				},
				StartsAt:     now.Add(-40 * time.Minute).Truncate(time.Minute),
				EndsAt:       now.Add(-3 * time.Minute).Truncate(time.Minute),
				GeneratorURL: "https://grafana.example.com/alerting/grafana/cpu-high/view",
				Fingerprint:  "1b2c3d4e5f6a7b8c",
				DashboardURL: "https://grafana.example.com/d/node-exporter",
				Values:       map[string]float64{"A": 41.7},
				OrgID:        &orgID,
			},
		},
		GroupLabels:       KV{"alertname": "High CPU usage"},
		CommonLabels:      KV{"alertname": "High CPU usage", "severity": "critical", "team": "platform"},
		CommonAnnotations: KV{},
		ExternalURL:       "https://grafana.example.com/",
		Version:           "1",
		GroupKey:          `{}:{alertname="High CPU usage"}`,
		OrgID:             1,
		Title:             "[FIRING:1] High CPU usage",
		State:             "alerting",
	}
}
