// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ble

import (
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/discovery"

	idiscovery "v.io/x/ref/lib/discovery"
)

type scanner struct {
	ctx    *context.T
	driver Driver

	rescan chan struct{}
	done   chan struct{}

	mu          sync.Mutex
	listeners   map[string][]chan<- *idiscovery.AdInfo // GUARDED_BY(mu)
	scanRecords map[discovery.AdId]*scanRecord         // GUARDED_BY(mu)

	// TTL for scanned advertisement. If we do not see the advertisement again
	// during that period, we send a "Lost" notification.
	ttl time.Duration
}

type scanRecord struct {
	interfaceName string
	expiry        time.Time
}

func (s *scanner) addListener(interfaceName string) chan *idiscovery.AdInfo {
	// TODO(jhahn): Revisit the buffer size. A channel with one buffer should be
	// enough to avoid deadlock, but more buffers are added for smooth operation.
	ch := make(chan *idiscovery.AdInfo, 10)
	s.mu.Lock()
	listeners := append(s.listeners[interfaceName], ch)
	s.listeners[interfaceName] = listeners
	s.mu.Unlock()
	select {
	case s.rescan <- struct{}{}:
	default:
	}
	return ch
}

func (s *scanner) removeListener(interfaceName string, ch chan *idiscovery.AdInfo) {
	go func() {
		for range ch {
		}
	}()

	s.mu.Lock()
	listeners := s.listeners[interfaceName]
	for i, listener := range listeners {
		if listener == ch {
			n := len(listeners) - 1
			listeners[i], listeners[n] = listeners[n], nil
			listeners = listeners[:n]
			break
		}
	}
	if len(listeners) > 0 {
		s.listeners[interfaceName] = listeners
	} else {
		delete(s.listeners, interfaceName)
	}
	s.mu.Unlock()
	select {
	case s.rescan <- struct{}{}:
	default:
	}
	close(ch)
}

func (s *scanner) shutdown() {
	close(s.done)
}

func (s *scanner) scanLoop() {
	refreshInterval := s.ttl / 2

	isScanning := false
	stopScan := func() {
		if isScanning {
			s.driver.StopScan()
		}
		isScanning = false
	}
	defer stopScan()

	var refresh <-chan time.Time
	for {
		select {
		case <-refresh:
		case <-s.rescan:
		case <-s.done:
			return
		}

		stopScan()

		s.mu.Lock()
		if len(s.listeners) == 0 {
			s.scanRecords = make(map[discovery.AdId]*scanRecord)
			s.mu.Unlock()
			refresh = nil
			continue
		}

		uuids := make([]string, 0, len(s.listeners)*2)
		if _, ok := s.listeners[""]; !ok {
			for interfaceName, _ := range s.listeners {
				uuid := newServiceUuid(interfaceName)
				uuids = append(uuids, uuid.String())
				toggleServiceUuid(uuid)
				uuids = append(uuids, uuid.String())
			}
		}
		s.mu.Unlock()

		if err := s.driver.StartScan(uuids, vanadiumUuidBase, vanadiumUuidMask, s); err != nil {
			s.ctx.Error(err)
		} else {
			isScanning = true
		}

		go s.gc()
		refresh = time.After(refreshInterval)
	}
}

func (s *scanner) OnDiscovered(uuid string, characteristics map[string][]byte, rssi int) {
	// TODO(jhahn): Add rssi to adinfo.
	adinfo, err := decodeAdInfo(characteristics)
	if err != nil {
		s.ctx.Error(err)
		return
	}

	s.mu.Lock()
	s.scanRecords[adinfo.Ad.Id] = &scanRecord{adinfo.Ad.InterfaceName, time.Now().Add(s.ttl)}
	for _, ch := range append(s.listeners[adinfo.Ad.InterfaceName], s.listeners[""]...) {
		ch <- adinfo
	}
	s.mu.Unlock()
}

func (s *scanner) gc() {
	s.mu.Lock()
	now := time.Now()
	for id, rec := range s.scanRecords {
		if rec.expiry.After(now) {
			continue
		}

		delete(s.scanRecords, id)
		adinfo := &idiscovery.AdInfo{Ad: discovery.Advertisement{Id: id}, Lost: true}
		for _, ch := range append(s.listeners[rec.interfaceName], s.listeners[""]...) {
			ch <- adinfo
		}
	}
	s.mu.Unlock()
}

func newScanner(ctx *context.T, driver Driver, ttl time.Duration) *scanner {
	s := &scanner{
		ctx:         ctx,
		driver:      driver,
		rescan:      make(chan struct{}, 1),
		done:        make(chan struct{}),
		listeners:   make(map[string][]chan<- *idiscovery.AdInfo),
		scanRecords: make(map[discovery.AdId]*scanRecord),
		ttl:         ttl,
	}
	go s.scanLoop()
	return s
}
