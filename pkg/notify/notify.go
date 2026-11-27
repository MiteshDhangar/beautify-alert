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

func orDefault(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

// Handle sends the emails for one notification.
func (s *Service) Handle(ctx context.Context, n alert.Notification, o routing.Overrides) Result {
	var res Result
	plan := routing.Plan(n, s.Settings, o)
	limit := orDefault(s.Settings.MaxEmailsPerNotification, config.DefaultMaxEmailsPerNotification)
	for i, e := range plan {
		if i == limit {
			res.Emails = append(res.Emails, s.overLimit(plan[i:], limit))
			break
		}
		res.Emails = append(res.Emails, s.handleEmail(ctx, n, e))
	}
	return res
}

// overLimit records the emails a notification was not allowed to send, as
// one failed entry, so a flood of alerts cannot also flood the delivery log.
func (s *Service) overLimit(rest []routing.Email, limit int) EmailResult {
	alerts := 0
	for _, e := range rest {
		alerts += len(e.Alerts)
	}
	err := fmt.Errorf("not sent: this notification would send %d more emails (%d alerts) over the limit of %d emails per notification. Raise \"Max emails per notification\" on the Configuration page if this is expected", len(rest), alerts, limit)
	entry := s.record(history.Entry{Source: history.SourceAlert, Template: rest[0].Template, Alerts: alerts}, err)
	return EmailResult{ID: entry.ID, Template: rest[0].Template, TemplateFound: true, Alerts: alerts, Error: err.Error()}
}

// allowedRecipients drops recipients outside the allowed domains and caps
// how many one email has, with a warning for each.
func (s *Service) allowedRecipients(valid []string) (kept []string, warnings []string) {
	var blocked []string
	for _, a := range valid {
		if s.Settings.DomainAllowed(a) {
			kept = append(kept, a)
		} else {
			blocked = append(blocked, a)
		}
	}
	if len(blocked) > 0 {
		warnings = append(warnings, fmt.Sprintf("skipped recipient outside the allowed domains: %s", strings.Join(quoteAll(blocked), ", ")))
	}
	if limit := orDefault(s.Settings.MaxRecipientsPerEmail, config.DefaultMaxRecipientsPerEmail); len(kept) > limit {
		warnings = append(warnings, fmt.Sprintf("sent to the first %d of %d recipients (limit per email: %d)", limit, len(kept), limit))
		kept = kept[:limit]
	}
	return kept, warnings
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
	if len(invalid) > 0 {
		r.Warnings = append(r.Warnings, fmt.Sprintf("skipped invalid recipient %s", strings.Join(quoteAll(invalid), ", ")))
	}
	planned := len(valid) + len(invalid)
	valid, warnings := s.allowedRecipients(valid)
	r.Warnings = append(r.Warnings, warnings...)
	r.To = valid

	rendered, err := templates.Render(tmpl, sub)
	if err != nil {
		// Deliver the alert anyway: an email in the default layout beats none.
		r.Warnings = append(r.Warnings, fmt.Sprintf("template %q failed (%v); sent with the built-in template", tmpl.Name, err))
		rendered, err = templates.Render(templates.Builtin(), sub)
	}

	var msg *delivery.Message
	switch {
	case err != nil:
	case len(valid) == 0 && planned > 0:
		err = fmt.Errorf("no valid recipients: %s", strings.Join(r.Warnings, "; "))
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

// Caller is who asks for a test email.
type Caller struct {
	// Email is the Grafana user's own address, if any.
	Email string
	// Admin callers may send tests to any address the allowed domains permit.
	Admin bool
}

// ErrTestRecipient is returned when an editor sends a test email to an
// address they are not allowed to use.
var ErrTestRecipient = errors.New("test emails from editors can only go to your own email address, the default recipients, a saved template's recipients, or an allowed recipient domain")

// testRecipientAllowed reports whether a non-admin may send a test to addr:
// a test carries HTML the caller wrote, so it must not reach strangers.
func (s *Service) testRecipientAllowed(addr string, c Caller) bool {
	key := strings.ToLower(addr)
	if c.Email != "" && key == strings.ToLower(c.Email) {
		return true
	}
	known := config.SplitAddresses(s.Settings.DefaultRecipients)
	for _, t := range s.Settings.Templates {
		known = append(known, config.SplitAddresses(t.Recipients)...)
	}
	for _, k := range known {
		if strings.ToLower(k) == key {
			return true
		}
	}
	return len(s.Settings.AllowedDomains()) > 0 && s.Settings.DomainAllowed(addr)
}

// SendTest renders t with sample data and sends it with a [TEST] subject.
func (s *Service) SendTest(ctx context.Context, t config.Template, to []string, c Caller) (history.Entry, error) {
	if len(to) == 0 {
		to = config.SplitAddresses(firstNonEmpty(t.Recipients, s.Settings.DefaultRecipients))
	}
	if !c.Admin {
		for _, a := range to {
			if !s.testRecipientAllowed(a, c) {
				return history.Entry{}, fmt.Errorf("%w (not allowed: %q)", ErrTestRecipient, a)
			}
		}
	}
	valid, invalid := config.ValidAddresses(to)
	if len(invalid) > 0 {
		return history.Entry{}, fmt.Errorf("invalid recipient %s", strings.Join(quoteAll(invalid), ", "))
	}
	to, warnings := s.allowedRecipients(valid)
	if len(to) == 0 {
		return history.Entry{}, fmt.Errorf("no recipients: %s", strings.Join(warnings, "; "))
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
		AlertStatus: sample.Status, Alerts: len(sample.Alerts), Message: &m, Warnings: warnings}, err), err
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
	to, warnings := s.allowedRecipients(m.To)
	if len(to) == 0 {
		return history.Entry{}, fmt.Errorf("no recipients: %s", strings.Join(warnings, "; "))
	}
	m.To = to
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
