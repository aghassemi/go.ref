// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strconv"
	"strings"
	"time"

	netcontext "golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/cloud"
	"google.golang.org/cloud/bigtable"
	"google.golang.org/cloud/bigtable/bttest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"v.io/v23/context"
	"v.io/v23/conventions"
	"v.io/v23/security"
	"v.io/v23/security/access"
	v23mt "v.io/v23/services/mounttable"

	"v.io/x/ref/lib/timekeeper"
)

const (
	metadataFamily = "m"
	serversFamily  = "s"
	childrenFamily = "c"

	creatorColumn     = "c"
	permissionsColumn = "p"
	stickyColumn      = "s"
	versionColumn     = "v"
)

// NewBigTable returns a BigTable object that abstracts some aspects of the
// Cloud Bigtable API.
func NewBigTable(keyFile, project, zone, cluster, tableName string) (*BigTable, error) {
	ctx := netcontext.Background()
	tk, err := getTokenSource(ctx, bigtable.Scope, keyFile)
	if err != nil {
		return nil, err
	}
	client, err := bigtable.NewClient(ctx, project, zone, cluster, cloud.WithTokenSource(tk))
	if err != nil {
		return nil, err
	}

	bt := &BigTable{
		tableName: tableName,
		cache:     &rowCache{},
		createAdminClient: func() (*bigtable.AdminClient, error) {
			return bigtable.NewAdminClient(ctx, project, zone, cluster, cloud.WithTokenSource(tk))

		},
	}
	bt.nodeTbl = client.Open(bt.nodeTableName())
	bt.counterTbl = client.Open(bt.counterTableName())
	return bt, nil
}

