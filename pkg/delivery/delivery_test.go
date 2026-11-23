package delivery

import (
	"strings"
	"testing"
	"time"
)

func TestHeadersThreadByKey(t *testing.T) {
	now := time.Unix(1, 0)
	a := Headers(Message{Subject: "[FIRING] x", ThreadKey: "group-1"}, "alerts@example.com", now)
	b := Headers(Message{Subject: "[RESOLVED] x", ThreadKey: "group-1"}, "alerts@example.com", now.Add(time.Second))
	c := Headers(Message{Subject: "y", ThreadKey: "group-2"}, "alerts@example.com", now)
	if a["References"] != b["References"] || a["References"] == c["References"] {
		t.Errorf("references: %q %q %q", a["References"], b["References"], c["References"])
	}
	if a["Message-ID"] == b["Message-ID"] {
		t.Error("message IDs must differ")
	}
	if !strings.HasSuffix(a["Message-ID"], "@example.com>") {
		t.Errorf("message ID domain: %s", a["Message-ID"])
	}
	if _, ok := Headers(Message{}, "a@b.c", now)["References"]; ok {
		t.Error("no thread key should mean no References")
	}
}
