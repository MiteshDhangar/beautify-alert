// Package config is the plugin's saved configuration: delivery settings,
// routing labels and templates. It is stored in the app plugin's jsonData,
// with secrets in secureJsonData.
package config

import (
	"bytes"
	"compress/gzip"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
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
	// SkipResolved drops emails that contain only resolved alerts. Useful for
	// rules whose query only returns rows in a time window: when the window
	// ends, every alert resolves at once, which is not news.
	SkipResolved bool `json:"skipResolved,omitempty"`
}

// InlinesCSS reports whether CSS inlining is on for t.
func (t Template) InlinesCSS() bool { return t.InlineCSS == nil || *t.InlineCSS }

// SMTP holds SMTP server settings.
type SMTP struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"-"`
	Security string `json:"security"`
	// CACertificate is a PEM certificate trusted in addition to the system's
	// roots: the server's own certificate if it is self-signed, or the
	// private CA that issued it. Certificates are always verified.
	CACertificate string `json:"caCertificate,omitempty"`
	// LegacySkipVerify is the "Skip certificate check" setting of versions
	// before 0.1.6. It no longer turns checks off; it only explains a
	// certificate error.
	LegacySkipVerify bool `json:"skipVerify,omitempty"`
}

// CertPool returns the system roots plus CACertificate, or nil when
// CACertificate is empty (meaning: the system roots).
func (s SMTP) CertPool() (*x509.CertPool, error) {
	if strings.TrimSpace(s.CACertificate) == "" {
		return nil, nil
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM([]byte(s.CACertificate)) {
		return nil, errors.New("the trusted certificate is not a PEM certificate; paste the text from -----BEGIN CERTIFICATE----- to -----END CERTIFICATE-----")
	}
	return pool, nil
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
	// TemplatesGz is Templates as gzip-compressed JSON in base64. The editor
	// saves templates this way because Grafana limits a plugin's settings to
	// 64 KB (MaxSettingsBytes). It replaces Templates when set. Provisioning
	// files can keep using plain Templates.
	TemplatesGz string `json:"templatesGz,omitempty"`
	// Threading groups the emails of one alert group into one email thread.
	// Nil means true.
	Threading *bool `json:"threading,omitempty"`
	// MaxAlertsPerEmail caps how many alerts one email lists; the rest are
	// counted as "and N more". Gmail cuts off emails larger than about
	// 102 KB, roughly 40-50 alerts with the default template.
	MaxAlertsPerEmail int `json:"maxAlertsPerEmail,omitempty"`
	// AllowedRecipientDomains limits every email to these domains, separated
	// by commas, such as "example.com, example.org". Empty allows any domain.
	AllowedRecipientDomains string `json:"allowedRecipientDomains,omitempty"`
	// MaxRecipientsPerEmail caps the recipients of one email.
	MaxRecipientsPerEmail int `json:"maxRecipientsPerEmail,omitempty"`
	// MaxEmailsPerNotification caps the emails one Grafana notification can
	// become, for example when every alert names its own recipients.
	MaxEmailsPerNotification int `json:"maxEmailsPerNotification,omitempty"`
}

// Defaults for the limits.
const (
	DefaultMaxAlertsPerEmail        = 40
	DefaultMaxRecipientsPerEmail    = 50
	DefaultMaxEmailsPerNotification = 20
)

// AllowedDomains returns the allowed recipient domains, lowercase, or nil
// when any domain is allowed.
func (s Settings) AllowedDomains() []string {
	var out []string
	for _, d := range strings.FieldsFunc(s.AllowedRecipientDomains, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\n' }) {
		if d = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(d), "@")); d != "" {
			out = append(out, d)
		}
	}
	return out
}

// DomainAllowed reports whether addr may receive email under
// AllowedRecipientDomains. Subdomains are not included: list each one.
func (s Settings) DomainAllowed(addr string) bool {
	domains := s.AllowedDomains()
	if len(domains) == 0 {
		return true
	}
	at := strings.LastIndex(addr, "@")
	if at < 0 {
		return false
	}
	d := strings.ToLower(strings.TrimRight(addr[at+1:], ">"))
	for _, allowed := range domains {
		if d == allowed {
			return true
		}
	}
	return false
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
	if out.TemplatesGz != "" {
		ts, err := DecodeTemplates(out.TemplatesGz)
		if err != nil {
			return out, fmt.Errorf("invalid compressed templates: %w", err)
		}
		out.Templates, out.TemplatesGz = ts, ""
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
	if s.MaxAlertsPerEmail <= 0 {
		s.MaxAlertsPerEmail = DefaultMaxAlertsPerEmail
	}
	if s.MaxRecipientsPerEmail <= 0 {
		s.MaxRecipientsPerEmail = DefaultMaxRecipientsPerEmail
	}
	if s.MaxEmailsPerNotification <= 0 {
		s.MaxEmailsPerNotification = DefaultMaxEmailsPerNotification
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
	if _, err := s.Delivery.SMTP.CertPool(); err != nil {
		return err
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

// MaxSettingsBytes is Grafana's limit on a plugin's stored jsonData (the size
// of a MySQL TEXT column, applied on every database).
const MaxSettingsBytes = 65535

// DecodeTemplates reads templates saved as base64 gzip-compressed JSON.
func DecodeTemplates(b64 string) ([]Template, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()
	// Templates are small; refuse anything that would expand absurdly.
	data, err := io.ReadAll(io.LimitReader(zr, 16<<20))
	if err != nil {
		return nil, err
	}
	var ts []Template
	if err := json.Unmarshal(data, &ts); err != nil {
		return nil, err
	}
	return ts, nil
}

// EncodeTemplates is the inverse of DecodeTemplates.
func EncodeTemplates(ts []Template) (string, error) {
	data, err := json.Marshal(ts)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if _, err := zw.Write(data); err != nil {
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// ValidAddresses splits addrs like SplitAddresses and separates addresses
// that can be sent to from ones that cannot be parsed.
func ValidAddresses(addrs []string) (valid, invalid []string) {
	for _, a := range addrs {
		if parsed, err := mail.ParseAddress(a); err == nil && strings.Contains(parsed.Address, "@") {
			valid = append(valid, a)
		} else {
			invalid = append(invalid, a)
		}
	}
	return valid, invalid
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
