// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: ben

// Package ben defines datastructures to archive microbenchmark results.
//
// These are the data structures common to tools described in
// https://docs.google.com/document/d/1v-iKwej3eYT_RNhPwQ81A9fa8H15Q6RzNyv2rrAeAUc/edit?usp=sharing
package ben

import (
	"fmt"
	"v.io/v23/vdl"
)

var _ = __VDLInit() // Must be first; see __VDLInit comments for details.

//////////////////////////////////////////////////
// Type definitions

// Cpu describes the CPU of the machine on which the microbenchmarks were run.
type Cpu struct {
	Architecture  string // Architecture of the CPU, e.g. "amd64", "386" etc.
	Description   string // A detailed description of the CPU, e.g., "Intel(R) Core(TM) i7-5557U CPU @ 3.10GHz"
	ClockSpeedMhz uint32 // Clock speed of the CPU in MHz
}

func (Cpu) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/ben.Cpu"`
}) {
}

func (x Cpu) VDLIsZero() bool {
	return x == Cpu{}
}

func (x Cpu) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(vdl.TypeOf((*Cpu)(nil)).Elem()); err != nil {
		return err
	}
	if x.Architecture != "" {
		if err := enc.NextField("Architecture"); err != nil {
			return err
		}
		if err := enc.StartValue(vdl.StringType); err != nil {
			return err
		}
		if err := enc.EncodeString(x.Architecture); err != nil {
			return err
		}
		if err := enc.FinishValue(); err != nil {
			return err
		}
	}
	if x.Description != "" {
		if err := enc.NextField("Description"); err != nil {
			return err
		}
		if err := enc.StartValue(vdl.StringType); err != nil {
			return err
		}
		if err := enc.EncodeString(x.Description); err != nil {
			return err
		}
		if err := enc.FinishValue(); err != nil {
			return err
		}
	}
	if x.ClockSpeedMhz != 0 {
		if err := enc.NextField("ClockSpeedMhz"); err != nil {
			return err
		}
		if err := enc.StartValue(vdl.Uint32Type); err != nil {
			return err
		}
		if err := enc.EncodeUint(uint64(x.ClockSpeedMhz)); err != nil {
			return err
		}
		if err := enc.FinishValue(); err != nil {
			return err
		}
	}
	if err := enc.NextField(""); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *Cpu) VDLRead(dec vdl.Decoder) error {
	*x = Cpu{}
	if err := dec.StartValue(); err != nil {
		return err
	}
	if (dec.StackDepth() == 1 || dec.IsAny()) && !vdl.Compatible(vdl.TypeOf(*x), dec.Type()) {
		return fmt.Errorf("incompatible struct %T, from %v", *x, dec.Type())
	}
	for {
		f, err := dec.NextField()
		if err != nil {
			return err
		}
		switch f {
		case "":
			return dec.FinishValue()
		case "Architecture":
			if err := dec.StartValue(); err != nil {
				return err
			}
			var err error
			if x.Architecture, err = dec.DecodeString(); err != nil {
				return err
			}
			if err := dec.FinishValue(); err != nil {
				return err
			}
		case "Description":
			if err := dec.StartValue(); err != nil {
				return err
			}
			var err error
			if x.Description, err = dec.DecodeString(); err != nil {
				return err
			}
			if err := dec.FinishValue(); err != nil {
				return err
			}
		case "ClockSpeedMhz":
			if err := dec.StartValue(); err != nil {
				return err
			}
			tmp, err := dec.DecodeUint(32)
			if err != nil {
				return err
			}
			x.ClockSpeedMhz = uint32(tmp)
			if err := dec.FinishValue(); err != nil {
				return err
			}
		default:
			if err := dec.SkipValue(); err != nil {
				return err
			}
		}
	}
}

// Os describes the Operating System on which the microbenchmarks were run.
type Os struct {
	Name    string // Short name of the operating system: linux, darwin, android etc.
	Version string // Details of the distribution/version, e.g., "Ubuntu 14.04", "Mac OS X 10.11.2 15C50" etc.
}

func (Os) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/ben.Os"`
}) {
}

