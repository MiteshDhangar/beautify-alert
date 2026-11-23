package config

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func TestLoadAppliesDefaults(t *testing.T) {
	s, err := Load(backend.AppInstanceSettings{
		JSONData:                json.RawMessage(`{"delivery":{"smtp":{"host":"smtp.x.com","security":"tls"}}}`),
		DecryptedSecureJSONData: map[string]string{SecureSMTPPassword: "pw"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if s.Delivery.Provider != ProviderSMTP || s.Delivery.SMTP.Port != 465 || s.Delivery.SMTP.Password != "pw" {
		t.Errorf("delivery %+v", s.Delivery)
	}
	if s.Routing.TemplateLabel != DefaultTemplateLabel || s.Routing.RecipientsLabel != DefaultRecipientsLabel {
		t.Errorf("routing %+v", s.Routing)
	}
	if !s.ThreadsEmails() {
		t.Error("threading should default to on")
	}
}

func TestLoadRejectsBadJSON(t *testing.T) {
	if _, err := Load(backend.AppInstanceSettings{JSONData: json.RawMessage(`{"templates": 3}`)}); err == nil {
		t.Error("want error")
	}
}

func TestCheckDelivery(t *testing.T) {
	var s Settings
	s.applyDefaults()
	err := s.CheckDelivery()
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("err = %v", err)
	}
	if want := "from address and SMTP host"; !strings.Contains(err.Error(), want) {
		t.Errorf("err %q should name %q", err, want)
	}
	s.Delivery.FromAddress, s.Delivery.SMTP.Host = "a@b.c", "smtp"
	if err := s.CheckDelivery(); err != nil {
		t.Error(err)
	}
}

func TestSplitAddresses(t *testing.T) {
	got := SplitAddresses(" a@x.com, b@x.com;\nA@x.com ,, ")
	if len(got) != 2 || got[0] != "a@x.com" || got[1] != "b@x.com" {
		t.Errorf("got %q", got)
	}
}

func TestValidateTemplateName(t *testing.T) {
	for _, ok := range []string{"default", "stock-low", "p1_db", "a"} {
		if err := ValidateTemplateName(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "Upper", "-lead", "has space", string(make([]byte, 65))} {
		if ValidateTemplateName(bad) == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}
