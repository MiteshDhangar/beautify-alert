// Package routing decides which emails one Grafana notification becomes:
// which template each alert uses and who receives it.
package routing

import (
	"sort"
	"strings"

	"github.com/MiteshDhangar/beautify-alert/pkg/alert"
	"github.com/MiteshDhangar/beautify-alert/pkg/config"
)

// Overrides come from the webhook URL's query string (?template=&to=) and win
// over everything else. They let one contact point force a template.
type Overrides struct {
	Template   string
	Recipients string
}

// Email is one email to render and send.
type Email struct {
	Template   string       `json:"template"`
	Recipients []string     `json:"recipients"`
	Alerts     alert.Alerts `json:"-"`
}

// Plan splits a notification into one email per (template, recipients) pair,
// keeping the order alerts arrived in.
//
// For each alert:
//   - template: override, else the template label, else the template
//     annotation, else "default";
//   - recipients: override, else the recipients label, else the recipients
//     annotation, else the template's own recipients, else the default
//     recipients.
func Plan(n alert.Notification, s config.Settings, o Overrides) []Email {
	var emails []*Email
	byKey := map[string]*Email{}
	for _, a := range n.Alerts {
		name := firstNonEmpty(o.Template, a.Labels[s.Routing.TemplateLabel], a.Annotations[s.Routing.TemplateLabel], config.DefaultTemplateName)
		var templateRecipients string
		if t, ok := s.Template(name); ok {
			templateRecipients = t.Recipients
		}
		to := config.SplitAddresses(firstNonEmpty(
			o.Recipients,
			a.Labels[s.Routing.RecipientsLabel],
			a.Annotations[s.Routing.RecipientsLabel],
			templateRecipients,
			s.DefaultRecipients,
		))
		key := name + "|" + canonical(to)
		e, ok := byKey[key]
		if !ok {
			e = &Email{Template: name, Recipients: to}
			byKey[key] = e
			emails = append(emails, e)
		}
		e.Alerts = append(e.Alerts, a)
	}
	out := make([]Email, len(emails))
	for i, e := range emails {
		out[i] = *e
	}
	return out
}

func canonical(addrs []string) string {
	c := make([]string, len(addrs))
	for i, a := range addrs {
		c[i] = strings.ToLower(a)
	}
	sort.Strings(c)
	return strings.Join(c, ",")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}
