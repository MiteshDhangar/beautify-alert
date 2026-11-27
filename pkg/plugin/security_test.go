package plugin

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/MiteshDhangar/beautify-alert/pkg/config"
)

func resetLimiter() {
	testLimiter = &windowLimiter{limit: testEmailLimit, window: testEmailWindow, hits: map[string][]time.Time{}}
}

func TestEditorTestEmailsOnlyGoToKnownAddresses(t *testing.T) {
	resetLimiter()
	s := testSettings()
	s.Templates[0].Recipients = "payments-team@x.com"
	f := &fakeSender{}
	a := newTestApp(s, f)
	tmpl := config.Template{Name: "x", HTML: "<p>hi</p>"}
	for _, tc := range []struct {
		role, to string
		want     int
	}{
		{"Editor", "me@x.com", http.StatusOK},            // own address
		{"Editor", "ops@x.com", http.StatusOK},           // default recipients
		{"Editor", "payments-team@x.com", http.StatusOK}, // a saved template's recipients
		{"Editor", "", http.StatusOK},                    // falls back to the default recipients
		{"Editor", "victim@outside.test", http.StatusForbidden},
		{"Editor", "me@x.com, victim@outside.test", http.StatusForbidden},
		{"Admin", "anyone@outside.test", http.StatusOK},
	} {
		rec := call(t, a, tc.role, http.MethodPost, "/test", previewRequest{Template: tmpl, To: tc.to})
		if rec.status != tc.want {
			t.Errorf("%s to %q: %d %s", tc.role, tc.to, rec.status, rec.body)
		}
	}
	for _, m := range f.sent {
		for _, to := range m.To {
			if strings.Contains(to, "victim") {
				t.Errorf("sent to %s", to)
			}
		}
	}
	// An unsaved template's own recipients do not count.
	rec := call(t, a, "Editor", http.MethodPost, "/test", previewRequest{Template: config.Template{Name: "x", Recipients: "victim@outside.test", HTML: "x"}})
	if rec.status != http.StatusForbidden {
		t.Errorf("unsaved template recipients: %d %s", rec.status, rec.body)
	}
}

func TestAllowedDomainsWidenEditorTestsAndLimitAdmins(t *testing.T) {
	resetLimiter()
	s := testSettings()
	s.AllowedRecipientDomains = "x.com, @partner.org"
	f := &fakeSender{}
	a := newTestApp(s, f)
	tmpl := config.Template{Name: "x", HTML: "<p>hi</p>"}
	if rec := call(t, a, "Editor", http.MethodPost, "/test", previewRequest{Template: tmpl, To: "someone@partner.org"}); rec.status != http.StatusOK {
		t.Errorf("allowed domain: %d %s", rec.status, rec.body)
	}
	if rec := call(t, a, "Admin", http.MethodPost, "/test", previewRequest{Template: tmpl, To: "anyone@outside.test"}); rec.status != http.StatusBadRequest {
		t.Errorf("admin outside allowed domains: %d %s", rec.status, rec.body)
	}
}

func TestTestEmailsAreRateLimited(t *testing.T) {
	resetLimiter()
	f := &fakeSender{}
	a := newTestApp(testSettings(), f)
	var last recorder
	for i := 0; i < testEmailLimit+1; i++ {
		last = call(t, a, "Admin", http.MethodPost, "/test", previewRequest{Template: config.Template{Name: "x", HTML: "x"}, To: "me@x.com"})
	}
	if last.status != http.StatusTooManyRequests || len(f.sent) != testEmailLimit {
		t.Errorf("status %d, sent %d", last.status, len(f.sent))
	}
}

func TestVerifyReusesSavedPasswordOnlyForSavedServer(t *testing.T) {
	s := testSettings()
	s.Delivery.SMTP = config.SMTP{Host: "smtp.x.com", Port: 587, Username: "svc", Password: "saved-secret", Security: config.SecurityStartTLS}
	f := &fakeSender{}
	a := newTestApp(s, f)
	base := s.Delivery
	for name, tc := range map[string]struct {
		change func(d *config.Delivery)
		want   int
	}{
		"other host":        {func(d *config.Delivery) { d.SMTP.Host = "evil.test" }, http.StatusBadRequest},
		"other port":        {func(d *config.Delivery) { d.SMTP.Port = 2525 }, http.StatusBadRequest},
		"no TLS":            {func(d *config.Delivery) { d.SMTP.Security = config.SecurityNone }, http.StatusBadRequest},
		"skip cert check":   {func(d *config.Delivery) { d.SMTP.SkipVerify = true }, http.StatusBadRequest},
		"same server":       {func(d *config.Delivery) {}, http.StatusBadGateway}, // reaches delivery.New with the saved password; the host does not exist
		"host in uppercase": {func(d *config.Delivery) { d.SMTP.Host = "SMTP.X.COM" }, http.StatusBadGateway},
	} {
		d := base
		tc.change(&d)
		rec := call(t, a, "Admin", http.MethodPost, "/verify", verifyRequest{Delivery: d})
		if rec.status != tc.want {
			t.Errorf("%s: %d %s", name, rec.status, rec.body)
		}
		if tc.want == http.StatusBadRequest && !strings.Contains(string(rec.body), "enter the password") {
			t.Errorf("%s: %s", name, rec.body)
		}
	}
}

func TestRenderSlotsReturnBusy(t *testing.T) {
	a := newTestApp(testSettings(), &fakeSender{})
	for i := 0; i < cap(renderSlots); i++ {
		renderSlots <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(renderSlots); i++ {
			<-renderSlots
		}
	}()
	rec := call(t, a, "Editor", http.MethodPost, "/preview", previewRequest{Template: config.Template{HTML: "x"}})
	if rec.status != http.StatusTooManyRequests {
		t.Errorf("preview %d %s", rec.status, rec.body)
	}
}

func TestOneNotificationSendsAtMostTheEmailLimit(t *testing.T) {
	f := &fakeSender{}
	a := newTestApp(testSettings(), f)
	var alerts []map[string]any
	for i := 0; i < 300; i++ {
		alerts = append(alerts, map[string]any{"status": "firing", "fingerprint": string(rune('a'+i%26)) + strings.Repeat("x", i),
			"labels": map[string]string{"alertname": "A", "email_to": "v" + strings.Repeat("x", i) + "@x.com"}})
	}
	rec := call(t, a, "Editor", http.MethodPost, "/alert", map[string]any{"status": "firing", "alerts": alerts})
	var res struct{ Emails []struct{ OK bool } }
	_ = json.Unmarshal(rec.body, &res)
	if len(f.sent) != config.DefaultMaxEmailsPerNotification || len(res.Emails) != config.DefaultMaxEmailsPerNotification+1 || res.Emails[len(res.Emails)-1].OK {
		t.Errorf("sent %d, results %d: %s", len(f.sent), len(res.Emails), rec.body[:min(300, len(rec.body))])
	}
}
