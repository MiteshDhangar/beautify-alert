// Package config is the plugin's saved configuration: delivery settings,
// routing labels and templates. It is stored in the app plugin's jsonData,
// with secrets in secureJsonData.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// Defaults for routing labels. Both are configurable.
const (
	DefaultTemplateLabel   = "email_template"
	DefaultRecipientsLabel = "email_to"
	// DefaultTemplateName is used when an alert names no template.
	DefaultTemplateName = "default"
)

// Secure field names in secureJsonData.
const (
	SecureSMTPPassword = "smtpPassword"
)

// SMTP security modes.
const (
	SecurityStartTLS = "starttls"
	SecurityTLS      = "tls"
	SecurityNone     = "none"
)

// Provider names. Only SMTP is implemented; the field exists so more
// providers (SES, Microsoft Graph, ...) can be added without a migration.
const (
	ProviderSMTP = "smtp"
)

var templateNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// Template is one user-defined email template.
type Template struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Recipients is used when an alert has no recipients label.
	Recipients string `json:"recipients,omitempty"`
	Subject    string `json:"subject"`
	HTML       string `json:"html"`
	// InlineCSS moves <style> rules into style attributes before sending, for
	// email clients that ignore <style>. Nil means true.
	InlineCSS *bool `json:"inlineCss,omitempty"`
}

// InlinesCSS reports whether CSS inlining is on for t.
func (t Template) InlinesCSS() bool { return t.InlineCSS == nil || *t.InlineCSS }

// SMTP holds SMTP server settings.
type SMTP struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"-"`
	Security   string `json:"security"`
	SkipVerify bool   `json:"skipVerify"`
}

// Delivery holds how and from whom email is sent.
type Delivery struct {
	Provider    string `json:"provider"`
	FromAddress string `json:"fromAddress"`
	FromName    string `json:"fromName"`
	ReplyTo     string `json:"replyTo,omitempty"`
	SMTP        SMTP   `json:"smtp"`
}

// Routing names the alert labels that pick a template and recipients.
type Routing struct {
	TemplateLabel   string `json:"templateLabel"`
	RecipientsLabel string `json:"recipientsLabel"`
}

// Settings is the whole plugin configuration.
type Settings struct {
	Delivery          Delivery   `json:"delivery"`
	DefaultRecipients string     `json:"defaultRecipients"`
	Routing           Routing    `json:"routing"`
	Templates         []Template `json:"templates"`
	// Threading groups the emails of one alert group into one email thread.
	// Nil means true.
	Threading *bool `json:"threading,omitempty"`
}

// ThreadsEmails reports whether threading is on.
func (s Settings) ThreadsEmails() bool { return s.Threading == nil || *s.Threading }

// Load reads Settings from the app instance settings and applies defaults.
func Load(s backend.AppInstanceSettings) (Settings, error) {
	var out Settings
	if len(s.JSONData) > 0 {
		if err := json.Unmarshal(s.JSONData, &out); err != nil {
			return out, fmt.Errorf("invalid plugin settings: %w", err)
		}
	}
	out.Delivery.SMTP.Password = s.DecryptedSecureJSONData[SecureSMTPPassword]
	out.applyDefaults()
	return out, nil
}

func (s *Settings) applyDefaults() {
	if s.Delivery.Provider == "" {
		s.Delivery.Provider = ProviderSMTP
	}
	if s.Delivery.SMTP.Security == "" {
		s.Delivery.SMTP.Security = SecurityStartTLS
	}
	if s.Delivery.SMTP.Port == 0 {
		switch s.Delivery.SMTP.Security {
		case SecurityTLS:
			s.Delivery.SMTP.Port = 465
		case SecurityNone:
			s.Delivery.SMTP.Port = 25
		default:
			s.Delivery.SMTP.Port = 587
		}
	}
	if s.Routing.TemplateLabel == "" {
		s.Routing.TemplateLabel = DefaultTemplateLabel
	}
	if s.Routing.RecipientsLabel == "" {
		s.Routing.RecipientsLabel = DefaultRecipientsLabel
	}
}

// Template returns the template named name and whether it exists.
func (s Settings) Template(name string) (Template, bool) {
	for _, t := range s.Templates {
		if t.Name == name {
			return t, true
		}
	}
	return Template{}, false
}

// ErrNotConfigured is returned when email cannot be sent yet.
var ErrNotConfigured = errors.New("email delivery is not configured")

// CheckDelivery reports what is missing before email can be sent.
func (s Settings) CheckDelivery() error {
	var missing []string
	if s.Delivery.FromAddress == "" {
		missing = append(missing, "from address")
	}
	switch s.Delivery.Provider {
	case ProviderSMTP:
		if s.Delivery.SMTP.Host == "" {
			missing = append(missing, "SMTP host")
		}
	default:
		return fmt.Errorf("unknown email provider %q", s.Delivery.Provider)
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: set the %s on the plugin's Configuration page", ErrNotConfigured, strings.Join(missing, " and "))
	}
	return nil
}

// ValidateTemplateName reports whether name can be used as a template name.
func ValidateTemplateName(name string) error {
	if !templateNameRe.MatchString(name) {
		return fmt.Errorf("template name %q must be 1 to 64 lowercase letters, digits, - or _, starting with a letter or digit", name)
	}
	return nil
}

// SplitAddresses splits addresses separated by commas, semicolons or new
// lines, trims them and drops duplicates.
func SplitAddresses(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == '\n' })
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		key := strings.ToLower(p)
		if p != "" && !seen[key] {
			seen[key] = true
			out = append(out, p)
		}
	}
	return out
}
