package delivery

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	mail "github.com/wneessen/go-mail"

	"github.com/MiteshDhangar/beautify-alert/pkg/config"
)

// Grafana's webhook contact point does not retry, so the sender retries
// short failures itself. Attempts and timeouts stay under Grafana's 30 second
// webhook timeout.
var (
	smtpTimeout = 8 * time.Second
	retryDelays = []time.Duration{1 * time.Second, 3 * time.Second}
)

// SMTP sends through an SMTP server.
type SMTP struct {
	cfg config.Delivery
	now func() time.Time
}

// tlsConfig always verifies the server certificate. Servers with a
// self-signed or private certificate work when the admin pastes that
// certificate (or its CA) as the trusted certificate.
func tlsConfig(c config.SMTP) (*tls.Config, error) {
	pool, err := c.CertPool()
	if err != nil {
		return nil, err
	}
	return &tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12, RootCAs: pool}, nil
}

func (s *SMTP) client() (*mail.Client, error) {
	c := s.cfg.SMTP
	tc, err := tlsConfig(c)
	if err != nil {
		return nil, err
	}
	opts := []mail.Option{
		mail.WithPort(c.Port),
		mail.WithTimeout(smtpTimeout),
		mail.WithTLSConfig(tc),
	}
	switch c.Security {
	case config.SecurityTLS:
		opts = append(opts, mail.WithSSL())
	case config.SecurityNone:
		opts = append(opts, mail.WithTLSPolicy(mail.NoTLS))
	default:
		opts = append(opts, mail.WithTLSPolicy(mail.TLSMandatory))
	}
	if c.Username != "" {
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
			mail.WithUsername(c.Username),
			mail.WithPassword(c.Password),
		)
	}
	return mail.NewClient(c.Host, opts...)
}

// Send implements Sender. It retries failures that may pass on a second try,
// such as a refused connection or a temporary SMTP error.
func (s *SMTP) Send(ctx context.Context, m Message) error {
	msg, err := s.message(m)
	if err != nil {
		return err
	}
	for attempt := 0; ; attempt++ {
		err = s.sendOnce(ctx, msg)
		if err == nil || !retryable(err) || attempt >= len(retryDelays) {
			break
		}
		select {
		case <-ctx.Done():
			return s.explain(err)
		case <-time.After(retryDelays[attempt]):
		}
	}
	if err != nil {
		return s.explain(err)
	}
	return nil
}

func (s *SMTP) message(m Message) (*mail.Msg, error) {
	if len(m.To) == 0 {
		return nil, fmt.Errorf("no recipients: set default recipients on the Configuration page, recipients on the template, or an email_to label on the alert")
	}
	msg := mail.NewMsg()
	if err := msg.FromFormat(s.cfg.FromName, s.cfg.FromAddress); err != nil {
		return nil, fmt.Errorf("invalid from address %q: %w", s.cfg.FromAddress, err)
	}
	if err := msg.To(m.To...); err != nil {
		return nil, fmt.Errorf("invalid recipient: %w", err)
	}
	if s.cfg.ReplyTo != "" {
		if err := msg.ReplyTo(s.cfg.ReplyTo); err != nil {
			return nil, fmt.Errorf("invalid reply-to address %q: %w", s.cfg.ReplyTo, err)
		}
	}
	msg.Subject(m.Subject)
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	for k, v := range Headers(m, s.cfg.FromAddress, now()) {
		if k == "Message-ID" {
			msg.SetMessageIDWithValue(trimAngles(v))
			continue
		}
		msg.SetGenHeaderPreformatted(mail.Header(k), v)
	}
	msg.SetBodyString(mail.TypeTextPlain, m.Text)
	msg.AddAlternativeString(mail.TypeTextHTML, m.HTML)
	return msg, nil
}

func (s *SMTP) sendOnce(ctx context.Context, msg *mail.Msg) error {
	c, err := s.client()
	if err != nil {
		return err
	}
	return c.DialAndSendWithContext(ctx, msg)
}

