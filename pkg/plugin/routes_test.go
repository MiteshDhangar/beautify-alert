package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"

	"github.com/MiteshDhangar/beautify-alert/pkg/alert"
	"github.com/MiteshDhangar/beautify-alert/pkg/config"
	"github.com/MiteshDhangar/beautify-alert/pkg/delivery"
	"github.com/MiteshDhangar/beautify-alert/pkg/history"
)

type fakeSender struct {
	sent []delivery.Message
	err  error
}

func (f *fakeSender) Send(_ context.Context, m delivery.Message) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, m)
	return nil
}
func (f *fakeSender) Verify(context.Context) error { return f.err }

type recorder struct {
	status int
	body   []byte
}

func (r *recorder) Send(resp *backend.CallResourceResponse) error {
	r.status, r.body = resp.Status, resp.Body
	return nil
}

func testSettings() config.Settings {
	s := config.Settings{
		Delivery:          config.Delivery{Provider: "smtp", FromAddress: "alerts@x.com", SMTP: config.SMTP{Host: "smtp.x.com"}},
		DefaultRecipients: "ops@x.com",
		Routing:           config.Routing{TemplateLabel: "email_template", RecipientsLabel: "email_to"},
		Templates: []config.Template{
			{Name: "payments", Subject: "Payments: {{ .CommonLabels.alertname }}", HTML: `<h1>{{ len .Alerts }} payment alerts</h1>`},
		},
	}
	return s
}

func newTestApp(s config.Settings, f *fakeSender) *App {
	a := newApp(s, f, nil)
	a.service.Log = history.New()
	return a
}

