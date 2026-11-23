package delivery

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	mail "github.com/wneessen/go-mail"

	"github.com/MiteshDhangar/beautify-alert/pkg/config"
)

const smtpTimeout = 30 * time.Second

// SMTP sends through an SMTP server.
type SMTP struct {
	cfg config.Delivery
	now func() time.Time
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

// Send implements Sender.
func (s *SMTP) Send(ctx context.Context, m Message) error {
	if len(m.To) == 0 {
		return fmt.Errorf("no recipients: set default recipients on the Configuration page, recipients on the template, or an email_to label on the alert")
	}
	msg := mail.NewMsg()
	if err := msg.FromFormat(s.cfg.FromName, s.cfg.FromAddress); err != nil {
		return fmt.Errorf("invalid from address %q: %w", s.cfg.FromAddress, err)
	}
	if err := msg.To(m.To...); err != nil {
		return fmt.Errorf("invalid recipient: %w", err)
	}
	if s.cfg.ReplyTo != "" {
		if err := msg.ReplyTo(s.cfg.ReplyTo); err != nil {
			return fmt.Errorf("invalid reply-to address %q: %w", s.cfg.ReplyTo, err)
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

	c, err := s.client()
	if err != nil {
		return fmt.Errorf("smtp: %w", err)
	}
	if err := c.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf("smtp %s:%d: %w", s.cfg.SMTP.Host, s.cfg.SMTP.Port, err)
	}
	return nil
}

// Verify implements Sender: it connects, negotiates TLS and authenticates.
func (s *SMTP) Verify(ctx context.Context) error {
	c, err := s.client()
	if err != nil {
		return fmt.Errorf("smtp: %w", err)
	}
	if err := c.DialWithContext(ctx); err != nil {
		return fmt.Errorf("smtp %s:%d: %w", s.cfg.SMTP.Host, s.cfg.SMTP.Port, err)
	}
	return c.Close()
}

func trimAngles(s string) string {
	if len(s) >= 2 && s[0] == '<' && s[len(s)-1] == '>' {
		return s[1 : len(s)-1]
	}
	return s
}
