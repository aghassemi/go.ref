// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	"fmt"
	"sort"
	"strings"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/discovery"
	"v.io/v23/security"
	wire "v.io/v23/services/syncbase"
	"v.io/x/lib/nsync"
	"v.io/x/ref/services/syncbase/server/interfaces"
)

const visibilityKey = "vis"

// Discovery implements v.io/v23/discovery.T for syncbase based
// applications.
// TODO(mattr): Actually this is not syncbase specific.  At some
// point we should just replace the result of v23.NewDiscovery
// with this.
type Discovery struct {
	nhDiscovery discovery.T
	// TODO(mattr): Add global discovery.
}

// NewDiscovery creates a new syncbase discovery object.
func NewDiscovery(ctx *context.T) (discovery.T, error) {
	nhDiscovery, err := v23.NewDiscovery(ctx)
	if err != nil {
		return nil, err
	}
	return &Discovery{nhDiscovery: nhDiscovery}, nil
}

// Scan implements v.io/v23/discovery/T.Scan.
func (d *Discovery) Scan(ctx *context.T, query string) (<-chan discovery.Update, error) {
	nhUpdates, err := d.nhDiscovery.Scan(ctx, query)
	if err != nil {
		return nil, err
	}

	// Currently setting visibility on the neighborhood discovery
	// service turns IBE encryption on.  We currently don't have the
	// infrastructure support for IBE, so that would make our advertisements
	// unreadable by everyone.
	// Instead we add the visibility list to the attributes of the advertisement
	// and filter on the client side.  This is a temporary measure until
	// IBE is set up.  See v.io/i/1345.
	updates := make(chan discovery.Update)
	go func() {
		defer close(updates)
		for {
			var u discovery.Update
			select {
			case <-ctx.Done():
				return
			case u = <-nhUpdates:
			}
			if u == nil {
				continue
			}
			patterns := splitPatterns(u.Attribute(visibilityKey))
			if len(patterns) > 0 && !matchesPatterns(ctx, patterns) {
				continue
			}
			updates <- update{u}
		}
	}()

	return updates, nil
}

// Advertise implements v.io/v23/discovery/T.Advertise.
func (d *Discovery) Advertise(ctx *context.T, ad *discovery.Advertisement, visibility []security.BlessingPattern) (<-chan struct{}, error) {
	// Currently setting visibility on the neighborhood discovery
	// service turns IBE encryption on.  We currently don't have the
	// infrastructure support for IBE, so that would make our advertisements
	// unreadable by everyone.
	// Instead we add the visibility list to the attributes of the advertisement
	// and filter on the client side.  This is a temporary measure until
	// IBE is set up.  See v.io/i/1345.
	adCopy := *ad
	if len(visibility) > 0 {
		adCopy.Attributes = make(discovery.Attributes, len(ad.Attributes)+1)
		for k, v := range ad.Attributes {
			adCopy.Attributes[k] = v
		}
		patterns := joinPatterns(visibility)
		adCopy.Attributes[visibilityKey] = patterns
	}
	ch, err := d.nhDiscovery.Advertise(ctx, &adCopy, nil)
	ad.Id = adCopy.Id
	return ch, err
}

func matchesPatterns(ctx *context.T, patterns []security.BlessingPattern) bool {
	p := v23.GetPrincipal(ctx)
	blessings := p.BlessingStore().PeerBlessings()
	for _, b := range blessings {
		names := security.BlessingNames(p, b)
		for _, pattern := range patterns {
			if pattern.MatchedBy(names...) {
				return true
			}
		}
	}
	return false
}

// update wraps the discovery.Update to remove the visibility attribute which we add.
type update struct {
	discovery.Update
}

func (u update) Attribute(name string) string {
	if name == visibilityKey {
		return ""
	}
	return u.Update.Attribute(name)
}

func (u update) Advertisement() discovery.Advertisement {
	cp := u.Update.Advertisement()
	orig := cp.Attributes
	cp.Attributes = make(discovery.Attributes, len(orig))
	for k, v := range orig {
		if k != visibilityKey {
			cp.Attributes[k] = v
		}
	}
	return cp
}

// blessingSeparator is used to join multiple blessings into a
// single string.
// Note that comma cannot appear in blessings, see:
// v.io/v23/security/certificate.go
const blessingsSeparator = ','