func call(t *testing.T, a *App, role, method, path string, body any) recorder {
	t.Helper()
	b, _ := json.Marshal(body)
	var rec recorder
	err := a.CallResource(context.Background(), &backend.CallResourceRequest{
		PluginContext: backend.PluginContext{User: &backend.User{Login: "u", Email: "me@x.com", Role: role}},
		Method:        method,
		Path:          strings.TrimPrefix(strings.SplitN(path, "?", 2)[0], "/"),
		URL:           strings.TrimPrefix(path, "/"),
		Body:          b,
	}, &rec)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func TestAlertUsesNamedTemplateAndThreads(t *testing.T) {
	f := &fakeSender{}
	a := newTestApp(testSettings(), f)
	n := alert.Sample(time.Now())
	for i := range n.Alerts {
		n.Alerts[i].Labels["email_template"] = "payments"
	}
	rec := call(t, a, "Editor", http.MethodPost, "/alert", n)
	if rec.status != http.StatusOK {
		t.Fatalf("status %d: %s", rec.status, rec.body)
	}
	if len(f.sent) != 1 {
		t.Fatalf("sent %d", len(f.sent))
	}
	m := f.sent[0]
	if m.Subject != "Payments: High CPU usage" || !strings.Contains(m.HTML, "2 payment alerts") || m.To[0] != "ops@x.com" {
		t.Errorf("sent %+v", m)
	}
	if m.ThreadKey == "" || m.Text == "" {
		t.Errorf("thread key %q, text %q", m.ThreadKey, m.Text)
	}
	if got := a.service.Log.List(10); len(got) != 1 || !got[0].OK || got[0].Template != "payments" {
		t.Errorf("history %+v", got)
	}
}

func TestAlertQueryOverrides(t *testing.T) {
	f := &fakeSender{}
	a := newTestApp(testSettings(), f)
	rec := call(t, a, "Editor", http.MethodPost, "/alert?template=payments&to=cto@x.com", alert.Sample(time.Now()))
	if rec.status != http.StatusOK || f.sent[0].To[0] != "cto@x.com" || !strings.HasPrefix(f.sent[0].Subject, "Payments") {
		t.Errorf("status %d sent %+v", rec.status, f.sent)
	}
}

func TestAlertFailureMakesGrafanaRetry(t *testing.T) {
	f := &fakeSender{err: context.DeadlineExceeded}
	a := newTestApp(testSettings(), f)
	rec := call(t, a, "Editor", http.MethodPost, "/alert", alert.Sample(time.Now()))
	if rec.status != http.StatusBadGateway {
		t.Errorf("status %d", rec.status)
	}
	if got := a.service.Log.List(10); len(got) != 1 || got[0].OK || got[0].Error == "" {
		t.Errorf("history %+v", got)
	}
}

func TestViewerCannotSend(t *testing.T) {
	f := &fakeSender{}
	a := newTestApp(testSettings(), f)
	for _, path := range []string{"/alert", "/test"} {
		if rec := call(t, a, "Viewer", http.MethodPost, path, alert.Sample(time.Now())); rec.status != http.StatusForbidden {
			t.Errorf("%s: status %d", path, rec.status)
		}
	}
	if rec := call(t, a, "Editor", http.MethodPost, "/verify", map[string]any{}); rec.status != http.StatusForbidden {
		t.Errorf("verify needs Admin: status %d", rec.status)
	}
	if len(f.sent) != 0 {
		t.Error("sent as viewer")
	}
}

func TestPreviewAndErrors(t *testing.T) {
	a := newTestApp(testSettings(), &fakeSender{})
	rec := call(t, a, "Viewer", http.MethodPost, "/preview", previewRequest{Template: config.Template{HTML: "<b>x</b>"}})
	if rec.status != http.StatusForbidden {
		t.Errorf("viewer preview %d %s", rec.status, rec.body)
	}
	rec = call(t, a, "Editor", http.MethodPost, "/preview", previewRequest{Template: config.Template{HTML: "<b>{{ .Status }}</b>"}, Status: "resolved"})
	if rec.status != http.StatusOK || !strings.Contains(string(rec.body), "resolved") {
		t.Errorf("preview %d %s", rec.status, rec.body)
	}
	rec = call(t, a, "Editor", http.MethodPost, "/preview", previewRequest{Template: config.Template{HTML: "{{ .Nope "}})
	if rec.status != http.StatusUnprocessableEntity || !strings.Contains(string(rec.body), `"part":"html"`) {
		t.Errorf("error %d %s", rec.status, rec.body)
	}
}

func TestTestSendAndResend(t *testing.T) {
	f := &fakeSender{}
	a := newTestApp(testSettings(), f)
	rec := call(t, a, "Editor", http.MethodPost, "/test", previewRequest{Template: config.Template{Name: "payments", HTML: "<p>hi</p>"}, To: "me@x.com"})
	if rec.status != http.StatusOK || len(f.sent) != 1 || !strings.HasPrefix(f.sent[0].Subject, "[TEST]") || f.sent[0].To[0] != "me@x.com" {
		t.Fatalf("test %d %s %+v", rec.status, rec.body, f.sent)
	}
	var e history.Entry
	_ = json.Unmarshal(rec.body, &e)
	rec = call(t, a, "Editor", http.MethodPost, "/history/"+e.ID+"/resend", nil)
	if rec.status != http.StatusOK || len(f.sent) != 2 {
		t.Errorf("resend %d %s", rec.status, rec.body)
	}
	rec = call(t, a, "Editor", http.MethodGet, "/history/"+e.ID, nil)
	var full history.Entry
	_ = json.Unmarshal(rec.body, &full)
	if rec.status != http.StatusOK || full.Message == nil || full.Message.HTML != "<p>hi</p>" {
		t.Errorf("entry %d %+v", rec.status, full)
	}
	rec = call(t, a, "Editor", http.MethodGet, "/history", nil)
	var list struct{ Entries []history.Entry }
	_ = json.Unmarshal(rec.body, &list)
	if rec.status != http.StatusOK || len(list.Entries) != 2 || list.Entries[0].Message != nil {
		t.Errorf("list should have 2 entries without bodies: %s", rec.body)
	}
}

func TestNotConfiguredIsClear(t *testing.T) {
	s := testSettings()
	s.Delivery.SMTP.Host = ""
	a := newTestApp(s, &fakeSender{})
	rec := call(t, a, "Editor", http.MethodPost, "/test", previewRequest{Template: config.Template{HTML: "x"}})
	if rec.status != http.StatusBadRequest || !strings.Contains(string(rec.body), "SMTP host") {
		t.Errorf("%d %s", rec.status, rec.body)
	}
}

func TestDeliveryLogIsPerOrganization(t *testing.T) {
	a := newApp(testSettings(), &fakeSender{}, nil) // real per-org logs
	callOrg := func(org string, method, path string, body any) recorder {
		b, _ := json.Marshal(body)
		var rec recorder
		if err := a.CallResource(context.Background(), &backend.CallResourceRequest{
			PluginContext: backend.PluginContext{Namespace: org, User: &backend.User{Login: "u", Role: "Admin"}},
			Method:        method, Path: strings.TrimPrefix(path, "/"), URL: strings.TrimPrefix(path, "/"), Body: b,
		}, &rec); err != nil {
			t.Fatal(err)
		}
		return rec
	}
	callOrg("org-9001", http.MethodPost, "/test", previewRequest{Template: config.Template{HTML: "<p>org one</p>"}, To: "a@x.com"})
	count := func(org string) int {
		var list struct{ Entries []history.Entry }
		_ = json.Unmarshal(callOrg(org, http.MethodGet, "/history", nil).body, &list)
		return len(list.Entries)
	}
	if count("org-9001") != 1 || count("org-9002") != 0 {
		t.Errorf("org 9001 sees %d, org 9002 sees %d", count("org-9001"), count("org-9002"))
	}
}