func (x Os) VDLIsZero() bool {
	return x == Os{}
}

func (x Os) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(vdl.TypeOf((*Os)(nil)).Elem()); err != nil {
		return err
	}
	if x.Name != "" {
		if err := enc.NextField("Name"); err != nil {
			return err
		}
		if err := enc.StartValue(vdl.StringType); err != nil {
			return err
		}
		if err := enc.EncodeString(x.Name); err != nil {
			return err
		}
		if err := enc.FinishValue(); err != nil {
			return err
		}
	}
	if x.Version != "" {
		if err := enc.NextField("Version"); err != nil {
			return err
		}
		if err := enc.StartValue(vdl.StringType); err != nil {
			return err
		}
		if err := enc.EncodeString(x.Version); err != nil {
			return err
		}
		if err := enc.FinishValue(); err != nil {
			return err
		}
	}
	if err := enc.NextField(""); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *Os) VDLRead(dec vdl.Decoder) error {
	*x = Os{}
	if err := dec.StartValue(); err != nil {
		return err
	}
	if (dec.StackDepth() == 1 || dec.IsAny()) && !vdl.Compatible(vdl.TypeOf(*x), dec.Type()) {
		return fmt.Errorf("incompatible struct %T, from %v", *x, dec.Type())
	}
	for {
		f, err := dec.NextField()
		if err != nil {
			return err
		}
		switch f {
		case "":
			return dec.FinishValue()
		case "Name":
			if err := dec.StartValue(); err != nil {
				return err
			}
			var err error
			if x.Name, err = dec.DecodeString(); err != nil {
				return err
			}
			if err := dec.FinishValue(); err != nil {
				return err
			}
		case "Version":
			if err := dec.StartValue(); err != nil {
				return err
			}
			var err error
			if x.Version, err = dec.DecodeString(); err != nil {
				return err
			}
			if err := dec.FinishValue(); err != nil {
				return err
			}
		default:
			if err := dec.SkipValue(); err != nil {
				return err
			}
		}
	}
}

// Scenario encapsulates the conditions on the machine on which the microbenchmarks were run.
type Scenario struct {
	Cpu   Cpu
	Os    Os
	Label string // Arbitrary string label assigned by the uploader.
}

func (Scenario) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/ben.Scenario"`
}) {
}

func (x Scenario) VDLIsZero() bool {
	return x == Scenario{}
}

func (x Scenario) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(vdl.TypeOf((*Scenario)(nil)).Elem()); err != nil {
		return err
	}
	if x.Cpu != (Cpu{}) {
		if err := enc.NextField("Cpu"); err != nil {
			return err
		}
		if err := x.Cpu.VDLWrite(enc); err != nil {
			return err
		}
	}
	if x.Os != (Os{}) {
		if err := enc.NextField("Os"); err != nil {
			return err
		}
		if err := x.Os.VDLWrite(enc); err != nil {
			return err
		}
	}
	if x.Label != "" {
		if err := enc.NextField("Label"); err != nil {
			return err
		}
		if err := enc.StartValue(vdl.StringType); err != nil {
			return err
		}
		if err := enc.EncodeString(x.Label); err != nil {
			return err
		}
		if err := enc.FinishValue(); err != nil {
			return err
		}
	}
	if err := enc.NextField(""); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *Scenario) VDLRead(dec vdl.Decoder) error {
	*x = Scenario{}
	if err := dec.StartValue(); err != nil {
		return err
	}
	if (dec.StackDepth() == 1 || dec.IsAny()) && !vdl.Compatible(vdl.TypeOf(*x), dec.Type()) {
		return fmt.Errorf("incompatible struct %T, from %v", *x, dec.Type())
	}
	for {
		f, err := dec.NextField()
		if err != nil {
			return err
		}
		switch f {
		case "":
			return dec.FinishValue()
		case "Cpu":
			if err := x.Cpu.VDLRead(dec); err != nil {
				return err
			}
		case "Os":
			if err := x.Os.VDLRead(dec); err != nil {
				return err
			}
		case "Label":
			if err := dec.StartValue(); err != nil {
				return err
			}
			var err error
			if x.Label, err = dec.DecodeString(); err != nil {
				return err
			}
			if err := dec.FinishValue(); err != nil {
				return err
			}
		default:
			if err := dec.SkipValue(); err != nil {
				return err
			}
		}
	}
}

