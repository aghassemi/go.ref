// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: profile

// Package profile defines types for the implementation of Vanadium profiles.
package profile

import (
	"v.io/v23/services/build"
	"v.io/v23/vdl"
)

var _ = __VDLInit() // Must be first; see __VDLInit comments for details.

//////////////////////////////////////////////////
// Type definitions

// Library describes a shared library that applications may use.
type Library struct {
	// Name is the name of the library.
	Name string
	// MajorVersion is the major version of the library.
	MajorVersion string
	// MinorVersion is the minor version of the library.
	MinorVersion string
}

func (Library) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/profile.Library"`
}) {
}

func (x Library) VDLIsZero() bool {
	return x == Library{}
}

func (x Library) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(__VDLType_struct_1); err != nil {
		return err
	}
	if x.Name != "" {
		if err := enc.NextFieldValueString(0, vdl.StringType, x.Name); err != nil {
			return err
		}
	}
	if x.MajorVersion != "" {
		if err := enc.NextFieldValueString(1, vdl.StringType, x.MajorVersion); err != nil {
			return err
		}
	}
	if x.MinorVersion != "" {
		if err := enc.NextFieldValueString(2, vdl.StringType, x.MinorVersion); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *Library) VDLRead(dec vdl.Decoder) error {
	*x = Library{}
	if err := dec.StartValue(__VDLType_struct_1); err != nil {
		return err
	}
	decType := dec.Type()
	for {
		index, err := dec.NextField()
		switch {
		case err != nil:
			return err
		case index == -1:
			return dec.FinishValue()
		}
		if decType != __VDLType_struct_1 {
			index = __VDLType_struct_1.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		switch index {
		case 0:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Name = value
			}
		case 1:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.MajorVersion = value
			}
		case 2:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.MinorVersion = value
			}
		}
	}
}

// Specification is how we represent a profile internally. It should
// provide enough information to allow matching of binaries to devices.
type Specification struct {
	// Label is a human-friendly concise label for the profile,
	// e.g. "linux-media".
	Label string
	// Description is a human-friendly description of the profile.
	Description string
	// Arch is the target hardware architecture of the profile.
	Arch build.Architecture
	// Os is the target operating system of the profile.
	Os build.OperatingSystem
	// Format is the file format supported by the profile.
	Format build.Format
	// Libraries is a set of libraries the profile requires.
	Libraries map[Library]struct{}
}

func (Specification) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/profile.Specification"`
}) {
}

func (x Specification) VDLIsZero() bool {
	if x.Label != "" {
		return false
	}
	if x.Description != "" {
		return false
	}
	if x.Arch != build.ArchitectureAmd64 {
		return false
	}
	if x.Os != build.OperatingSystemDarwin {
		return false
	}
	if x.Format != build.FormatElf {
		return false
	}
	if len(x.Libraries) != 0 {
		return false
	}
	return true
}

func (x Specification) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(__VDLType_struct_2); err != nil {
		return err
	}
	if x.Label != "" {
		if err := enc.NextFieldValueString(0, vdl.StringType, x.Label); err != nil {
			return err
		}
	}
	if x.Description != "" {
		if err := enc.NextFieldValueString(1, vdl.StringType, x.Description); err != nil {
			return err
		}
	}
	if x.Arch != build.ArchitectureAmd64 {
		if err := enc.NextFieldValueString(2, __VDLType_enum_3, x.Arch.String()); err != nil {
			return err
		}
	}
	if x.Os != build.OperatingSystemDarwin {
		if err := enc.NextFieldValueString(3, __VDLType_enum_4, x.Os.String()); err != nil {
			return err
		}
	}
	if x.Format != build.FormatElf {
		if err := enc.NextFieldValueString(4, __VDLType_enum_5, x.Format.String()); err != nil {
			return err
		}
	}
	if len(x.Libraries) != 0 {
		if err := enc.NextField(5); err != nil {
			return err
		}
		if err := __VDLWriteAnon_set_1(enc, x.Libraries); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func __VDLWriteAnon_set_1(enc vdl.Encoder, x map[Library]struct{}) error {
	if err := enc.StartValue(__VDLType_set_6); err != nil {
		return err
	}
	if err := enc.SetLenHint(len(x)); err != nil {
		return err
	}
	for key := range x {
		if err := enc.NextEntry(false); err != nil {
			return err
		}
		if err := key.VDLWrite(enc); err != nil {
			return err
		}
	}
	if err := enc.NextEntry(true); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *Specification) VDLRead(dec vdl.Decoder) error {
	*x = Specification{}
	if err := dec.StartValue(__VDLType_struct_2); err != nil {
		return err
	}
	decType := dec.Type()
	for {
		index, err := dec.NextField()
		switch {
		case err != nil:
			return err
		case index == -1:
			return dec.FinishValue()
		}
		if decType != __VDLType_struct_2 {
			index = __VDLType_struct_2.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		switch index {
		case 0:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Label = value
			}
		case 1:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Description = value
			}
		case 2:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				if err := x.Arch.Set(value); err != nil {
					return err
				}
			}
		case 3:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				if err := x.Os.Set(value); err != nil {
					return err
				}
			}
		case 4:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				if err := x.Format.Set(value); err != nil {
					return err
				}
			}
		case 5:
			if err := __VDLReadAnon_set_1(dec, &x.Libraries); err != nil {
				return err
			}
		}
	}
}

func __VDLReadAnon_set_1(dec vdl.Decoder, x *map[Library]struct{}) error {
	if err := dec.StartValue(__VDLType_set_6); err != nil {
		return err
	}
	var tmpMap map[Library]struct{}
	if len := dec.LenHint(); len > 0 {
		tmpMap = make(map[Library]struct{}, len)
	}
	for {
		switch done, err := dec.NextEntry(); {
		case err != nil:
			return err
		case done:
			*x = tmpMap
			return dec.FinishValue()
		default:
			var key Library
			if err := key.VDLRead(dec); err != nil {
				return err
			}
			if tmpMap == nil {
				tmpMap = make(map[Library]struct{})
			}
			tmpMap[key] = struct{}{}
		}
	}
}

// Hold type definitions in package-level variables, for better performance.
var (
	__VDLType_struct_1 *vdl.Type
	__VDLType_struct_2 *vdl.Type
	__VDLType_enum_3   *vdl.Type
	__VDLType_enum_4   *vdl.Type
	__VDLType_enum_5   *vdl.Type
	__VDLType_set_6    *vdl.Type
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

	// Register types.
	vdl.Register((*Library)(nil))
	vdl.Register((*Specification)(nil))

	// Initialize type definitions.
	__VDLType_struct_1 = vdl.TypeOf((*Library)(nil)).Elem()
	__VDLType_struct_2 = vdl.TypeOf((*Specification)(nil)).Elem()
	__VDLType_enum_3 = vdl.TypeOf((*build.Architecture)(nil))
	__VDLType_enum_4 = vdl.TypeOf((*build.OperatingSystem)(nil))
	__VDLType_enum_5 = vdl.TypeOf((*build.Format)(nil))
	__VDLType_set_6 = vdl.TypeOf((*map[Library]struct{})(nil))

	return struct{}{}
}
