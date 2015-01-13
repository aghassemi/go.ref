package vtrace

import (
	"encoding/binary"
	"reflect"
	"sort"
	"testing"

	"v.io/core/veyron2/uniqueid"
	"v.io/core/veyron2/vtrace"

	"v.io/core/veyron/lib/flags"
)

var nextid = uint64(1)

func id() uniqueid.ID {
	var out uniqueid.ID
	binary.BigEndian.PutUint64(out[8:], nextid)
	nextid++
	return out
}

func makeTraces(n int, st *Store) []uniqueid.ID {
	traces := make([]uniqueid.ID, n)
	for i := range traces {
		curid := id()
		traces[i] = curid
		st.ForceCollect(curid)
	}
	return traces
}

func recordids(records ...vtrace.TraceRecord) map[uniqueid.ID]bool {
	out := make(map[uniqueid.ID]bool)
	for _, trace := range records {
		out[trace.ID] = true
	}
	return out
}

func traceids(traces ...uniqueid.ID) map[uniqueid.ID]bool {
	out := make(map[uniqueid.ID]bool)
	for _, trace := range traces {
		out[trace] = true
	}
	return out
}

func pretty(in map[uniqueid.ID]bool) []int {
	out := make([]int, 0, len(in))
	for k, _ := range in {
		out = append(out, int(k[15]))
	}
	sort.Ints(out)
	return out
}

func compare(t *testing.T, want map[uniqueid.ID]bool, records []vtrace.TraceRecord) {
	got := recordids(records...)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Got wrong traces.  Got %v, want %v.", pretty(got), pretty(want))
	}
}

func TestTrimming(t *testing.T) {
	st, err := NewStore(flags.VtraceFlags{CacheSize: 5})
	if err != nil {
		t.Fatalf("Could not create store: %v", err)
	}
	traces := makeTraces(10, st)

	compare(t, traceids(traces[5:]...), st.TraceRecords())

	traces = append(traces, id(), id(), id())

	// Starting a span on an existing trace brings it to the front of the queue
	// and prevent it from being removed when a new trace begins.
	st.start(&span{trace: traces[5], id: id()})
	st.ForceCollect(traces[10])
	compare(t, traceids(traces[10], traces[5], traces[7], traces[8], traces[9]), st.TraceRecords())

	// Finishing a span on one of the traces should bring it back into the stored set.
	st.finish(&span{trace: traces[7], id: id()})
	st.ForceCollect(traces[11])
	compare(t, traceids(traces[10], traces[11], traces[5], traces[7], traces[9]), st.TraceRecords())

	// Annotating a span on one of the traces should bring it back into the stored set.
	st.annotate(&span{trace: traces[9], id: id()}, "hello")
	st.ForceCollect(traces[12])
	compare(t, traceids(traces[10], traces[11], traces[12], traces[7], traces[9]), st.TraceRecords())
}

func TestRegexp(t *testing.T) {
	traces := []uniqueid.ID{id(), id(), id()}

	type testcase struct {
		pattern string
		results []uniqueid.ID
	}
	tests := []testcase{
		{".*", traces},
		{"foo.*", traces},
		{".*bar", traces[1:2]},
		{".*bang", traces[2:3]},
	}

	for _, test := range tests {
		st, err := NewStore(flags.VtraceFlags{
			CacheSize:     10,
			CollectRegexp: test.pattern,
		})
		if err != nil {
			t.Fatalf("Could not create store: %v", err)
		}

		newSpan(traces[0], "foo", traces[0], st)
		newSpan(traces[1], "foobar", traces[1], st)
		sp := newSpan(traces[2], "baz", traces[2], st)
		sp.Annotate("foobang")

		compare(t, traceids(test.results...), st.TraceRecords())
	}
}
