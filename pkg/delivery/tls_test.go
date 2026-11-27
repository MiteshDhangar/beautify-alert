package delivery

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MiteshDhangar/beautify-alert/pkg/config"
)

// selfSigned returns a self-signed certificate for 127.0.0.1, as a mail
// server on a private network might have, and its PEM text.
func selfSigned(t *testing.T) (tls.Certificate, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "mail.internal"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key},
		string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

// fakeTLSSMTP serves SMTP over implicit TLS (port 465 style) with cert.
func fakeTLSSMTP(t *testing.T, cert tls.Certificate) (int, *atomic.Int32) {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	delivered := &atomic.Int32{}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go serve(c, delivered)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, delivered
}

func tlsSender(port int, smtp config.SMTP) *SMTP {
	smtp.Host, smtp.Port, smtp.Security = "127.0.0.1", port, config.SecurityTLS
	return &SMTP{cfg: config.Delivery{Provider: "smtp", FromAddress: "a@example.com", SMTP: smtp}}
}

func TestSelfSignedServerNeedsItsCertificate(t *testing.T) {
	cert, certPEM := selfSigned(t)
	port, delivered := fakeTLSSMTP(t, cert)
	ctx := context.Background()
	msg := Message{To: []string{"b@example.com"}, Subject: "s", HTML: "<p>x</p>", Text: "x"}

	// Without the certificate the connection is refused: checks are always on.
	err := tlsSender(port, config.SMTP{}).Verify(ctx)
	if err == nil || !strings.Contains(err.Error(), "could not be verified") || !strings.Contains(err.Error(), "trusted certificate") {
		t.Fatalf("untrusted: %v", err)
	}
	// The old "Skip certificate check" setting no longer skips anything, and says so.
	err = tlsSender(port, config.SMTP{LegacySkipVerify: true}).Verify(ctx)
	if err == nil || !strings.Contains(err.Error(), "was removed in 0.1.6") {
		t.Fatalf("legacy skip: %v", err)
	}
	// With the server's certificate pasted, it connects and sends.
	s := tlsSender(port, config.SMTP{CACertificate: certPEM})
	if err := s.Verify(ctx); err != nil {
		t.Fatalf("trusted verify: %v", err)
	}
	if err := s.Send(ctx, msg); err != nil || delivered.Load() != 1 {
		t.Fatalf("trusted send: %v, delivered %d", err, delivered.Load())
	}
	// A different certificate does not help.
	_, otherPEM := selfSigned(t)
	if err := tlsSender(port, config.SMTP{CACertificate: otherPEM}).Verify(ctx); err == nil {
		t.Fatal("other certificate was accepted")
	}
}

func TestInvalidTrustedCertificate(t *testing.T) {
	s := config.Settings{Delivery: config.Delivery{Provider: "smtp", FromAddress: "a@example.com",
		SMTP: config.SMTP{Host: "h", CACertificate: "not a certificate"}}}
	if err := s.CheckDelivery(); err == nil || !strings.Contains(err.Error(), "not a PEM certificate") {
		t.Errorf("err = %v", err)
	}
}