// SourceCode represents the state of the source code used to build the
// microbenchmarks.
//
// Typically it would be the commit hash of a git repository or the contents of
// a manifest of a jiri (https://github.com/vanadium/go.jiri) project and not
// the complete source code itself.
type SourceCode string

func (SourceCode) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/ben.SourceCode"`
}) {
}

func (x SourceCode) VDLIsZero() bool {
	return x == ""
}

func (x SourceCode) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(vdl.TypeOf((*SourceCode)(nil))); err != nil {
		return err
	}
	if err := enc.EncodeString(string(x)); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *SourceCode) VDLRead(dec vdl.Decoder) error {
	if err := dec.StartValue(); err != nil {
		return err
	}
	tmp, err := dec.DecodeString()
	if err != nil {
		return err
	}
	*x = SourceCode(tmp)
	return dec.FinishValue()
}

// Run encapsulates the results of a single microbenchmark run.
type Run struct {
	Name              string // Name of the microbenchmark. <package>.Benchmark<Name> in Go.
	Iterations        uint64
	NanoSecsPerOp     float64 // Nano-seconds per iteration.
	AllocsPerOp       uint64  // Memory allocations per iteration.
	AllocedBytesPerOp uint64  // Size of memory allocations per iteration.
	MegaBytesPerSec   float64 // Throughput in MB/s.
	Parallelism       uint32  // For Go, the GOMAXPROCS used during benchmark execution
}

func (Run) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/ben.Run"`
}) {
}

func (x Run) VDLIsZero() bool {
	return x == Run{}
}

