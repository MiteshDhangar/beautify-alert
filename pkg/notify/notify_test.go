package notify

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/MiteshDhangar/beautify-alert/pkg/alert"
	"github.com/MiteshDhangar/beautify-alert/pkg/config"
	"github.com/MiteshDhangar/beautify-alert/pkg/delivery"
	"github.com/MiteshDhangar/beautify-alert/pkg/history"
	"github.com/MiteshDhangar/beautify-alert/pkg/routing"
)

type fakeSender struct{ sent []delivery.Message }

func (f *fakeSender) Send(_ context.Context, m delivery.Message) error {
	f.sent = append(f.sent, m)
	return nil
}
func (f *fakeSender) Verify(context.Context) error { return nil }

func service(templates ...config.Template) (*Service, *fakeSender) {
	f := &fakeSender{}
	s := config.Settings{
		Delivery:          config.Delivery{Provider: "smtp", FromAddress: "a@x.com", SMTP: config.SMTP{Host: "h"}},
		DefaultRecipients: "ops@x.com",
		Routing:           config.Routing{TemplateLabel: "email_template", RecipientsLabel: "email_to"},
		MaxAlertsPerEmail: 3,
		Templates:         templates,
	}
	return &Service{Settings: s, Sender: f, Log: history.New(), Now: time.Now}, f
}

func one(labels alert.KV) alert.Notification {
	labels["alertname"] = "A"
	return alert.Notification{GroupKey: "g", Alerts: alert.Alerts{{Status: "firing", Labels: labels}}}
}

func TestInvalidRecipientsAreSkipped(t *testing.T) {
	s, f := service()
	res := s.Handle(context.Background(), one(alert.KV{"email_to": "good@x.com, not-an-email"}), routing.Overrides{})
	e := res.Emails[0]
	if !e.OK || len(f.sent) != 1 || len(f.sent[0].To) != 1 || f.sent[0].To[0] != "good@x.com" {
		t.Fatalf("result %+v sent %+v", e, f.sent)
	}
	if len(e.Warnings) != 1 || !strings.Contains(e.Warnings[0], "not-an-email") {
		t.Errorf("warnings %q", e.Warnings)
	}
	if got := s.Log.List(1)[0]; len(got.Warnings) != 1 {
		t.Errorf("log warnings %q", got.Warnings)
	}
}

func TestOnlyInvalidRecipientsFail(t *testing.T) {
	s, f := service()
	res := s.Handle(context.Background(), one(alert.KV{"email_to": "nope"}), routing.Overrides{})
	if res.Emails[0].OK || len(f.sent) != 0 || !strings.Contains(res.Emails[0].Error, "no valid recipients") {
		t.Errorf("result %+v", res.Emails[0])
	}
}

func TestBrokenTemplateFallsBackToBuiltin(t *testing.T) {
	s, f := service(config.Template{Name: "broken", HTML: `{{ (index .Alerts 5).Labels.alertname }}`})
	res := s.Handle(context.Background(), one(alert.KV{"email_template": "broken"}), routing.Overrides{})
	e := res.Emails[0]
	if !e.OK || len(f.sent) != 1 || !strings.Contains(f.sent[0].HTML, "Firing") {
		t.Fatalf("result %+v", e)
	}
	if len(e.Warnings) != 1 || !strings.Contains(e.Warnings[0], "built-in template") {
		t.Errorf("warnings %q", e.Warnings)
	}
}

func TestAlertsPerEmailAreCapped(t *testing.T) {
	s, f := service()
	n := alert.Notification{GroupKey: "g"}
	for i := 0; i < 10; i++ {
		n.Alerts = append(n.Alerts, alert.Alert{Status: "firing", Labels: alert.KV{"alertname": "A"}})
	}
	res := s.Handle(context.Background(), n, routing.Overrides{})
	e := res.Emails[0]
	if !e.OK || e.Alerts != 10 || !strings.Contains(f.sent[0].HTML, "And 7 more alerts") || !strings.Contains(f.sent[0].Subject, "FIRING:10") {
		t.Errorf("result %+v subject %q", e, f.sent[0].Subject)
	}
	if len(e.Warnings) != 1 || !strings.Contains(e.Warnings[0], "listed 3 of 10") {
		t.Errorf("warnings %q", e.Warnings)
	}
}

func TestGrafanaErrorAlertIsNamed(t *testing.T) {
	s, f := service()
	n := alert.Notification{Alerts: alert.Alerts{{Status: "firing",
		Labels:      alert.KV{"alertname": "DatasourceError", "rulename": "Disk usage"},
		Annotations: alert.KV{"Error": "connection refused"}}}}
	s.Handle(context.Background(), n, routing.Overrides{})
	m := f.sent[0]
	if m.Subject != "[FIRING:1] Disk usage (query failed)" || !strings.Contains(m.HTML, "connection refused") || !strings.Contains(m.Text, "Error: connection refused") {
		t.Errorf("subject %q", m.Subject)
	}
}

func TestSkipResolvedOnlyEmails(t *testing.T) {
	s, f := service(config.Template{Name: "window", HTML: "<p>{{ .Status }}</p>", SkipResolved: true})
	resolved := alert.Notification{Alerts: alert.Alerts{{Status: "resolved", Labels: alert.KV{"alertname": "A", "email_template": "window"}}}}
	res := s.Handle(context.Background(), resolved, routing.Overrides{})
	if !res.Emails[0].OK || res.Emails[0].Skipped == "" || len(f.sent) != 0 {
		t.Fatalf("resolved-only should be skipped: %+v", res.Emails[0])
	}
	if e := s.Log.List(1)[0]; e.Skipped == "" {
		t.Errorf("skip not logged: %+v", e)
	}
	mixed := alert.Notification{Alerts: alert.Alerts{
		{Status: "resolved", Labels: alert.KV{"alertname": "A", "email_template": "window"}},
		{Status: "firing", Labels: alert.KV{"alertname": "A", "email_template": "window"}},
	}}
	s.Handle(context.Background(), mixed, routing.Overrides{})
	if len(f.sent) != 1 {
		t.Errorf("mixed email should be sent, sent %d", len(f.sent))
	}
}

func TestRecipientLimitsAndDomains(t *testing.T) {
	s, f := service()
	s.Settings.AllowedRecipientDomains = "x.com"
	s.Settings.MaxRecipientsPerEmail = 3
	n := one(alert.KV{"email_to": "a@x.com, b@x.com, c@other.test, d@x.com, e@x.com"})
	res := s.Handle(context.Background(), n, routing.Overrides{})
	if len(f.sent) != 1 || strings.Join(f.sent[0].To, ",") != "a@x.com,b@x.com,d@x.com" {
		t.Fatalf("sent %+v", f.sent)
	}
	w := strings.Join(res.Emails[0].Warnings, " | ")
	if !strings.Contains(w, "outside the allowed domains") || !strings.Contains(w, "first 3 of 4") {
		t.Errorf("warnings %q", w)
	}
	// Nobody left: the email fails instead of going out.
	n.Alerts[0].Labels["email_to"] = "c@other.test"
	res = s.Handle(context.Background(), n, routing.Overrides{})
	if res.Emails[0].OK || len(f.sent) != 1 {
		t.Errorf("res %+v", res.Emails[0])
	}
}