// joinPatterns concatenates the elements of a to create a single string.
// The string can be split again with SplitPatterns.
func joinPatterns(a []security.BlessingPattern) string {
	if len(a) == 0 {
		return ""
	}
	if len(a) == 1 {
		return string(a[0])
	}
	n := (len(a) - 1)
	for i := 0; i < len(a); i++ {
		n += len(a[i])
	}

	b := make([]byte, n)
	bp := copy(b, a[0])
	for _, s := range a[1:] {
		b[bp] = blessingsSeparator
		bp++
		bp += copy(b[bp:], s)
	}
	return string(b)
}

// splitPatterns splits BlessingPatterns that were joined with
// JoinBlessingPattern.
func splitPatterns(patterns string) []security.BlessingPattern {
	if patterns == "" {
		return nil
	}
	n := strings.Count(patterns, string(blessingsSeparator)) + 1
	out := make([]security.BlessingPattern, n)
	last, start := 0, 0
	for i, r := range patterns {
		if r == blessingsSeparator {
			out[last] = security.BlessingPattern(patterns[start:i])
			last++
			start = i + 1
		}
	}
	out[last] = security.BlessingPattern(patterns[start:])
	return out
}

var state struct {
	mu    nsync.Mu
	scans map[security.Principal]*scanState
}

type scanState struct {
	dbs       map[wire.Id]*inviteQueue
	listeners int
	cancel    context.CancelFunc
}

func newScan(ctx *context.T) (*scanState, error) {
	ctx, cancel := context.WithRootCancel(ctx)
	scan := &scanState{
		dbs:    make(map[wire.Id]*inviteQueue),
		cancel: cancel,
	}
	d, err := NewDiscovery(ctx)
	if err != nil {
		scan.cancel()
		return nil, err
	}
	query := fmt.Sprintf("v.InterfaceName=\"%s/%s\"",
		interfaces.SyncDesc.PkgPath, interfaces.SyncDesc.Name)
	updates, err := d.Scan(ctx, query)
	if err != nil {
		scan.cancel()
		return nil, err
	}
	go func() {
		for u := range updates {
			if invite, db, ok := makeInvite(u.Advertisement()); ok {
				state.mu.Lock()
				q := scan.dbs[db]
				if u.IsLost() {
					// TODO(mattr): Removing like this can result in resurfacing already
					// retrieved invites.  For example if the ACL of a syncgroup is
					// changed to add a new memeber, then we might see a remove and then
					// later an add.  In this case we would end up moving the invite
					// to the end of the queue.  One way to fix this would be to
					// keep removed invites for some time.
					if q != nil {
						q.remove(invite)
						if q.empty() {
							delete(scan.dbs, db)
						}
					}
				} else {
					if q == nil {
						q = newInviteQueue()
						scan.dbs[db] = q
					}
					q.add(invite)
					q.cond.Broadcast()
				}
				state.mu.Unlock()
			}
		}
	}()
	return scan, nil
}

// Invite represents an invitation to join a syncgroup as found via Discovery.
type Invite struct {
	Syncgroup wire.Id  //Syncgroup is the Id of the syncgroup you've been invited to.
	Addresses []string //Addresses are the list of addresses of the inviting server.
	key       string
}

// ListenForInvites listens via Discovery for syncgroup invitations for the given
// database and sends the invites to the provided channel.  We stop listening when
// the given context is canceled.  When that happens we close the given channel.
func ListenForInvites(ctx *context.T, db wire.Id, ch chan<- Invite) error {
	defer state.mu.Unlock()
	state.mu.Lock()

	p := v23.GetPrincipal(ctx)
	scan := state.scans[p]
	if scan == nil {
		var err error
		if scan, err = newScan(ctx); err != nil {
			return err
		}
		if state.scans == nil {
			state.scans = make(map[security.Principal]*scanState)
		}
		state.scans[p] = scan
	}
	scan.listeners++

	q := scan.dbs[db]
	if q == nil {
		q = newInviteQueue()
		scan.dbs[db] = q
	}

	go func() {
		c := q.scan()
		for {
			next, ok := q.next(ctx, c)
			if !ok {
				break
			}
			select {
			case ch <- next.copy():
			case <-ctx.Done():
				break
			}
		}
		close(ch)

		defer state.mu.Unlock()
		state.mu.Lock()

		if q.empty() {
			delete(scan.dbs, db)
		}
		scan.listeners--
		if scan.listeners == 0 {
			scan.cancel()
			delete(state.scans, v23.GetPrincipal(ctx))
		}
	}()

	return nil
}

