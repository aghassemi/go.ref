// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package discovery

import (
	"github.com/pborman/uuid"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/discovery"
)

// Scan implements discovery.Scanner.
func (ds *ds) Scan(ctx *context.T, query string) (<-chan discovery.Update, error) {
	// TODO(jhann): Implement a simple query processor.
	var serviceUuid uuid.UUID
	if len(query) > 0 {
		serviceUuid = NewServiceUUID(query)
	}
	// TODO(jhahn): Revisit the buffer size.
	scanCh := make(chan Advertisement, 10)
	ctx, cancel := context.WithCancel(ctx)
	for _, plugin := range ds.plugins {
		err := plugin.Scan(ctx, serviceUuid, scanCh)
		if err != nil {
			cancel()
			return nil, err
		}
	}
	// TODO(jhahn): Revisit the buffer size.
	updateCh := make(chan discovery.Update, 10)
	go doScan(ctx, scanCh, updateCh)
	return updateCh, nil
}

func doScan(ctx *context.T, scanCh <-chan Advertisement, updateCh chan<- discovery.Update) {
	defer close(updateCh)

	// Get the blessing names belong to the principal.
	//
	// TODO(jhahn): It isn't clear that we will always have the blessing required to decrypt
	// the advertisement as their "default" blessing - indeed it may not even be in the store.
	// Revisit this issue.
	principal := v23.GetPrincipal(ctx)
	var names []string
	if principal != nil {
		blessings := principal.BlessingStore().Default()
		for n, _ := range principal.BlessingsInfo(blessings) {
			names = append(names, n)
		}
	}

	// A plugin may returns a Lost event with clearing all attributes including encryption
	// keys. Thus, we have to keep what we've found so far so that we can ignore the Lost
	// events for instances that we ignored due to permission.
	found := make(map[string]struct{})
	for {
		select {
		case ad := <-scanCh:
			if err := decrypt(&ad, names); err != nil {
				// Couldn't decrypt it. Ignore it.
				if err != errNoPermission {
					ctx.Error(err)
				}
				continue
			}
			id := string(ad.InstanceUuid)
			// TODO(jhahn): Merge scanData based on InstanceUuid.
			if ad.Lost {
				if _, ok := found[id]; ok {
					delete(found, id)
					updateCh <- discovery.UpdateLost{discovery.Lost{InstanceUuid: ad.InstanceUuid}}
				}
			} else {
				found[id] = struct{}{}
				updateCh <- discovery.UpdateFound{discovery.Found{Service: ad.Service}}
			}
		case <-ctx.Done():
			return
		}
	}
}
