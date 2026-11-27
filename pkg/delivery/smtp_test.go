package delivery

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mail "github.com/wneessen/go-mail"

	"github.com/MiteshDhangar/beautify-alert/pkg/config"
)

// fakeSMTP accepts SMTP sessions on a local port. The first failFirst
// connections are closed straight away, like a server that is restarting.
func fakeSMTP(t *testing.T, failFirst int32) (port int, delivered *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	var conns atomic.Int32
	delivered = &atomic.Int32{}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			if conns.Add(1) <= failFirst {
				_ = c.Close()
				continue
			}
			go serve(c, delivered)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, delivered
}

func serve(c net.Conn, delivered *atomic.Int32) {
	defer func() { _ = c.Close() }()
	r := bufio.NewReader(c)
	w := func(s string) { _, _ = fmt.Fprintf(c, "%s\r\n", s) }
	w("220 fake ESMTP")
	inData := false
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if inData {
			if line == "." {
				inData = false
				delivered.Add(1)
				w("250 queued")
			}
			continue
		}
		switch cmd := strings.ToUpper(strings.SplitN(line, " ", 2)[0]); cmd {
		case "EHLO", "HELO":
			w("250 fake")
		case "MAIL", "RCPT", "RSET", "NOOP":
			w("250 ok")
		case "DATA":
			inData = true
			w("354 go ahead")
		case "QUIT":
			w("221 bye")
			return
		default:
			w("502 unknown")
		}
	}
}

func testSMTP(port int) *SMTP {
	return &SMTP{cfg: config.Delivery{Provider: "smtp", FromAddress: "a@example.com", SMTP: config.SMTP{Host: "127.0.0.1", Port: port, Security: config.SecurityNone}}}
}

func TestSendRetriesRefusedConnection(t *testing.T) {
	old := retryDelays
	retryDelays = []time.Duration{10 * time.Millisecond, 10 * time.Millisecond}
	t.Cleanup(func() { retryDelays = old })

	port, delivered := fakeSMTP(t, 2)
	err := testSMTP(port).Send(context.Background(), Message{To: []string{"b@example.com"}, Subject: "s", HTML: "<p>h</p>", Text: "t"})
	if err != nil || delivered.Load() != 1 {
		t.Fatalf("err %v delivered %d", err, delivered.Load())
	}
}

func TestSendGivesUpAfterRetries(t *testing.T) {
	old := retryDelays
	retryDelays = []time.Duration{10 * time.Millisecond}
	t.Cleanup(func() { retryDelays = old })

	port, delivered := fakeSMTP(t, 5)
	err := testSMTP(port).Send(context.Background(), Message{To: []string{"b@example.com"}, Subject: "s", HTML: "h", Text: "t"})
	if err == nil || delivered.Load() != 0 || !strings.Contains(err.Error(), fmt.Sprintf("127.0.0.1:%d", port)) {
		t.Fatalf("err %v delivered %d", err, delivered.Load())
	}
}

func TestRetryable(t *testing.T) {
	cases := map[error]bool{
		errors.New("dial tcp 127.0.0.1:25: connect: connection refused"): true,
		errors.New("i/o timeout"):                                         true,
		mail.ErrNoSupportedAuthDiscovered:                                 false,
		errors.New("535 5.7.8 Username and Password not accepted (auth)"): false,
		errors.New("tls: failed to verify certificate: x509: ..."):        false,
	}
	for err, want := range cases {
		if got := retryable(err); got != want {
			t.Errorf("retryable(%q) = %v, want %v", err, got, want)
		}
	}
}

func TestExplainAuthDiscovery(t *testing.T) {
	err := testSMTP(25).explain(fmt.Errorf("dial failed: %w", mail.ErrNoSupportedAuthDiscovered))
	if !strings.Contains(err.Error(), "clear the username") || !errors.Is(err, mail.ErrNoSupportedAuthDiscovered) {
		t.Errorf("explain = %v", err)
	}
}

func TestExplainHidesServerBytesAndMatchesRealAuthErrors(t *testing.T) {
	s := &SMTP{cfg: config.Delivery{SMTP: config.SMTP{Host: "db.internal", Port: 3306}}}
	got := s.explain(errors.New("short response: \"J\x00\x00\x008.4.11\x00secret-banner\"")).Error()
	if strings.Contains(got, "J") || strings.Contains(got, "secret-banner") || !strings.Contains(got, "did not answer like an SMTP server") {
		t.Errorf("non-SMTP server: %q", got)
	}
	got = s.explain(errors.New(`sending SMTP RCPT TO command: 452 "4.5.3 Too many recipients", u535@outside.test`)).Error()
	if strings.Contains(got, "username or password") || !strings.Contains(got, "this many recipients") {
		t.Errorf("452: %q", got)
	}
	got = s.explain(errors.New("authentication failed: 535 5.7.8 Username and Password not accepted")).Error()
	if !strings.Contains(got, "username or password") {
		t.Errorf("535: %q", got)
	}
	got = s.explain(errors.New("reply: 550 bad\r\nX-Injected: 1" + strings.Repeat("a", 2000))).Error()
	if strings.ContainsAny(got, "\r\n") || len(got) > maxErrorLength+10 {
		t.Errorf("control characters or length: %q", got[:80])
	}
	if retryable(errors.New(`452 "4.5.3 Too many recipients"`)) || retryable(errors.New("short response: x")) {
		t.Error("should not retry")
	}
}
