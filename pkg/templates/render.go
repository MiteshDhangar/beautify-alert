// Package templates renders email templates.
//
// The HTML body uses Go's html/template: the template's own markup is kept
// and every value from an alert is escaped, so an annotation containing
// "<script>" shows as text. The subject uses text/template. Both get the
// Sprig function library (minus functions that read the environment or the
// network) and a few alert helpers.
package templates

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	"math"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/vanng822/go-premailer/premailer"

	"github.com/MiteshDhangar/beautify-alert/pkg/alert"
	"github.com/MiteshDhangar/beautify-alert/pkg/config"
)

// MaxOutputSize caps a rendered email so a runaway template cannot exhaust memory.
const MaxOutputSize = 5 << 20

// Rendered is a rendered email.
type Rendered struct {
	Subject string `json:"subject"`
	HTML    string `json:"html"`
	Text    string `json:"text"`
}

// unsafeSprigFuncs read the plugin's environment or reach the network.
var unsafeSprigFuncs = []string{"env", "expandenv", "getHostByName"}

// Helpers are the functions added on top of Sprig. Documented in docs/templates.md.
func helpers() map[string]any {
	return map[string]any{
		"statusColor":   StatusColor,
		"severityColor": SeverityColor,
		"formatTime":    formatTime,
		"since":         since,
		"humanize":      humanize,
	}
}

// StatusColor returns red for firing and green for anything else.
func StatusColor(status string) string {
	if status == alert.StatusFiring {
		return "#dc2626"
	}
	return "#16a34a"
}

// SeverityColor maps common severity label values to a color.
func SeverityColor(severity string) string {
	switch strings.ToLower(severity) {
	case "critical", "crit", "p1", "fatal", "emergency":
		return "#dc2626"
	case "high", "error", "major", "p2":
		return "#ea580c"
	case "warning", "warn", "medium", "p3":
		return "#d97706"
	case "info", "low", "minor", "p4", "notice":
		return "#2563eb"
	default:
		return "#6b7280"
	}
}

// formatTime formats t with a Go layout and an optional IANA time zone:
// {{ formatTime .StartsAt "02 Jan 2006 15:04 MST" "Asia/Kolkata" }}.
func formatTime(t time.Time, layout string, zone ...string) (string, error) {
	if t.IsZero() {
		return "", nil
	}
	if len(zone) > 0 && zone[0] != "" {
		loc, err := time.LoadLocation(zone[0])
		if err != nil {
			return "", fmt.Errorf("formatTime: unknown time zone %q", zone[0])
		}
		t = t.In(loc)
	}
	return t.Format(layout), nil
}

// since returns how long ago t was, rounded to the minute: "12m", "3h5m", "2d4h".
func since(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t).Round(time.Minute)
	if d < time.Minute {
		return "just now"
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd%dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh%dm", hours, mins)
	default:
		return fmt.Sprintf("%dm", mins)
	}
}

// humanize formats a number compactly: 1234 -> "1.23k", 0.5 -> "0.5".
func humanize(v float64) string {
	abs := math.Abs(v)
	for _, u := range []struct {
		div    float64
		suffix string
	}{{1e12, "T"}, {1e9, "G"}, {1e6, "M"}, {1e3, "k"}} {
		if abs >= u.div {
			return trimFloat(v/u.div) + u.suffix
		}
	}
	return trimFloat(v)
}

func trimFloat(v float64) string {
	s := fmt.Sprintf("%.3g", v)
	if strings.Contains(s, "e") {
		s = fmt.Sprintf("%.2f", v)
	}
	return s
}

func htmlFuncs() htmltemplate.FuncMap {
	f := sprig.HtmlFuncMap()
	for _, k := range unsafeSprigFuncs {
		delete(f, k)
	}
	for k, v := range helpers() {
		f[k] = v
	}
	// safeHTML inserts a value without escaping. Use it only on trusted content.
	f["safeHTML"] = func(s string) htmltemplate.HTML { return htmltemplate.HTML(s) } //nolint:gosec // documented opt-in
	return f
}

func textFuncs() texttemplate.FuncMap {
	f := sprig.TxtFuncMap()
	for _, k := range unsafeSprigFuncs {
		delete(f, k)
	}
	for k, v := range helpers() {
		f[k] = v
	}
	return f
}

// Error is a template error with the part that failed.
type Error struct {
	Part string // "subject" or "html"
	Err  error
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %v", e.Part, e.Err) }
func (e *Error) Unwrap() error { return e.Err }

// Render renders t with n.
func Render(t config.Template, n alert.Notification) (Rendered, error) {
	var out Rendered

	subjectSrc := t.Subject
	if strings.TrimSpace(subjectSrc) == "" {
		subjectSrc = DefaultSubject
	}
	st, err := texttemplate.New("subject").Funcs(textFuncs()).Option("missingkey=zero").Parse(subjectSrc)
	if err != nil {
		return out, &Error{Part: "subject", Err: err}
	}
	var sb limitedBuffer
	if err := st.Execute(&sb, n); err != nil {
		return out, &Error{Part: "subject", Err: err}
	}
	// A subject is one line.
	out.Subject = strings.Join(strings.Fields(sb.String()), " ")

	ht, err := htmltemplate.New("html").Funcs(htmlFuncs()).Option("missingkey=zero").Parse(t.HTML)
	if err != nil {
		return out, &Error{Part: "html", Err: err}
	}
	var hb limitedBuffer
	if err := ht.Execute(&hb, n); err != nil {
		return out, &Error{Part: "html", Err: err}
	}
	out.HTML = hb.String()
	if t.InlinesCSS() && strings.Contains(out.HTML, "<style") {
		inlined, err := inlineCSS(out.HTML)
		if err != nil {
			return out, &Error{Part: "html", Err: fmt.Errorf("inlining CSS: %w", err)}
		}
		out.HTML = inlined
	}
	out.Text = PlainText(out.Subject, n)
	return out, nil
}

func inlineCSS(html string) (string, error) {
	opts := premailer.NewOptions()
	opts.RemoveClasses = false
	opts.KeepBangImportant = true
	p, err := premailer.NewPremailerFromString(html, opts)
	if err != nil {
		return "", err
	}
	return p.Transform()
}

// PlainText builds the text/plain part sent with every email.
func PlainText(subject string, n alert.Notification) string {
	var b strings.Builder
	b.WriteString(subject + "\n\n")
	for _, a := range n.Alerts {
		fmt.Fprintf(&b, "[%s] %s\n", strings.ToUpper(a.Status), a.Labels["alertname"])
		if s := a.Annotations["summary"]; s != "" {
			b.WriteString(s + "\n")
		}
		if d := a.Annotations["description"]; d != "" {
			b.WriteString(d + "\n")
		}
		for _, kv := range a.Labels.SortedPairs() {
			fmt.Fprintf(&b, "  %s=%s\n", kv.Name, kv.Value)
		}
		if !a.StartsAt.IsZero() {
			b.WriteString("Started: " + a.StartsAt.UTC().Format(time.RFC1123) + "\n")
		}
		if a.DashboardURL != "" {
			b.WriteString("Dashboard: " + a.DashboardURL + "\n")
		}
		if a.GeneratorURL != "" {
			b.WriteString("Rule: " + a.GeneratorURL + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

type limitedBuffer struct{ bytes.Buffer }

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if l.Len()+len(p) > MaxOutputSize {
		return 0, fmt.Errorf("rendered output is larger than %d bytes", MaxOutputSize)
	}
	return l.Buffer.Write(p)
}
