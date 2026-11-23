package history

import (
	"fmt"
	"strings"
	"testing"

	"github.com/MiteshDhangar/beautify-alert/pkg/delivery"
)

func TestRingKeepsNewestFirst(t *testing.T) {
	l := New()
	for i := 0; i < Capacity+10; i++ {
		l.Add(Entry{Subject: fmt.Sprint(i)})
	}
	got := l.List(0)
	if len(got) != Capacity {
		t.Fatalf("len %d", len(got))
	}
	if got[0].Subject != fmt.Sprint(Capacity+9) || got[len(got)-1].Subject != "10" {
		t.Errorf("order: first %s last %s", got[0].Subject, got[len(got)-1].Subject)
	}
	if l.List(3)[2].Subject != fmt.Sprint(Capacity+7) {
		t.Error("limit")
	}
}

func TestLargeBodiesAreDropped(t *testing.T) {
	l := New()
	small := l.Add(Entry{Message: &delivery.Message{HTML: "<p>x</p>"}})
	big := l.Add(Entry{Message: &delivery.Message{HTML: strings.Repeat("x", maxStoredBody+1)}})
	if e, _ := l.Get(small.ID); e.Message == nil {
		t.Error("small body dropped")
	}
	if e, _ := l.Get(big.ID); e.Message != nil {
		t.Error("big body kept")
	}
	if l.List(1)[0].Message != nil {
		t.Error("List returned a body")
	}
}