// inviteQueue is a linked list based queue.  As we get new invitations we
// add them to the queue, and when advertisements are lost we remove elements.
// Each call to scan creates a cursor on the queue that will iterate until
// the Listen is canceled.  We wait when we hit the end of the queue via the
// condition variable cond.
type inviteQueue struct {
	mu   nsync.Mu
	cond nsync.CV

	elems        map[string]*ielement
	sentinel     ielement
	cursors      map[int]*ielement
	nextCursorId int
}

type ielement struct {
	invite     Invite
	prev, next *ielement
}

func newInviteQueue() *inviteQueue {
	iq := &inviteQueue{
		elems:   make(map[string]*ielement),
		cursors: make(map[int]*ielement),
	}
	iq.sentinel.next, iq.sentinel.prev = &iq.sentinel, &iq.sentinel
	return iq
}

func (q *inviteQueue) empty() bool {
	defer q.mu.Unlock()
	q.mu.Lock()
	return len(q.elems) == 0 && len(q.cursors) == 0
}

func (q *inviteQueue) size() int {
	defer q.mu.Unlock()
	q.mu.Lock()
	return len(q.elems)
}

func (q *inviteQueue) remove(i Invite) {
	defer q.mu.Unlock()
	q.mu.Lock()

	el, ok := q.elems[i.key]
	if !ok {
		return
	}
	delete(q.elems, i.key)
	el.next.prev, el.prev.next = el.prev, el.next
	for id, c := range q.cursors {
		if c == el {
			q.cursors[id] = c.prev
		}
	}
}

func (q *inviteQueue) add(i Invite) {
	defer q.mu.Unlock()
	q.mu.Lock()

	if _, ok := q.elems[i.key]; ok {
		return
	}
	el := &ielement{i, q.sentinel.prev, &q.sentinel}
	q.sentinel.prev, q.sentinel.prev.next = el, el
	q.elems[i.key] = el
	q.cond.Broadcast()
}

func (q *inviteQueue) next(ctx *context.T, cursor int) (Invite, bool) {
	defer q.mu.Unlock()
	q.mu.Lock()
	c, exists := q.cursors[cursor]
	if !exists {
		return Invite{}, false
	}
	for c.next == &q.sentinel {
		if q.cond.WaitWithDeadline(&q.mu, nsync.NoDeadline, ctx.Done()) != nsync.OK {
			delete(q.cursors, cursor)
			return Invite{}, false
		}
	}
	c = c.next
	q.cursors[cursor] = c
	return c.invite, true
}

func (q *inviteQueue) scan() int {
	defer q.mu.Unlock()
	q.mu.Lock()
	id := q.nextCursorId
	q.nextCursorId++
	q.cursors[id] = &q.sentinel
	return id
}

func makeInvite(ad discovery.Advertisement) (Invite, wire.Id, bool) {
	var dbId, sgId wire.Id
	var ok bool
	if dbId.Name, ok = ad.Attributes[wire.DiscoveryAttrDatabaseName]; !ok {
		return Invite{}, dbId, false
	}
	if dbId.Blessing, ok = ad.Attributes[wire.DiscoveryAttrDatabaseBlessing]; !ok {
		return Invite{}, dbId, false
	}
	if sgId.Name, ok = ad.Attributes[wire.DiscoveryAttrSyncgroupName]; !ok {
		return Invite{}, dbId, false
	}
	if sgId.Blessing, ok = ad.Attributes[wire.DiscoveryAttrSyncgroupBlessing]; !ok {
		return Invite{}, dbId, false
	}
	i := Invite{
		Addresses: append([]string{}, ad.Addresses...),
		Syncgroup: sgId,
	}
	sort.Strings(i.Addresses)
	i.key = fmt.Sprintf("%v", i)
	return i, dbId, true
}

func (i Invite) copy() Invite {
	cp := i
	cp.Addresses = append([]string{}, i.Addresses...)
	return cp
}
