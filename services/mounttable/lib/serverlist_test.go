package mounttable

import (
	"testing"
	"time"
)

type fakeTime struct {
	theTime time.Time
}

func (ft *fakeTime) now() time.Time {
	return ft.theTime
}
func (ft *fakeTime) advance(d time.Duration) {
	ft.theTime = ft.theTime.Add(d)
}
func NewFakeTimeClock() *fakeTime {
	return &fakeTime{theTime: time.Now()}
}

func TestServerList(t *testing.T) {
	eps := []string{
		"endpoint:adfasdf@@who",
		"endpoint:sdfgsdfg@@x/",
		"endpoint:sdfgsdfg@@y",
		"endpoint:dfgsfdg@@",
	}

	// Test adding entries.
	ft := NewFakeTimeClock()
	setServerListClock(ft)
	sl := NewServerList()
	for i, ep := range eps {
		sl.add(ep, time.Duration(5*i)*time.Second)
	}
	if sl.len() != len(eps) {
		t.Fatalf("got %d, want %d", sl.len(), len(eps))
	}

	// Test timing out entries.
	ft.advance(6 * time.Second)
	if sl.removeExpired() != len(eps)-2 {
		t.Fatalf("got %d, want %d", sl.len(), len(eps)-2)
	}

	// Test removing entries.
	sl.remove(eps[2])
	if sl.len() != len(eps)-3 {
		t.Fatalf("got %d, want %d", sl.len(), len(eps)-3)
	}
}
