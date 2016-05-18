// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: nativetest

// Package nativetest tests a package with native type conversions.
package nativetest

import (
	"fmt"
	"time"
	"v.io/v23/vdl"
	"v.io/v23/vdl/testdata/nativetest"
)

var _ = __VDLInit() // Must be first; see __VDLInit comments for details.

//////////////////////////////////////////////////
// Type definitions

type WireString int32

func (WireString) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/vdl/testdata/nativetest.WireString"`
}) {
}

func (x WireString) VDLIsZero() bool {
	return x == 0
}

func (x WireString) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(vdl.TypeOf((*WireString)(nil))); err != nil {
		return err
	}
	if err := enc.EncodeInt(int64(x)); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *WireString) VDLRead(dec vdl.Decoder) error {
	if err := dec.StartValue(); err != nil {
		return err
	}
	tmp, err := dec.DecodeInt(32)
	if err != nil {
		return err
	}
	*x = WireString(tmp)
	return dec.FinishValue()
}

type WireTime int32

func (WireTime) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/vdl/testdata/nativetest.WireTime"`
}) {
}

func (x WireTime) VDLIsZero() bool {
	return x == 0
}

func (x WireTime) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(vdl.TypeOf((*WireTime)(nil))); err != nil {
		return err
	}
	if err := enc.EncodeInt(int64(x)); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *WireTime) VDLRead(dec vdl.Decoder) error {
	if err := dec.StartValue(); err != nil {
		return err
	}
	tmp, err := dec.DecodeInt(32)
	if err != nil {
		return err
	}
	*x = WireTime(tmp)
	return dec.FinishValue()
}

type WireSamePkg int32

func (WireSamePkg) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/vdl/testdata/nativetest.WireSamePkg"`
}) {
}

func (x WireSamePkg) VDLIsZero() bool {
	return x == 0
}

func (x WireSamePkg) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(vdl.TypeOf((*WireSamePkg)(nil))); err != nil {
		return err
	}
	if err := enc.EncodeInt(int64(x)); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *WireSamePkg) VDLRead(dec vdl.Decoder) error {
	if err := dec.StartValue(); err != nil {
		return err
	}
	tmp, err := dec.DecodeInt(32)
	if err != nil {
		return err
	}
	*x = WireSamePkg(tmp)
	return dec.FinishValue()
}

type WireMultiImport int32

func (WireMultiImport) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/vdl/testdata/nativetest.WireMultiImport"`
}) {
}

func (x WireMultiImport) VDLIsZero() bool {
	return x == 0
}

func (x WireMultiImport) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(vdl.TypeOf((*WireMultiImport)(nil))); err != nil {
		return err
	}
	if err := enc.EncodeInt(int64(x)); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *WireMultiImport) VDLRead(dec vdl.Decoder) error {
	if err := dec.StartValue(); err != nil {
		return err
	}
	tmp, err := dec.DecodeInt(32)
	if err != nil {
		return err
	}
	*x = WireMultiImport(tmp)
	return dec.FinishValue()
}

type WireRenameMe int32

func (WireRenameMe) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/vdl/testdata/nativetest.WireRenameMe"`
}) {
}

func (x WireRenameMe) VDLIsZero() bool {
	return x == 0
}

func (x WireRenameMe) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(vdl.TypeOf((*WireRenameMe)(nil))); err != nil {
		return err
	}
	if err := enc.EncodeInt(int64(x)); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *WireRenameMe) VDLRead(dec vdl.Decoder) error {
	if err := dec.StartValue(); err != nil {
		return err
	}
	tmp, err := dec.DecodeInt(32)
	if err != nil {
		return err
	}
	*x = WireRenameMe(tmp)
	return dec.FinishValue()
}

type WireAll struct {
	A string
	B time.Time
	C nativetest.NativeSamePkg
	D map[nativetest.NativeSamePkg]time.Time
	E WireRenameMe
}

func (WireAll) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/vdl/testdata/nativetest.WireAll"`
}) {
}

func (x WireAll) VDLIsZero() bool {
	if x.A != "" {
		return false
	}
	if !x.B.IsZero() {
		return false
	}
	if x.C != "" {
		return false
	}
	if x.D != nil {
		return false
	}
	if x.E != 0 {
		return false
	}
	return true
}

