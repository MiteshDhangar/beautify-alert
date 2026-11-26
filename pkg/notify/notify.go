// Package notify turns a Grafana notification into sent emails: it plans
// the emails, renders each with its template, sends it and records the
// outcome in the delivery log.
package notify

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MiteshDhangar/beautify-alert/pkg/alert"
	"github.com/MiteshDhangar/beautify-alert/pkg/config"
	"github.com/MiteshDhangar/beautify-alert/pkg/delivery"
	"github.com/MiteshDhangar/beautify-alert/pkg/history"
	"github.com/MiteshDhangar/beautify-alert/pkg/routing"
	"github.com/MiteshDhangar/beautify-alert/pkg/templates"
)

// Service sends alert emails.
type Service struct {
	Settings config.Settings
	Sender   delivery.Sender
	Log      *history.Log
	Now      func() time.Time
}

// EmailResult is the outcome of one planned email.
type EmailResult struct {
	ID       string `json:"id"`
	Template string `json:"template"`
	// TemplateFound is false when the alert named a template that does not
	// exist and the default template was used instead.
	TemplateFound bool     `json:"templateFound"`
	To            []string `json:"to"`
	Subject       string   `json:"subject,omitempty"`
	Alerts        int      `json:"alerts"`
	OK            bool     `json:"ok"`
	Error         string   `json:"error,omitempty"`
	// Warnings are problems the email was sent despite, such as a skipped
	// recipient or a template that failed and was replaced by the built-in one.
	Warnings []string `json:"warnings,omitempty"`
	// Skipped explains why no email was sent on purpose, for example a
	// resolved-only email for a template that skips those.
	Skipped string `json:"skipped,omitempty"`
}

// Result is the outcome of one notification.
type Result struct {
	Emails []EmailResult `json:"emails"`
}

// Failed reports whether any email failed.
func (r Result) Failed() bool {
	for _, e := range r.Emails {
		if !e.OK {
			return true
		}
	}
	return false
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Handle sends the emails for one notification.
func (s *Service) Handle(ctx context.Context, n alert.Notification, o routing.Overrides) Result {
	var res Result
	for _, e := range routing.Plan(n, s.Settings, o) {
		r := s.handleEmail(ctx, n, e)
		res.Emails = append(res.Emails, r)
	}
	return res
}

func (s *Service) handleEmail(ctx context.Context, n alert.Notification, e routing.Email) EmailResult {
	tmpl, found := templates.Resolve(s.Settings, e.Template)
	sub := n.Subset(e.Alerts).Limit(s.Settings.MaxAlertsPerEmail)
	r := EmailResult{Template: e.Template, TemplateFound: found, Alerts: len(e.Alerts)}
	if tmpl.SkipResolved && len(e.Alerts.Firing()) == 0 {
		r.OK, r.To, r.Skipped = true, e.Recipients, "only resolved alerts, and this template skips resolved-only emails"
		entry := s.record(history.Entry{Source: history.SourceAlert, Template: e.Template, To: e.Recipients,
			AlertStatus: alert.StatusResolved, Alerts: len(e.Alerts), Skipped: r.Skipped}, nil)
		r.ID = entry.ID
		return r
	}
	if !found {
		r.Warnings = append(r.Warnings, fmt.Sprintf("template %q does not exist; used the default template", e.Template))
	}
	if sub.TruncatedAlerts > 0 && sub.TruncatedAlerts > n.TruncatedAlerts {
		r.Warnings = append(r.Warnings, fmt.Sprintf("listed %d of %d alerts (limit per email: %d)", len(sub.Alerts), len(e.Alerts), s.Settings.MaxAlertsPerEmail))
	}

	valid, invalid := config.ValidAddresses(e.Recipients)
	r.To = valid
	if len(invalid) > 0 {
		r.Warnings = append(r.Warnings, fmt.Sprintf("skipped invalid recipient %s", strings.Join(quoteAll(invalid), ", ")))
	}

	rendered, err := templates.Render(tmpl, sub)
	if err != nil {
		// Deliver the alert anyway: an email in the default layout beats none.
		r.Warnings = append(r.Warnings, fmt.Sprintf("template %q failed (%v); sent with the built-in template", tmpl.Name, err))
		rendered, err = templates.Render(templates.Builtin(), sub)
	}

	var msg *delivery.Message
	switch {
	case err != nil:
	case len(valid) == 0 && len(invalid) > 0:
		err = fmt.Errorf("no valid recipients: %s", strings.Join(quoteAll(invalid), ", "))
	default:
		r.Subject = rendered.Subject
		m := delivery.Message{To: valid, Subject: rendered.Subject, HTML: rendered.HTML, Text: rendered.Text}
		if s.Settings.ThreadsEmails() && n.GroupKey != "" {
			m.ThreadKey = n.GroupKey + "|" + e.Template
		}
		msg = &m
		err = s.send(ctx, m)
	}
	r.OK = err == nil
	if err != nil {
		r.Error = err.Error()
	}
	entry := s.record(history.Entry{
		Source: history.SourceAlert, Template: e.Template, To: valid, Subject: r.Subject,
		AlertStatus: sub.Status, Alerts: len(e.Alerts), Message: msg, Warnings: r.Warnings,
	}, err)
	r.ID = entry.ID
	return r
}

func quoteAll(xs []string) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = fmt.Sprintf("%q", x)
	}
	return out
}

// SendTest renders t with sample data and sends it with a [TEST] subject.
func (s *Service) SendTest(ctx context.Context, t config.Template, to []string) (history.Entry, error) {
	if len(to) == 0 {
		to = config.SplitAddresses(firstNonEmpty(t.Recipients, s.Settings.DefaultRecipients))
	}
	sample := alert.Sample(s.now())
	rendered, err := templates.Render(t, sample)
	if err != nil {
		return history.Entry{}, err
	}
	m := delivery.Message{To: to, Subject: "[TEST] " + rendered.Subject, HTML: rendered.HTML, Text: rendered.Text}
	err = s.send(ctx, m)
	name := t.Name
	if name == "" {
		name = "(unsaved)"
	}
	return s.record(history.Entry{Source: history.SourceTest, Template: name, To: to, Subject: m.Subject,
		AlertStatus: sample.Status, Alerts: len(sample.Alerts), Message: &m}, err), err
}

// ErrNotResendable is returned when a log entry has no stored message.
var ErrNotResendable = errors.New("this email cannot be resent: its body was not kept (it was too large or the log entry is gone)")

// Resend sends the stored message of a log entry again.
func (s *Service) Resend(ctx context.Context, id string) (history.Entry, error) {
	old, ok := s.Log.Get(id)
	if !ok || old.Message == nil {
		return history.Entry{}, ErrNotResendable
	}
	m := *old.Message
	err := s.send(ctx, m)
	return s.record(history.Entry{Source: history.SourceResend, Template: old.Template, To: m.To, Subject: m.Subject,
		AlertStatus: old.AlertStatus, Alerts: old.Alerts, Message: &m}, err), err
}

func (s *Service) send(ctx context.Context, m delivery.Message) error {
	if err := s.Settings.CheckDelivery(); err != nil {
		return err
	}
	if s.Sender == nil {
		return fmt.Errorf("%w: no email provider", config.ErrNotConfigured)
	}
	return s.Sender.Send(ctx, m)
}

func (s *Service) record(e history.Entry, err error) history.Entry {
	e.OK = err == nil
	if err != nil {
		e.Error = err.Error()
	}
	if s.Log == nil {
		return e
	}
	return s.Log.Add(e)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