// retryable reports whether err may pass on another attempt: network errors
// and temporary (4xx) SMTP replies. Wrong passwords, rejected recipients and
// TLS mismatches are not retried.
func retryable(err error) bool {
	var se *mail.SendError
	if errors.As(err, &se) {
		return se.IsTemp()
	}
	if errors.Is(err, mail.ErrNoSupportedAuthDiscovered) {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, permanent := range []string{"auth", "certificate", "tls", "starttls", "x509", "too many recipients", "4.5.3", "short response", "invalid response", "malformed"} {
		if strings.Contains(msg, permanent) {
			return false
		}
	}
	return true
}

// authRejectedRe matches an SMTP 535 reply ("535 5.7.8 ..." or "535-..."),
// not 535 inside an address or a number.
var authRejectedRe = regexp.MustCompile(`(?:^|[\s:"])535[ -]`)

// explain adds the server and, for common mistakes, what to change. The
// message never includes raw bytes from the server: the host may not be an
// SMTP server at all, and its reply is not for the plugin's users to read.
func (s *SMTP) explain(err error) error {
	where := fmt.Sprintf("smtp %s:%d", s.cfg.SMTP.Host, s.cfg.SMTP.Port)
	text := err.Error()
	switch {
	case strings.Contains(text, "short response") || strings.Contains(text, "invalid response code") || strings.Contains(text, "malformed"):
		return fmt.Errorf("%s: the server did not answer like an SMTP server; check the host and port", where)
	case strings.Contains(strings.ToLower(text), "too many recipients") || strings.Contains(text, "4.5.3"):
		return cleanError{fmt.Errorf("%s: the server refused this many recipients in one email; lower \"Max recipients per email\" on the Configuration page: %w", where, err)}
	case errors.Is(err, mail.ErrNoSupportedAuthDiscovered):
		return fmt.Errorf("%s: the server offered no login method on this connection. If it needs a login, set Security to STARTTLS or TLS; if it does not, clear the username: %w", where, err)
	case strings.Contains(err.Error(), "does not support STARTTLS"):
		return fmt.Errorf("%s: the server does not support STARTTLS. Use TLS for port 465, or None for a local relay: %w", where, err)
	case strings.Contains(text, "x509") || strings.Contains(strings.ToLower(text), "certificate"):
		hint := "use the host name on the certificate, or paste the server's certificate (or its CA) as the trusted certificate on the Configuration page"
		if s.cfg.SMTP.LegacySkipVerify && strings.TrimSpace(s.cfg.SMTP.CACertificate) == "" {
			hint = "\"Skip certificate check\" was removed in 0.1.6 because it made the connection open to interception; paste the server's certificate (or its CA) as the trusted certificate on the Configuration page instead"
		}
		return cleanError{fmt.Errorf("%s: the server's certificate could not be verified; %s: %w", where, hint, err)}
	case authRejectedRe.MatchString(text) || strings.Contains(strings.ToLower(text), "username and password not accepted"):
		return cleanError{fmt.Errorf("%s: the server rejected the username or password. For Gmail and Microsoft 365, use an app password: %w", where, err)}
	}
	return cleanError{fmt.Errorf("%s: %w", where, err)}
}

// maxErrorLength caps an error message, which may quote the server.
const maxErrorLength = 600

// cleanError is err with control characters replaced and the message
// shortened, so a server reply cannot inject lines or flood the UI.
type cleanError struct{ err error }

func (e cleanError) Unwrap() error { return e.err }

func (e cleanError) Error() string {
	msg := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == utf8.RuneError {
			return ' '
		}
		return r
	}, e.err.Error())
	if len(msg) > maxErrorLength {
		msg = msg[:maxErrorLength] + "..."
	}
	return msg
}

// Verify implements Sender: it connects, negotiates TLS and authenticates.
func (s *SMTP) Verify(ctx context.Context) error {
	c, err := s.client()
	if err != nil {
		return s.explain(err)
	}
	if err := c.DialWithContext(ctx); err != nil {
		return s.explain(err)
	}
	return c.Close()
}

func trimAngles(s string) string {
	if len(s) >= 2 && s[0] == '<' && s[len(s)-1] == '>' {
		return s[1 : len(s)-1]
	}
	return s
}