func (x Run) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(vdl.TypeOf((*Run)(nil)).Elem()); err != nil {
		return err
	}
	if x.Name != "" {
		if err := enc.NextField("Name"); err != nil {
			return err
		}
		if err := enc.StartValue(vdl.StringType); err != nil {
			return err
		}
		if err := enc.EncodeString(x.Name); err != nil {
			return err
		}
		if err := enc.FinishValue(); err != nil {
			return err
		}
	}
	if x.Iterations != 0 {
		if err := enc.NextField("Iterations"); err != nil {
			return err
		}
		if err := enc.StartValue(vdl.Uint64Type); err != nil {
			return err
		}
		if err := enc.EncodeUint(x.Iterations); err != nil {
			return err
		}
		if err := enc.FinishValue(); err != nil {
			return err
		}
	}
	if x.NanoSecsPerOp != 0 {
		if err := enc.NextField("NanoSecsPerOp"); err != nil {
			return err
		}
		if err := enc.StartValue(vdl.Float64Type); err != nil {
			return err
		}
		if err := enc.EncodeFloat(x.NanoSecsPerOp); err != nil {
			return err
		}
		if err := enc.FinishValue(); err != nil {
			return err
		}
	}
	if x.AllocsPerOp != 0 {
		if err := enc.NextField("AllocsPerOp"); err != nil {
			return err
		}
		if err := enc.StartValue(vdl.Uint64Type); err != nil {
			return err
		}
		if err := enc.EncodeUint(x.AllocsPerOp); err != nil {
			return err
		}
		if err := enc.FinishValue(); err != nil {
			return err
		}
	}
	if x.AllocedBytesPerOp != 0 {
		if err := enc.NextField("AllocedBytesPerOp"); err != nil {
			return err
		}
		if err := enc.StartValue(vdl.Uint64Type); err != nil {
			return err
		}
		if err := enc.EncodeUint(x.AllocedBytesPerOp); err != nil {
			return err
		}
		if err := enc.FinishValue(); err != nil {
			return err
		}
	}
	if x.MegaBytesPerSec != 0 {
		if err := enc.NextField("MegaBytesPerSec"); err != nil {
			return err
		}
		if err := enc.StartValue(vdl.Float64Type); err != nil {
			return err
		}
		if err := enc.EncodeFloat(x.MegaBytesPerSec); err != nil {
			return err
		}
		if err := enc.FinishValue(); err != nil {
			return err
		}
	}
	if x.Parallelism != 0 {
		if err := enc.NextField("Parallelism"); err != nil {
			return err
		}
		if err := enc.StartValue(vdl.Uint32Type); err != nil {
			return err
		}
		if err := enc.EncodeUint(uint64(x.Parallelism)); err != nil {
			return err
		}
		if err := enc.FinishValue(); err != nil {
			return err
		}
	}
	if err := enc.NextField(""); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *Run) VDLRead(dec vdl.Decoder) error {
	*x = Run{}
	if err := dec.StartValue(); err != nil {
		return err
	}
	if (dec.StackDepth() == 1 || dec.IsAny()) && !vdl.Compatible(vdl.TypeOf(*x), dec.Type()) {
		return fmt.Errorf("incompatible struct %T, from %v", *x, dec.Type())
	}
	for {
		f, err := dec.NextField()
		if err != nil {
			return err
		}
		switch f {
		case "":
			return dec.FinishValue()
		case "Name":
			if err := dec.StartValue(); err != nil {
				return err
			}
			var err error
			if x.Name, err = dec.DecodeString(); err != nil {
				return err
			}
			if err := dec.FinishValue(); err != nil {
				return err
			}
		case "Iterations":
			if err := dec.StartValue(); err != nil {
				return err
			}
			var err error
			if x.Iterations, err = dec.DecodeUint(64); err != nil {
				return err
			}
			if err := dec.FinishValue(); err != nil {
				return err
			}
		case "NanoSecsPerOp":
			if err := dec.StartValue(); err != nil {
				return err
			}
			var err error
			if x.NanoSecsPerOp, err = dec.DecodeFloat(64); err != nil {
				return err
			}
			if err := dec.FinishValue(); err != nil {
				return err
			}
		case "AllocsPerOp":
			if err := dec.StartValue(); err != nil {
				return err
			}
			var err error
			if x.AllocsPerOp, err = dec.DecodeUint(64); err != nil {
				return err
			}
			if err := dec.FinishValue(); err != nil {
				return err
			}
		case "AllocedBytesPerOp":
			if err := dec.StartValue(); err != nil {
				return err
			}
			var err error
			if x.AllocedBytesPerOp, err = dec.DecodeUint(64); err != nil {
				return err
			}
			if err := dec.FinishValue(); err != nil {
				return err
			}
		case "MegaBytesPerSec":
			if err := dec.StartValue(); err != nil {
				return err
			}
			var err error
			if x.MegaBytesPerSec, err = dec.DecodeFloat(64); err != nil {
				return err
			}
			if err := dec.FinishValue(); err != nil {
				return err
			}
		case "Parallelism":
			if err := dec.StartValue(); err != nil {
				return err
			}
			tmp, err := dec.DecodeUint(32)
			if err != nil {
				return err
			}
			x.Parallelism = uint32(tmp)
			if err := dec.FinishValue(); err != nil {
				return err
			}
		default:
			if err := dec.SkipValue(); err != nil {
				return err
			}
		}
	}
}

var __VDLInitCalled bool

// __VDLInit performs vdl initialization.  It is safe to call multiple times.
// If you have an init ordering issue, just insert the following line verbatim
// into your source files in this package, right after the "package foo" clause:
//
//    var _ = __VDLInit()
//
// The purpose of this function is to ensure that vdl initialization occurs in
// the right order, and very early in the init sequence.  In particular, vdl
// registration and package variable initialization needs to occur before
// functions like vdl.TypeOf will work properly.
//
// This function returns a dummy value, so that it can be used to initialize the
// first var in the file, to take advantage of Go's defined init order.
func __VDLInit() struct{} {
	if __VDLInitCalled {
		return struct{}{}
	}
	__VDLInitCalled = true

	// Register types.
	vdl.Register((*Cpu)(nil))
	vdl.Register((*Os)(nil))
	vdl.Register((*Scenario)(nil))
	vdl.Register((*SourceCode)(nil))
	vdl.Register((*Run)(nil))

	return struct{}{}
}
