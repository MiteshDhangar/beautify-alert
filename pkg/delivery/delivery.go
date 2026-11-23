// Package delivery sends rendered emails. Each email provider implements
// Sender; SMTP is the first. New providers (Amazon SES, Microsoft Graph,
// SendGrid, ...) plug in through New without touching callers.
package delivery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/MiteshDhangar/beautify-alert/pkg/config"
)

// Message is one email ready to send.
type Message struct {
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
	Text    string   `json:"text"`
	// ThreadKey groups messages into one email thread. Messages with the same
	// key get the same References header. Empty means no threading.
	ThreadKey string `json:"threadKey,omitempty"`
}

// Sender sends messages through one provider.
type Sender interface {
	// Send delivers m. It returns an error the user can act on.
	Send(ctx context.Context, m Message) error
	// Verify connects and authenticates without sending anything.
	Verify(ctx context.Context) error
}

// New returns the Sender for the configured provider.
func New(d config.Delivery) (Sender, error) {
	switch d.Provider {
	case config.ProviderSMTP, "":
		return &SMTP{cfg: d}, nil
	default:
		return nil, fmt.Errorf("unknown email provider %q", d.Provider)
	}
}

// Headers returns the threading headers for m: a unique Message-ID and, when
// m has a ThreadKey, References and In-Reply-To pointing at one stable root
// ID, so mail clients show the firing and resolved emails of an alert group
// as one conversation.
func Headers(m Message, fromAddress string, now time.Time) map[string]string {
	domain := "beautify-alert.local"
	if at := strings.LastIndex(fromAddress, "@"); at >= 0 && at < len(fromAddress)-1 {
		domain = fromAddress[at+1:]
	}
	h := map[string]string{
		"Message-ID":               fmt.Sprintf("<ba.%d.%s@%s>", now.UnixNano(), shortHash(m.Subject+strings.Join(m.To, ",")), domain),
		"X-Mailer":                 "Beautify Alert for Grafana",
		"Auto-Submitted":           "auto-generated",
		"X-Auto-Response-Suppress": "All",
	}
	if m.ThreadKey != "" {
		root := fmt.Sprintf("<ba.thread.%s@%s>", shortHash(m.ThreadKey), domain)
		h["References"] = root
		h["In-Reply-To"] = root
		// Outlook groups by Thread-Topic.
		h["Thread-Topic"] = "beautify-alert " + shortHash(m.ThreadKey)
	}
	return h
}

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}
