// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"sync"
	"time"

	"v.io/x/ref/lib/stats"
	"v.io/x/ref/lib/stats/counter"
	"v.io/x/ref/lib/stats/histogram"

	"v.io/v23/naming"
)

type rpcStats struct {
	mu                  sync.RWMutex
	prefix              string
	methods             map[string]*perMethodStats
	blessingsCacheStats *blessingsCacheStats
}

func newRPCStats(prefix string) *rpcStats {
	return &rpcStats{
		prefix:              prefix,
		methods:             make(map[string]*perMethodStats),
		blessingsCacheStats: newBlessingsCacheStats(prefix),
	}
}

type perMethodStats struct {
	latency *histogram.Histogram
}

func (s *rpcStats) stop() {
	stats.Delete(s.prefix)
}

func (s *rpcStats) record(method string, latency time.Duration) {
	// Try first with a read lock. This will succeed in the most common
	// case. If it fails, try again with a write lock and create the stats
	// objects if they are still not there.
	s.mu.RLock()
	m, ok := s.methods[method]
	s.mu.RUnlock()
	if !ok {
		m = s.newPerMethodStats(method)
	}
	m.latency.Add(int64(latency / time.Millisecond))
}

func (s *rpcStats) recordBlessingCache(hit bool) {
	s.blessingsCacheStats.incr(hit)
}

// newPerMethodStats creates a new perMethodStats object if one doesn't exist
// already. It returns the newly created object, or the already existing one.
func (s *rpcStats) newPerMethodStats(method string) *perMethodStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.methods[method]
	if !ok {
		name := naming.Join(s.prefix, "methods", method, "latency-ms")
		s.methods[method] = &perMethodStats{
			latency: stats.NewHistogram(name, histogram.Options{
				NumBuckets:         25,
				GrowthFactor:       1,
				SmallestBucketSize: 1,
				MinValue:           0,
			}),
		}
		m = s.methods[method]
	}
	return m
}

// blessingsCacheStats keeps blessing cache hits and total calls received to determine
// how often the blessingCache is being used.
type blessingsCacheStats struct {
	callsReceived, cacheHits *counter.Counter
}

func newBlessingsCacheStats(prefix string) *blessingsCacheStats {
	cachePrefix := naming.Join(prefix, "security", "blessings", "cache")
	return &blessingsCacheStats{
		callsReceived: stats.NewCounter(naming.Join(cachePrefix, "attempts")),
		cacheHits:     stats.NewCounter(naming.Join(cachePrefix, "hits")),
	}
}

// Incr increments the cache attempt counter and the cache hit counter if hit is true.
func (s *blessingsCacheStats) incr(hit bool) {
	s.callsReceived.Incr(1)
	if hit {
		s.cacheHits.Incr(1)
	}
}