// NewTestBigTable returns a BigTable object that is connected to an in-memory
// fake bigtable cluster.
func NewTestBigTable(tableName string) (*BigTable, func(), error) {
	srv, err := bttest.NewServer("127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	ctx := netcontext.Background()
	conn, err := grpc.Dial(srv.Addr, grpc.WithInsecure())
	if err != nil {
		return nil, nil, err
	}
	client, err := bigtable.NewClient(ctx, "", "", "", cloud.WithBaseGRPC(conn))
	if err != nil {
		return nil, nil, err
	}

	bt := &BigTable{
		tableName: tableName,
		testMode:  true,
		cache:     &rowCache{},
		createAdminClient: func() (*bigtable.AdminClient, error) {
			conn, err := grpc.Dial(srv.Addr, grpc.WithInsecure())
			if err != nil {
				return nil, err
			}
			return bigtable.NewAdminClient(ctx, "", "", "", cloud.WithBaseGRPC(conn))
		},
	}
	bt.nodeTbl = client.Open(bt.nodeTableName())
	bt.counterTbl = client.Open(bt.counterTableName())
	return bt, func() { srv.Close() }, nil
}

type BigTable struct {
	tableName         string
	testMode          bool
	nodeTbl           *bigtable.Table
	counterTbl        *bigtable.Table
	cache             *rowCache
	createAdminClient func() (*bigtable.AdminClient, error)
}

func (b *BigTable) nodeTableName() string {
	return b.tableName
}

func (b *BigTable) counterTableName() string {
	return b.tableName + "-counters"
}

// SetupTable creates the table, column families, and GC policies.
func (b *BigTable) SetupTable(ctx *context.T, permissionsFile string) error {
	bctx, cancel := btctx(ctx)
	defer cancel()

	client, err := b.createAdminClient()
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.CreateTable(bctx, b.counterTableName()); err != nil {
		return err
	}
	if err := client.CreateTable(bctx, b.nodeTableName()); err != nil {
		return err
	}

	families := []struct {
		tableName  string
		familyName string
		gcPolicy   bigtable.GCPolicy
	}{
		{b.counterTableName(), metadataFamily, bigtable.MaxVersionsPolicy(1)},
		{b.nodeTableName(), metadataFamily, bigtable.MaxVersionsPolicy(1)},
		{b.nodeTableName(), serversFamily, bigtable.MaxVersionsPolicy(1)},
		{b.nodeTableName(), childrenFamily, bigtable.MaxVersionsPolicy(1)},
	}
	for _, f := range families {
		if err := client.CreateColumnFamily(bctx, f.tableName, f.familyName); err != nil {
			return err
		}
		if err := client.SetGCPolicy(bctx, f.tableName, f.familyName, f.gcPolicy); err != nil {
			return err
		}
	}

	if permissionsFile != "" {
		return createNodesFromFile(ctx, b, permissionsFile)
	}
	perms := make(access.Permissions)
	perms.Add(security.AllPrincipals, string(v23mt.Admin))
	return b.createRow(ctx, "", perms, "", b.now())
}

func (b *BigTable) timeFloor(t bigtable.Timestamp) bigtable.Timestamp {
	// The bigtable server expects millisecond granularity, but
	// bigtable.Now() returns a timestamp with microsecond granularity.
	//
	// https://github.com/GoogleCloudPlatform/gcloud-golang/blob/master/bigtable/bttest/inmem.go#L734
	// https://github.com/GoogleCloudPlatform/gcloud-golang/blob/master/bigtable/bigtable.go#L531
	return (t / 1000) * 1000
}

func (b *BigTable) timeNext(t bigtable.Timestamp) bigtable.Timestamp {
	return (t/1000 + 1) * 1000
}

func (b *BigTable) now() bigtable.Timestamp {
	return b.timeFloor(bigtable.Now())
}

func (b *BigTable) time(t time.Time) bigtable.Timestamp {
	return b.timeFloor(bigtable.Time(t))
}

// DeleteTable deletes the table.
func (b *BigTable) DeleteTable(ctx *context.T) error {
	bctx, cancel := btctx(ctx)
	defer cancel()

	client, err := b.createAdminClient()
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.DeleteTable(bctx, b.counterTableName()); err != nil {
		return err
	}
	return client.DeleteTable(bctx, b.nodeTableName())
}

// DumpTable prints all the mounttable nodes stored in the bigtable.
func (b *BigTable) DumpTable(ctx *context.T) error {
	bctx, cancel := btctx(ctx)
	defer cancel()

	clock := timekeeper.RealTime()
	return b.nodeTbl.ReadRows(bctx, bigtable.InfiniteRange(""),
		func(row bigtable.Row) bool {
			n := nodeFromRow(ctx, b, row, clock)
			if n.name == "" {
				n.name = "(root)"
			}
			fmt.Printf("%s version: %s", n.name, n.version)
			if n.sticky {
				fmt.Printf(" sticky")
			}
			fmt.Printf(" perms: %s", n.permissions)
			if len(n.servers) > 0 {
				fmt.Printf(" servers:")
				for _, s := range n.servers {
					delta := s.Deadline.Time.Sub(clock.Now())
					fmt.Printf(" [%s %ds]", s.Server, int(delta.Seconds()))
				}
				fmt.Printf(" flags: %+v", n.mountFlags)
			}
			if len(n.children) > 0 {
				fmt.Printf(" children: [%s]", strings.Join(n.children, " "))
			}
			fmt.Println()
			return true
		},
		bigtable.RowFilter(bigtable.LatestNFilter(1)),
	)
}

func (b *BigTable) CountRows(ctx *context.T) (int, error) {
	bctx, cancel := btctx(ctx)
	defer cancel()

	count := 0
	if err := b.nodeTbl.ReadRows(bctx, bigtable.InfiniteRange(""),
		func(row bigtable.Row) bool {
			count++
			return true
		},
		bigtable.RowFilter(bigtable.LatestNFilter(1)),
	); err != nil {
		return 0, err
	}
	return count, nil
}

func (b *BigTable) Counters(ctx *context.T) (map[string]int64, error) {
	bctx, cancel := btctx(ctx)
	defer cancel()

	counters := make(map[string]int64)
	if err := b.counterTbl.ReadRows(bctx, bigtable.InfiniteRange(""),
		func(row bigtable.Row) bool {
			c, err := decodeCounterValue(ctx, row)
			if err != nil {
				ctx.Errorf("decodeCounterValue: %v", err)
				return false
			}
			counters[row.Key()] = c
			return true
		},
		bigtable.RowFilter(bigtable.LatestNFilter(1)),
	); err != nil {
		return nil, err
	}
	return counters, nil
}

func getTokenSource(ctx netcontext.Context, scope, keyFile string) (oauth2.TokenSource, error) {
	if len(keyFile) == 0 {
		return google.DefaultTokenSource(ctx, scope)
	}
	data, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	config, err := google.JWTConfigFromJSON(data, scope)
	if err != nil {
		return nil, err
	}
	return config.TokenSource(ctx), nil
}

func btctx(ctx *context.T) (netcontext.Context, func()) {
	deadline, hasDeadline := ctx.Deadline()
	now := time.Now()
	if !hasDeadline || deadline.Sub(now) < time.Minute {
		deadline = now.Add(time.Minute)
	}
	return netcontext.WithDeadline(netcontext.Background(), deadline)
}

func (b *BigTable) apply(ctx *context.T, row string, m *bigtable.Mutation, opts ...bigtable.ApplyOption) error {
	bctx, cancel := btctx(ctx)
	defer cancel()
	// The local cache entry for this row is invalidated after each
	// mutation, whether it succeeds or not.
	// If it succeeds, the row has changed and the cached data is stale.
	// If it fails, it's likely because of a concurrent mutation by another
	// server.
	// Either way, we can't used the cached version anymore.
	defer b.cache.invalidate(row)
	return b.nodeTbl.Apply(bctx, row, m, opts...)
}

func (b *BigTable) readRow(ctx *context.T, key string, opts ...bigtable.ReadOption) (bigtable.Row, error) {
	row, err := b.cache.getRefresh(key,
		func() (bigtable.Row, error) {
			bctx, cancel := btctx(ctx)
			defer cancel()
			return b.nodeTbl.ReadRow(bctx, key, opts...)
		},
	)
	if grpc.Code(err) == codes.DeadlineExceeded {
		ctx.Errorf("Received DeadlineExceeded for %s", key)
		if b.testMode {
			panic("DeadlineExceeded from testserver")
		}
	}
	return row, err
}

func (b *BigTable) createRow(ctx *context.T, name string, perms access.Permissions, creator string, ts bigtable.Timestamp) error {
	jsonPerms, err := json.Marshal(perms)
	if err != nil {
		return err
	}
	if creator == "" {
		creator = conventions.ServerUser
	}
	mut := bigtable.NewMutation()
	mut.Set(metadataFamily, creatorColumn, ts, []byte(creator))
	mut.Set(metadataFamily, permissionsColumn, bigtable.ServerTime, jsonPerms)
	mut.Set(metadataFamily, versionColumn, bigtable.ServerTime, []byte(strconv.FormatUint(uint64(rand.Uint32()), 10)))
	if err := b.apply(ctx, rowKey(name), mut); err != nil {
		return err
	}
	return incrementCreatorNodeCount(ctx, b, creator, 1)
}
