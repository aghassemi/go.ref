// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package commands

import (
	"fmt"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/syncbase"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/cmd/sb/dbutil"
	"v.io/x/ref/lib/v23cmd"
)

var Commands = []*cmdline.Command{
	cmdDump,
	cmdMakeDemo,
	cmdSelect,
}

var (
	commandCtx *context.T
	commandDb  syncbase.Database
)

func SetCtx(ctx *context.T) {
	commandCtx = ctx
}

func SetDB(db syncbase.Database) {
	commandDb = db
}

type sbHandler func(ctx *context.T, db syncbase.Database, env *cmdline.Env, args []string) error

func SbRunner(handler sbHandler) cmdline.Runner {
	return v23cmd.RunnerFuncWithInit(func(ctx *context.T, env *cmdline.Env, args []string) error {
		db := commandDb // Set in shell handler.
		if db == nil {
			var err error
			if db, err = dbutil.OpenDB(ctx); err != nil {
				return err
			}
		}
		return handler(ctx, db, env, args)
	}, func() (*context.T, v23.Shutdown, error) {
		if commandCtx != nil {
			return commandCtx, func() {}, nil
		}
		return v23.TryInit()
	})
}

func GetCommand(name string) (*cmdline.Command, error) {
	for _, cmd := range Commands {
		if cmd.Name == name {
			return cmd, nil
		}
	}

	return nil, fmt.Errorf("no command %q", name)
}

func PrintUsage(command *cmdline.Command) {
	fmt.Println(command.Long)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Printf("\t%s [flags] %s\n", command.Name, command.ArgsName)
	fmt.Println()
	fmt.Println(command.ArgsLong)
}
