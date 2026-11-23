package templates

import (
	"embed"
	"fmt"

	"github.com/MiteshDhangar/beautify-alert/pkg/config"
)

//go:embed starters/*.html
var starterFS embed.FS

// DefaultSubject is used when a template has no subject.
const DefaultSubject = `[{{ .Status | upper }}{{ if eq .Status "firing" }}:{{ len .Alerts.Firing }}{{ end }}] {{ .CommonLabels.alertname | default "Grafana alert" }}`

// Starter is a built-in template users can start from.
type Starter struct {
	config.Template
	Title string `json:"title"`
}

var starters = []struct {
	file, name, title, description, subject string
}{
	{"default.html", config.DefaultTemplateName, "Cards", "One card per alert with status, severity, values, labels and links", DefaultSubject},
	{"compact.html", "compact", "Compact table", "One row per alert, for busy inboxes", `{{ .CommonLabels.alertname | default "Alerts" }}: {{ len .Alerts.Firing }} firing, {{ len .Alerts.Resolved }} resolved`},
	{"summary.html", "summary", "Summary", "Big status and counts, for managers and stakeholders", `{{ if eq .Status "firing" }}Action needed{{ else }}All clear{{ end }}: {{ .CommonLabels.alertname }}`},
	{"blank.html", "blank", "Blank", "A heading and one line per alert, to build your own", `{{ .CommonLabels.alertname }}`},
}

// Starters returns the built-in templates.
func Starters() []Starter {
	out := make([]Starter, 0, len(starters))
	for _, s := range starters {
		html, err := starterFS.ReadFile("starters/" + s.file)
		if err != nil {
			panic(fmt.Sprintf("missing embedded starter %s: %v", s.file, err))
		}
		out = append(out, Starter{
			Title: s.title,
			Template: config.Template{
				Name:        s.name,
				Description: s.description,
				Subject:     s.subject,
				HTML:        string(html),
			},
		})
	}
	return out
}

// Builtin returns the template used when an alert names a template that does
// not exist and no "default" template is saved.
func Builtin() config.Template {
	return Starters()[0].Template
}

// Resolve returns the template to use for name: the saved template, else the
// saved "default" template, else the built-in default. found is false when
// name itself does not exist.
func Resolve(s config.Settings, name string) (t config.Template, found bool) {
	if t, ok := s.Template(name); ok {
		return t, true
	}
	if t, ok := s.Template(config.DefaultTemplateName); ok {
		return t, false
	}
	return Builtin(), name == config.DefaultTemplateName
}
