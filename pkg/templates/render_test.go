package templates

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MiteshDhangar/beautify-alert/pkg/alert"
	"github.com/MiteshDhangar/beautify-alert/pkg/config"
)

func TestStartersRender(t *testing.T) {
	sample := alert.Sample(time.Now())
	for _, s := range Starters() {
		t.Run(s.Name, func(t *testing.T) {
			r, err := Render(s.Template, sample)
			if err != nil {
				t.Fatal(err)
			}
			if r.Subject == "" || !strings.Contains(r.HTML, "High CPU usage") {
				t.Errorf("subject %q, html missing alert name", r.Subject)
			}
			if strings.Contains(r.HTML, "ZgotmplZ") {
				t.Error("html/template rejected a value (ZgotmplZ in output)")
			}
			if strings.Contains(r.HTML, "<style") {
				t.Error("CSS was not inlined")
			}
		})
	}
}

func TestDefaultStarterContent(t *testing.T) {
	r, err := Render(Builtin(), alert.Sample(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	if r.Subject != "[FIRING:1] High CPU usage" {
		t.Errorf("subject = %q", r.Subject)
	}
	for _, want := range []string{"#dc2626", "CPU usage on api-server-1 is above 90%", "https://grafana.example.com/d/node-exporter", "94.2", "Runbook"} {
		if !strings.Contains(r.HTML, want) {
			t.Errorf("html missing %q", want)
		}
	}
	if strings.Contains(r.HTML, "email_template=") {
		t.Error("routing label shown in email")
	}
}

func TestValuesAreEscaped(t *testing.T) {
	n := alert.Sample(time.Now())
	n.Alerts[0].Annotations["summary"] = `<script>alert(1)</script>`
	r, err := Render(config.Template{HTML: `<p>{{ (index .Alerts 0).Annotations.summary }}</p>`}, n)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.HTML, "<script>") {
		t.Errorf("value not escaped: %s", r.HTML)
	}
}

func TestSafeHTMLIsOptIn(t *testing.T) {
	n := alert.Sample(time.Now())
	n.Alerts[0].Annotations["summary"] = `<b>bold</b>`
	r, err := Render(config.Template{HTML: `{{ safeHTML (index .Alerts 0).Annotations.summary }}`}, n)
	if err != nil || !strings.Contains(r.HTML, "<b>bold</b>") {
		t.Errorf("safeHTML: %v %q", err, r.HTML)
	}
}

func TestUnsafeFuncsRemoved(t *testing.T) {
	for _, f := range unsafeFuncs {
		_, err := Render(config.Template{HTML: `{{ ` + f + ` "HOME" }}`}, alert.Sample(time.Now()))
		if err == nil {
			t.Errorf("%s should not be available", f)
		}
	}
}

func TestErrorsNameThePart(t *testing.T) {
	_, err := Render(config.Template{Subject: "{{ .Nope", HTML: "x"}, alert.Sample(time.Now()))
	var te *Error
	if !errors.As(err, &te) || te.Part != "subject" {
		t.Errorf("err = %v", err)
	}
	_, err = Render(config.Template{HTML: "{{ range .Alerts }}"}, alert.Sample(time.Now()))
	if !errors.As(err, &te) || te.Part != "html" {
		t.Errorf("err = %v", err)
	}
}

func TestHelpers(t *testing.T) {
	ts := time.Date(2026, 11, 23, 12, 30, 0, 0, time.UTC)
	got, err := formatTime(ts, "15:04 MST", "Asia/Kolkata")
	if err != nil || got != "18:00 IST" {
		t.Errorf("formatTime = %q, %v", got, err)
	}
	if _, err := formatTime(ts, "15:04", "Mars/Olympus"); err == nil {
		t.Error("unknown zone should fail")
	}
	for in, want := range map[float64]string{94.2: "94.2", 1234: "1.23k", 2_500_000: "2.5M", 0.5: "0.5"} {
		if got := humanize(in); got != want {
			t.Errorf("humanize(%v) = %q, want %q", in, got, want)
		}
	}
	if SeverityColor("Warning") != "#d97706" || SeverityColor("odd") != "#6b7280" {
		t.Error("severityColor")
	}
	if got := since(time.Now().Add(-3*time.Hour - 5*time.Minute)); got != "3h5m" {
		t.Errorf("since = %q", got)
	}
}

func TestOutputIsCapped(t *testing.T) {
	_, err := Render(config.Template{HTML: `{{ range 600000 }}xxxxxxxxxx{{ end }}`}, alert.Sample(time.Now()))
	if err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("err = %v", err)
	}
}

func TestResolve(t *testing.T) {
	s := config.Settings{Templates: []config.Template{{Name: "default", HTML: "mine"}, {Name: "infra", HTML: "infra"}}}
	if tm, ok := Resolve(s, "infra"); !ok || tm.HTML != "infra" {
		t.Error("named template")
	}
	if tm, ok := Resolve(s, "missing"); ok || tm.HTML != "mine" {
		t.Error("missing falls back to saved default")
	}
	if tm, ok := Resolve(config.Settings{}, "missing"); ok || tm.HTML != Builtin().HTML {
		t.Error("missing falls back to built-in")
	}
}
