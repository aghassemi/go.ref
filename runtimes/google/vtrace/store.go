package vtrace

import (
	"math/rand"
	"sync"
	"time"

	"v.io/core/veyron2/context"
	"v.io/core/veyron2/uniqueid"
	"v.io/core/veyron2/vtrace"

	"v.io/core/veyron/lib/flags"
)

// Store implements a store for traces.  The idea is to keep all the
// information we have about some subset of traces that pass through
// the server.  For now we just implement an LRU cache, so the least
// recently started/finished/annotated traces expire after some
// maximum trace count is reached.
// TODO(mattr): LRU is the wrong policy in the long term, we should
// try to keep some diverse set of traces and allow users to
// specifically tell us to capture a specific trace.  LRU will work OK
// for many testing scenarios and low volume applications.
type Store struct {
	opts flags.VtraceFlags

	// traces and head together implement a linked-hash-map.
	// head points to the head and tail of the doubly-linked-list
	// of recently used items (the tail is the LRU traceStore).
	// TODO(mattr): Use rwmutex.
	mu     sync.Mutex
	traces map[uniqueid.ID]*traceStore // GUARDED_BY(mu)
	head   *traceStore                 // GUARDED_BY(mu)
}

// NewStore creates a new store according to the passed in opts.
func NewStore(opts flags.VtraceFlags) *Store {
	head := &traceStore{}
	head.next, head.prev = head, head

	return &Store{
		opts:   opts,
		traces: make(map[uniqueid.ID]*traceStore),
		head:   head,
	}
}

func (s *Store) ForceCollect(id uniqueid.ID) {
	s.mu.Lock()
	s.forceCollectLocked(id)
	s.mu.Unlock()
}

func (s *Store) forceCollectLocked(id uniqueid.ID) *traceStore {
	ts := s.traces[id]
	if ts == nil {
		ts = newTraceStore(id)
		s.traces[id] = ts
		ts.moveAfter(s.head)
		// Trim elements beyond our size limit.
		for len(s.traces) > s.opts.CacheSize {
			el := s.head.prev
			el.removeFromList()
			delete(s.traces, el.id)
		}
	}
	return ts
}

// Merge merges a vtrace.Response into the current store.
func (s *Store) merge(t vtrace.Response) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var ts *traceStore
	if t.Method == vtrace.InMemory {
		ts = s.forceCollectLocked(t.Trace.ID)
	} else {
		ts = s.traces[t.Trace.ID]
	}
	if ts != nil {
		ts.merge(t.Trace.Spans)
	}
}

// annotate stores an annotation for the trace if it is being collected.
func (s *Store) annotate(span *span, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ts := s.traces[span.Trace()]; ts != nil {
		ts.annotate(span, msg)
		ts.moveAfter(s.head)
	}
}

// start stores data about a starting span if the trace is being collected.
func (s *Store) start(span *span) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var ts *traceStore
	sr := s.opts.SampleRate
	// If this is a root span, we may automatically sample it for collection.
	if span.trace == span.parent && sr > 0.0 && (sr >= 1.0 || rand.Float64() < sr) {
		ts = s.forceCollectLocked(span.Trace())
	} else {
		ts = s.traces[span.Trace()]
	}
	if ts != nil {
		ts.start(span)
		ts.moveAfter(s.head)
	}
}

// finish stores data about a finished span if the trace is being collected.
func (s *Store) finish(span *span) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ts := s.traces[span.Trace()]; ts != nil {
		ts.finish(span)
		ts.moveAfter(s.head)
	}
}

// method returns the collection method for the given trace.
func (s *Store) method(id uniqueid.ID) vtrace.TraceMethod {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ts := s.traces[id]; ts != nil {
		return vtrace.InMemory
	}
	return vtrace.None
}

// TraceRecords returns TraceRecords for all traces saved in the store.
func (s *Store) TraceRecords() []vtrace.TraceRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]vtrace.TraceRecord, len(s.traces))
	i := 0
	for _, ts := range s.traces {
		ts.traceRecord(&out[i])
		i++
	}
	return out
}

// TraceRecord returns a TraceRecord for a given ID.  Returns
// nil if the given id is not present.
func (s *Store) TraceRecord(id uniqueid.ID) *vtrace.TraceRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := &vtrace.TraceRecord{}
	ts := s.traces[id]
	if ts != nil {
		ts.traceRecord(out)
	}
	return out
}

type traceStore struct {
	id         uniqueid.ID
	spans      map[uniqueid.ID]*vtrace.SpanRecord
	prev, next *traceStore
}

func newTraceStore(id uniqueid.ID) *traceStore {
	return &traceStore{
		id:    id,
		spans: make(map[uniqueid.ID]*vtrace.SpanRecord),
	}
}

func (ts *traceStore) record(s *span) *vtrace.SpanRecord {
	record, ok := ts.spans[s.id]
	if !ok {
		record = &vtrace.SpanRecord{
			ID:     s.id,
			Parent: s.parent,
			Name:   s.name,
			Start:  s.start.UnixNano(),
		}
		ts.spans[s.id] = record
	}
	return record
}

func (ts *traceStore) annotate(s *span, msg string) {
	record := ts.record(s)
	record.Annotations = append(record.Annotations, vtrace.Annotation{
		When:    time.Now().UnixNano(),
		Message: msg,
	})
}

func (ts *traceStore) start(s *span) {
	ts.record(s)
}

func (ts *traceStore) finish(s *span) {
	ts.record(s).End = time.Now().UnixNano()
}

func (ts *traceStore) merge(spans []vtrace.SpanRecord) {
	// TODO(mattr): We need to carefully merge here to correct for
	// clock skew and ordering.  We should estimate the clock skew
	// by assuming that children of parent need to start after parent
	// and end before now.
	for _, span := range spans {
		if ts.spans[span.ID] == nil {
			ts.spans[span.ID] = copySpanRecord(&span)
		}
	}
}

func (ts *traceStore) removeFromList() {
	if ts.prev != nil {
		ts.prev.next = ts.next
	}
	if ts.next != nil {
		ts.next.prev = ts.prev
	}
	ts.next = nil
	ts.prev = nil
}

func (ts *traceStore) moveAfter(prev *traceStore) {
	ts.removeFromList()
	ts.prev = prev
	ts.next = prev.next
	prev.next.prev = ts
	prev.next = ts
}

func copySpanRecord(in *vtrace.SpanRecord) *vtrace.SpanRecord {
	return &vtrace.SpanRecord{
		ID:          in.ID,
		Parent:      in.Parent,
		Name:        in.Name,
		Start:       in.Start,
		End:         in.End,
		Annotations: append([]vtrace.Annotation{}, in.Annotations...),
	}
}

func (ts *traceStore) traceRecord(out *vtrace.TraceRecord) {
	spans := make([]vtrace.SpanRecord, 0, len(ts.spans))
	for _, span := range ts.spans {
		spans = append(spans, *copySpanRecord(span))
	}
	out.ID = ts.id
	out.Spans = spans
}

// Merge merges a vtrace.Response into the current store.
func Merge(ctx *context.T, t vtrace.Response) {
	getStore(ctx).merge(t)
}
