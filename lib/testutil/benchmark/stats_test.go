package benchmark_test

import (
	"strings"
	"testing"
	"time"

	"v.io/x/ref/lib/testutil/benchmark"
)

func TestStatsBasic(t *testing.T) {
	stats := benchmark.NewStats(16)
	if !strings.Contains(stats.String(), "Histogram (empty)") {
		t.Errorf("unexpect stats output:\n%s\n", stats.String())
	}

	for i := time.Duration(1); i <= 10; i++ {
		stats.Add(i * time.Millisecond)
	}

	if !strings.Contains(stats.String(), "Count: 10 ") {
		t.Errorf("unexpect stats output:\n%s\n", stats.String())
	}

	stats.Clear()
	if !strings.Contains(stats.String(), "Histogram (empty)") {
		t.Errorf("unexpect stats output:\n%s\n", stats.String())
	}
}
