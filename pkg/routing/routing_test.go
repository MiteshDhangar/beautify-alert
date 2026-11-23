package routing

import (
	"testing"

	"github.com/MiteshDhangar/beautify-alert/pkg/alert"
	"github.com/MiteshDhangar/beautify-alert/pkg/config"
)

func settings() config.Settings {
	return config.Settings{
		DefaultRecipients: "ops@x.com",
		Routing:           config.Routing{TemplateLabel: "email_template", RecipientsLabel: "email_to"},
		Templates: []config.Template{
			{Name: "payments", Recipients: "finance@x.com"},
			{Name: "infra"},
		},
	}
}

func TestPlanSplitsByTemplateAndRecipients(t *testing.T) {
	n := alert.Notification{Alerts: alert.Alerts{
		{Status: "firing", Labels: alert.KV{"alertname": "a", "email_template": "payments"}},
		{Status: "firing", Labels: alert.KV{"alertname": "b"}},
		{Status: "resolved", Labels: alert.KV{"alertname": "c", "email_template": "payments"}},
		{Status: "firing", Labels: alert.KV{"alertname": "d", "email_template": "payments", "email_to": "cfo@x.com"}},
	}}
	got := Plan(n, settings(), Overrides{})
	if len(got) != 3 {
		t.Fatalf("got %d emails, want 3: %+v", len(got), got)
	}
	checks := []struct {
		template, to string
		alerts       int
	}{
		{"payments", "finance@x.com", 2}, // template recipients
		{"default", "ops@x.com", 1},      // default recipients
		{"payments", "cfo@x.com", 1},     // label beats template recipients
	}
	for i, c := range checks {
		if got[i].Template != c.template || got[i].Recipients[0] != c.to || len(got[i].Alerts) != c.alerts {
			t.Errorf("email %d = %s %v %d alerts, want %s %s %d", i, got[i].Template, got[i].Recipients, len(got[i].Alerts), c.template, c.to, c.alerts)
		}
	}
}

func TestPlanAnnotationsAndOverrides(t *testing.T) {
	n := alert.Notification{Alerts: alert.Alerts{
		{Labels: alert.KV{"alertname": "a"}, Annotations: alert.KV{"email_template": "infra", "email_to": "sre@x.com"}},
	}}
	got := Plan(n, settings(), Overrides{})
	if got[0].Template != "infra" || got[0].Recipients[0] != "sre@x.com" {
		t.Errorf("annotations not used: %+v", got[0])
	}
	got = Plan(n, settings(), Overrides{Template: "custom", Recipients: "a@x.com; b@x.com"})
	if got[0].Template != "custom" || len(got[0].Recipients) != 2 {
		t.Errorf("overrides not used: %+v", got[0])
	}
}

func TestPlanCustomLabelNames(t *testing.T) {
	s := settings()
	s.Routing = config.Routing{TemplateLabel: "mail_layout", RecipientsLabel: "notify"}
	n := alert.Notification{Alerts: alert.Alerts{
		{Labels: alert.KV{"mail_layout": "infra", "notify": "x@x.com", "email_template": "payments"}},
	}}
	got := Plan(n, s, Overrides{})
	if got[0].Template != "infra" || got[0].Recipients[0] != "x@x.com" {
		t.Errorf("custom labels not used: %+v", got[0])
	}
}

func TestPlanRecipientOrderDoesNotSplit(t *testing.T) {
	n := alert.Notification{Alerts: alert.Alerts{
		{Labels: alert.KV{"email_to": "a@x.com,b@x.com"}},
		{Labels: alert.KV{"email_to": "B@x.com, a@x.com"}},
	}}
	if got := Plan(n, settings(), Overrides{}); len(got) != 1 {
		t.Errorf("same recipients in another order made %d emails", len(got))
	}
}
