package principal

import (
	"sync"

	"veyron.io/veyron/veyron2/security"
)

// JSBlessingsHandles is a store for Blessings in use by JS code.
//
// We don't pass the full Blessings object to avoid serializing
// and deserializing a potentially huge forest of blessings.
// Instead we pass to JS a handle to a Blessings object and have
// all operations involving cryptographic operations call into go.
type JSBlessingsHandles struct {
	mu         sync.Mutex
	lastHandle int64
	store      map[int64]security.Blessings
}

// NewJSBlessingsHandles returns a newly initialized JSBlessingsHandles
func NewJSBlessingsHandles() *JSBlessingsHandles {
	return &JSBlessingsHandles{
		store: map[int64]security.Blessings{},
	}
}

// Add adds a Blessings to the store and returns the handle to it.
func (s *JSBlessingsHandles) Add(blessings security.Blessings) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastHandle++
	handle := s.lastHandle
	s.store[handle] = blessings
	return handle
}

// Remove removes the Blessings associated with the handle.
func (s *JSBlessingsHandles) Remove(handle int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.store, handle)
}

// Get returns the Blessings represented by the handle. Returns nil
// if no Blessings exists for the handle.
func (s *JSBlessingsHandles) Get(handle int64) security.Blessings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store[handle]
}
