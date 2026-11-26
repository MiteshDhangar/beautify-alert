package delivery

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"time"

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

// tlsConfig verifies the server certificate unless the admin turned on
// "Skip certificate check" for an SMTP server with a self-signed certificate
// (the same option as skip_verify in Grafana's own [smtp] settings).
func tlsConfig(c config.SMTP) *tls.Config {
	cfg := &tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12}
	if c.SkipVerify {
		cfg.InsecureSkipVerify = true // #nosec G402 -- explicit admin opt-in, off by default
	}
	return cfg
}

func (s *SMTP) client() (*mail.Client, error) {
	c := s.cfg.SMTP
	opts := []mail.Option{
		mail.WithPort(c.Port),
		mail.WithTimeout(smtpTimeout),
		mail.WithTLSConfig(tlsConfig(c)),
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
	for _, permanent := range []string{"auth", "certificate", "tls", "starttls", "x509"} {
		if strings.Contains(msg, permanent) {
			return false
		}
	}
	return true
}

// explain adds the server and, for common mistakes, what to change.
func (s *SMTP) explain(err error) error {
	where := fmt.Sprintf("smtp %s:%d", s.cfg.SMTP.Host, s.cfg.SMTP.Port)
	switch {
	case errors.Is(err, mail.ErrNoSupportedAuthDiscovered):
		return fmt.Errorf("%s: the server offered no login method on this connection. If it needs a login, set Security to STARTTLS or TLS; if it does not, clear the username: %w", where, err)
	case strings.Contains(err.Error(), "does not support STARTTLS"):
		return fmt.Errorf("%s: the server does not support STARTTLS. Use TLS for port 465, or None for a local relay: %w", where, err)
	case strings.Contains(err.Error(), "535") || strings.Contains(strings.ToLower(err.Error()), "username and password not accepted"):
		return fmt.Errorf("%s: the server rejected the username or password. For Gmail and Microsoft 365, use an app password: %w", where, err)
	}
	return fmt.Errorf("%s: %w", where, err)
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
