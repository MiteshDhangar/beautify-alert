// Package notify turns a Grafana notification into sent emails: it plans
// the emails, renders each with its template, sends it and records the
// outcome in the delivery log.
package notify

import (
	"context"
	"errors"
	"fmt"
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
		tmpl, found := templates.Resolve(s.Settings, e.Template)
		sub := n.Subset(e.Alerts)
		r := EmailResult{Template: e.Template, TemplateFound: found, To: e.Recipients, Alerts: len(e.Alerts)}

		rendered, err := templates.Render(tmpl, sub)
		var msg *delivery.Message
		if err == nil {
			r.Subject = rendered.Subject
			m := delivery.Message{To: e.Recipients, Subject: rendered.Subject, HTML: rendered.HTML, Text: rendered.Text}
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
		entry := s.record(history.SourceAlert, e.Template, e.Recipients, r.Subject, sub.Status, len(e.Alerts), msg, err)
		r.ID = entry.ID
		res.Emails = append(res.Emails, r)
	}
	return res
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
	return s.record(history.SourceTest, name, to, m.Subject, sample.Status, len(sample.Alerts), &m, err), err
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
	return s.record(history.SourceResend, old.Template, m.To, m.Subject, old.AlertStatus, old.Alerts, &m, err), err
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

func (s *Service) record(source, tmpl string, to []string, subject, status string, alerts int, m *delivery.Message, err error) history.Entry {
	e := history.Entry{
		Source:      source,
		Template:    tmpl,
		To:          to,
		Subject:     subject,
		AlertStatus: status,
		Alerts:      alerts,
		OK:          err == nil,
		Message:     m,
	}
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
