// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $V23_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"fmt"
	"math/rand"
	"time"

	"v.io/x/lib/cmdline"

	"v.io/v23"
	"v.io/v23/context"

	"v.io/x/ref/examples/rps"
	"v.io/x/ref/examples/rps/internal"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"

	_ "v.io/x/ref/runtime/factories/roaming"
)

var (
	name, aclFile string
	numGames      int
)

func main() {
	cmdRoot.Flags.StringVar(&name, "name", "", "Identifier to publish as (defaults to user@hostname).")
	cmdRoot.Flags.StringVar(&aclFile, "acl-file", "", "File containing JSON-encoded Permissions.")
	cmdRoot.Flags.IntVar(&numGames, "num-games", -1, "Number of games to play (-1 means unlimited).")
	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdRoot)
}

var cmdRoot = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runBot),
	Name:   "rpsbot",
	Short:  "repeatedly runs automated games",
	Long: `
Command rpsbot repeatedly runs automated games, implementing all three roles.
It publishes itself as player, judge, and scorekeeper. Then, it initiates games
with other players, in a loop. As soon as one game is over, it starts a new one.
`,
}

func runBot(ctx *context.T, env *cmdline.Env, args []string) error {
	auth := internal.NewAuthorizer(aclFile)
	server, err := v23.NewServer(ctx)
	if err != nil {
		return fmt.Errorf("NewServer failed: %v", err)
	}

	rand.Seed(time.Now().UnixNano())
	rpsService := NewRPS(ctx)

	listenSpec := v23.GetListenSpec(ctx)
	eps, err := server.Listen(listenSpec)
	if err != nil {
		return fmt.Errorf("Listen(%v) failed: %v", listenSpec, err)
	}
	if name == "" {
		name = internal.CreateName()
	}
	names := []string{
		fmt.Sprintf("rps/judge/%s", name),
		fmt.Sprintf("rps/player/%s", name),
		fmt.Sprintf("rps/scorekeeper/%s", name),
	}
	if err := server.Serve(names[0], rps.RockPaperScissorsServer(rpsService), auth); err != nil {
		return fmt.Errorf("Serve(%v) failed: %v", names[0], err)
	}
	for _, n := range names[1:] {
		if err := server.AddName(n); err != nil {
			return fmt.Errorf("(%v) failed: %v", n, err)
		}
	}
	ctx.Infof("Listening on endpoint %s (published as %v)", eps, names)

	go initiateGames(ctx, rpsService)
	<-signals.ShutdownOnSignals(ctx)
	return nil
}

func initiateGames(ctx *context.T, rpsService *RPS) {
	for i := 0; i < numGames || numGames == -1; i++ {
		if err := rpsService.Player().InitiateGame(ctx); err != nil {
			ctx.Infof("Failed to initiate game: %v", err)
		}
		time.Sleep(5 * time.Second)
	}
}
