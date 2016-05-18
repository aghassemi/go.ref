// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: serialization

package serialization

import (
	"fmt"
	"v.io/v23/security"
	"v.io/v23/vdl"
)

var _ = __VDLInit() // Must be first; see __VDLInit comments for details.

//////////////////////////////////////////////////
// Type definitions

type SignedHeader struct {
	ChunkSizeBytes int64
}

func (SignedHeader) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/security/serialization.SignedHeader"`
}) {
}

func (x SignedHeader) VDLIsZero() bool {
	return x == SignedHeader{}
}

func (x SignedHeader) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(vdl.TypeOf((*SignedHeader)(nil)).Elem()); err != nil {
		return err
	}
	if x.ChunkSizeBytes != 0 {
		if err := enc.NextField("ChunkSizeBytes"); err != nil {
			return err
		}
		if err := enc.StartValue(vdl.Int64Type); err != nil {
			return err
		}
		if err := enc.EncodeInt(x.ChunkSizeBytes); err != nil {
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

func (x *SignedHeader) VDLRead(dec vdl.Decoder) error {
	*x = SignedHeader{}
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
		case "ChunkSizeBytes":
			if err := dec.StartValue(); err != nil {
				return err
			}
			var err error
			if x.ChunkSizeBytes, err = dec.DecodeInt(64); err != nil {
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

type HashCode [32]byte

func (HashCode) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/security/serialization.HashCode"`
}) {
}

func (x HashCode) VDLIsZero() bool {
	return x == HashCode{}
}

func (x HashCode) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(vdl.TypeOf((*HashCode)(nil))); err != nil {
		return err
	}
	if err := enc.EncodeBytes([]byte(x[:])); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *HashCode) VDLRead(dec vdl.Decoder) error {
	if err := dec.StartValue(); err != nil {
		return err
	}
	bytes := x[:]
	if err := dec.DecodeBytes(32, &bytes); err != nil {
		return err
	}
	return dec.FinishValue()
}

type (
	// SignedData represents any single field of the SignedData union type.
	//
	// SignedData describes the information sent by a SigningWriter and read by VerifiyingReader.
	SignedData interface {
		// Index returns the field index.
		Index() int
		// Interface returns the field value as an interface.
		Interface() interface{}
		// Name returns the field name.
		Name() string
		// __VDLReflect describes the SignedData union type.
		__VDLReflect(__SignedDataReflect)
		VDLIsZero() bool
		VDLWrite(vdl.Encoder) error
	}
	// SignedDataSignature represents field Signature of the SignedData union type.
	SignedDataSignature struct{ Value security.Signature }
	// SignedDataHash represents field Hash of the SignedData union type.
	SignedDataHash struct{ Value HashCode }
	// __SignedDataReflect describes the SignedData union type.
	__SignedDataReflect struct {
		Name  string `vdl:"v.io/x/ref/lib/security/serialization.SignedData"`
		Type  SignedData
		Union struct {
			Signature SignedDataSignature
			Hash      SignedDataHash
		}
	}
)

func (x SignedDataSignature) Index() int                       { return 0 }
func (x SignedDataSignature) Interface() interface{}           { return x.Value }
func (x SignedDataSignature) Name() string                     { return "Signature" }
func (x SignedDataSignature) __VDLReflect(__SignedDataReflect) {}

func (x SignedDataHash) Index() int                       { return 1 }
func (x SignedDataHash) Interface() interface{}           { return x.Value }
func (x SignedDataHash) Name() string                     { return "Hash" }
func (x SignedDataHash) __VDLReflect(__SignedDataReflect) {}

func (x SignedDataSignature) VDLIsZero() bool {
	return x.Value.VDLIsZero()
}

func (x SignedDataHash) VDLIsZero() bool {
	return false
}

func (x SignedDataSignature) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(vdl.TypeOf((*SignedData)(nil))); err != nil {
		return err
	}
	if err := enc.NextField("Signature"); err != nil {
		return err
	}
	if err := x.Value.VDLWrite(enc); err != nil {
		return err
	}
	if err := enc.NextField(""); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x SignedDataHash) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(vdl.TypeOf((*SignedData)(nil))); err != nil {
		return err
	}
	if err := enc.NextField("Hash"); err != nil {
		return err
	}
	if err := x.Value.VDLWrite(enc); err != nil {
		return err
	}
	if err := enc.NextField(""); err != nil {
		return err
	}
	return enc.FinishValue()
}

func VDLReadSignedData(dec vdl.Decoder, x *SignedData) error {
	if err := dec.StartValue(); err != nil {
		return err
	}
	if (dec.StackDepth() == 1 || dec.IsAny()) && !vdl.Compatible(vdl.TypeOf(x), dec.Type()) {
		return fmt.Errorf("incompatible union %T, from %v", x, dec.Type())
	}
	f, err := dec.NextField()
	if err != nil {
		return err
	}
	switch f {
	case "Signature":
		var field SignedDataSignature
		if err := field.Value.VDLRead(dec); err != nil {
			return err
		}
		*x = field
	case "Hash":
		var field SignedDataHash
		if err := field.Value.VDLRead(dec); err != nil {
			return err
		}
		*x = field
	case "":
		return fmt.Errorf("missing field in union %T, from %v", x, dec.Type())
	default:
		return fmt.Errorf("field %q not in union %T, from %v", f, x, dec.Type())
	}
	switch f, err := dec.NextField(); {
	case err != nil:
		return err
	case f != "":
		return fmt.Errorf("extra field %q in union %T, from %v", f, x, dec.Type())
	}
	return dec.FinishValue()
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
	vdl.Register((*SignedHeader)(nil))
	vdl.Register((*HashCode)(nil))
	vdl.Register((*SignedData)(nil))

	return struct{}{}
}