func (x WireAll) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(vdl.TypeOf((*WireAll)(nil)).Elem()); err != nil {
		return err
	}
	if x.A != "" {
		if err := enc.NextField("A"); err != nil {
			return err
		}
		var wire WireString
		if err := WireStringFromNative(&wire, x.A); err != nil {
			return err
		}
		if err := wire.VDLWrite(enc); err != nil {
			return err
		}
	}
	if !x.B.IsZero() {
		if err := enc.NextField("B"); err != nil {
			return err
		}
		var wire WireTime
		if err := WireTimeFromNative(&wire, x.B); err != nil {
			return err
		}
		if err := wire.VDLWrite(enc); err != nil {
			return err
		}
	}
	if x.C != "" {
		if err := enc.NextField("C"); err != nil {
			return err
		}
		var wire WireSamePkg
		if err := WireSamePkgFromNative(&wire, x.C); err != nil {
			return err
		}
		if err := wire.VDLWrite(enc); err != nil {
			return err
		}
	}
	if x.D != nil {
		if err := enc.NextField("D"); err != nil {
			return err
		}
		var wire WireMultiImport
		if err := WireMultiImportFromNative(&wire, x.D); err != nil {
			return err
		}
		if err := wire.VDLWrite(enc); err != nil {
			return err
		}
	}
	if x.E != 0 {
		if err := enc.NextField("E"); err != nil {
			return err
		}
		if err := x.E.VDLWrite(enc); err != nil {
			return err
		}
	}
	if err := enc.NextField(""); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *WireAll) VDLRead(dec vdl.Decoder) error {
	*x = WireAll{}
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
		case "A":
			var wire WireString
			if err := wire.VDLRead(dec); err != nil {
				return err
			}
			if err := WireStringToNative(wire, &x.A); err != nil {
				return err
			}
		case "B":
			var wire WireTime
			if err := wire.VDLRead(dec); err != nil {
				return err
			}
			if err := WireTimeToNative(wire, &x.B); err != nil {
				return err
			}
		case "C":
			var wire WireSamePkg
			if err := wire.VDLRead(dec); err != nil {
				return err
			}
			if err := WireSamePkgToNative(wire, &x.C); err != nil {
				return err
			}
		case "D":
			var wire WireMultiImport
			if err := wire.VDLRead(dec); err != nil {
				return err
			}
			if err := WireMultiImportToNative(wire, &x.D); err != nil {
				return err
			}
		case "E":
			if err := x.E.VDLRead(dec); err != nil {
				return err
			}
		default:
			if err := dec.SkipValue(); err != nil {
				return err
			}
		}
	}
}

type ignoreme string

func (ignoreme) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/vdl/testdata/nativetest.ignoreme"`
}) {
}

func (x ignoreme) VDLIsZero() bool {
	return x == ""
}

func (x ignoreme) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(vdl.TypeOf((*ignoreme)(nil))); err != nil {
		return err
	}
	if err := enc.EncodeString(string(x)); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *ignoreme) VDLRead(dec vdl.Decoder) error {
	if err := dec.StartValue(); err != nil {
		return err
	}
	tmp, err := dec.DecodeString()
	if err != nil {
		return err
	}
	*x = ignoreme(tmp)
	return dec.FinishValue()
}

// Type-check native conversion functions.
var (
	_ func(WireMultiImport, *map[nativetest.NativeSamePkg]time.Time) error = WireMultiImportToNative
	_ func(*WireMultiImport, map[nativetest.NativeSamePkg]time.Time) error = WireMultiImportFromNative
	_ func(WireSamePkg, *nativetest.NativeSamePkg) error                   = WireSamePkgToNative
	_ func(*WireSamePkg, nativetest.NativeSamePkg) error                   = WireSamePkgFromNative
	_ func(WireString, *string) error                                      = WireStringToNative
	_ func(*WireString, string) error                                      = WireStringFromNative
	_ func(WireTime, *time.Time) error                                     = WireTimeToNative
	_ func(*WireTime, time.Time) error                                     = WireTimeFromNative
)

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

	// Register native type conversions first, so that vdl.TypeOf works.
	vdl.RegisterNative(WireMultiImportToNative, WireMultiImportFromNative)
	vdl.RegisterNative(WireSamePkgToNative, WireSamePkgFromNative)
	vdl.RegisterNative(WireStringToNative, WireStringFromNative)
	vdl.RegisterNative(WireTimeToNative, WireTimeFromNative)

	// Register types.
	vdl.Register((*WireString)(nil))
	vdl.Register((*WireTime)(nil))
	vdl.Register((*WireSamePkg)(nil))
	vdl.Register((*WireMultiImport)(nil))
	vdl.Register((*WireRenameMe)(nil))
	vdl.Register((*WireAll)(nil))
	vdl.Register((*ignoreme)(nil))

	return struct{}{}
}
