// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: watchable

package watchable

import (
	"fmt"
	"reflect"
	"v.io/v23/vdl"
	"v.io/v23/vom"
)

var _ = __VDLInit() // Must be first; see __VDLInit comments for details.

//////////////////////////////////////////////////
// Type definitions

// GetOp represents a store get operation.
type GetOp struct {
	Key []byte
}

func (GetOp) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/syncbase/store/watchable.GetOp"`
}) {
}

func (m *GetOp) FillVDLTarget(t vdl.Target, tt *vdl.Type) error {
	fieldsTarget1, err := t.StartFields(tt)
	if err != nil {
		return err
	}
	var var4 bool
	if len(m.Key) == 0 {
		var4 = true
	}
	if var4 {
		if err := fieldsTarget1.ZeroField("Key"); err != nil && err != vdl.ErrFieldNoExist {
			return err
		}
	} else {
		keyTarget2, fieldTarget3, err := fieldsTarget1.StartField("Key")
		if err != vdl.ErrFieldNoExist {
			if err != nil {
				return err
			}

			if err := fieldTarget3.FromBytes([]byte(m.Key), tt.NonOptional().Field(0).Type); err != nil {
				return err
			}
			if err := fieldsTarget1.FinishField(keyTarget2, fieldTarget3); err != nil {
				return err
			}
		}
	}
	if err := t.FinishFields(fieldsTarget1); err != nil {
		return err
	}
	return nil
}

func (m *GetOp) MakeVDLTarget() vdl.Target {
	return &GetOpTarget{Value: m}
}

type GetOpTarget struct {
	Value     *GetOp
	keyTarget vdl.BytesTarget
	vdl.TargetBase
	vdl.FieldsTargetBase
}

func (t *GetOpTarget) StartFields(tt *vdl.Type) (vdl.FieldsTarget, error) {

	if ttWant := vdl.TypeOf((*GetOp)(nil)).Elem(); !vdl.Compatible(tt, ttWant) {
		return nil, fmt.Errorf("type %v incompatible with %v", tt, ttWant)
	}
	return t, nil
}
func (t *GetOpTarget) StartField(name string) (key, field vdl.Target, _ error) {
	switch name {
	case "Key":
		t.keyTarget.Value = &t.Value.Key
		target, err := &t.keyTarget, error(nil)
		return nil, target, err
	default:
		return nil, nil, fmt.Errorf("field %s not in struct v.io/x/ref/services/syncbase/store/watchable.GetOp", name)
	}
}
func (t *GetOpTarget) FinishField(_, _ vdl.Target) error {
	return nil
}
func (t *GetOpTarget) ZeroField(name string) error {
	switch name {
	case "Key":
		t.Value.Key = []byte(nil)
		return nil
	default:
		return fmt.Errorf("field %s not in struct v.io/x/ref/services/syncbase/store/watchable.GetOp", name)
	}
}
func (t *GetOpTarget) FinishFields(_ vdl.FieldsTarget) error {

	return nil
}

func (x *GetOp) VDLRead(dec vdl.Decoder) error {
	*x = GetOp{}
	var err error
	if err = dec.StartValue(); err != nil {
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
		case "Key":
			if err = dec.StartValue(); err != nil {
				return err
			}
			if err = dec.DecodeBytes(-1, &x.Key); err != nil {
				return err
			}
			if err = dec.FinishValue(); err != nil {
				return err
			}
		default:
			if err = dec.SkipValue(); err != nil {
				return err
			}
		}
	}
}

// ScanOp represents a store scan operation.
type ScanOp struct {
	Start []byte
	Limit []byte
}

func (ScanOp) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/syncbase/store/watchable.ScanOp"`
}) {
}

func (m *ScanOp) FillVDLTarget(t vdl.Target, tt *vdl.Type) error {
	fieldsTarget1, err := t.StartFields(tt)
	if err != nil {
		return err
	}
	var var4 bool
	if len(m.Start) == 0 {
		var4 = true
	}
	if var4 {
		if err := fieldsTarget1.ZeroField("Start"); err != nil && err != vdl.ErrFieldNoExist {
			return err
		}
	} else {
		keyTarget2, fieldTarget3, err := fieldsTarget1.StartField("Start")
		if err != vdl.ErrFieldNoExist {
			if err != nil {
				return err
			}

			if err := fieldTarget3.FromBytes([]byte(m.Start), tt.NonOptional().Field(0).Type); err != nil {
				return err
			}
			if err := fieldsTarget1.FinishField(keyTarget2, fieldTarget3); err != nil {
				return err
			}
		}
	}
	var var7 bool
	if len(m.Limit) == 0 {
		var7 = true
	}
	if var7 {
		if err := fieldsTarget1.ZeroField("Limit"); err != nil && err != vdl.ErrFieldNoExist {
			return err
		}
	} else {
		keyTarget5, fieldTarget6, err := fieldsTarget1.StartField("Limit")
		if err != vdl.ErrFieldNoExist {
			if err != nil {
				return err
			}

			if err := fieldTarget6.FromBytes([]byte(m.Limit), tt.NonOptional().Field(1).Type); err != nil {
				return err
			}
			if err := fieldsTarget1.FinishField(keyTarget5, fieldTarget6); err != nil {
				return err
			}
		}
	}
	if err := t.FinishFields(fieldsTarget1); err != nil {
		return err
	}
	return nil
}

func (m *ScanOp) MakeVDLTarget() vdl.Target {
	return &ScanOpTarget{Value: m}
}

type ScanOpTarget struct {
	Value       *ScanOp
	startTarget vdl.BytesTarget
	limitTarget vdl.BytesTarget
	vdl.TargetBase
	vdl.FieldsTargetBase
}

func (t *ScanOpTarget) StartFields(tt *vdl.Type) (vdl.FieldsTarget, error) {

	if ttWant := vdl.TypeOf((*ScanOp)(nil)).Elem(); !vdl.Compatible(tt, ttWant) {
		return nil, fmt.Errorf("type %v incompatible with %v", tt, ttWant)
	}
	return t, nil
}
func (t *ScanOpTarget) StartField(name string) (key, field vdl.Target, _ error) {
	switch name {
	case "Start":
		t.startTarget.Value = &t.Value.Start
		target, err := &t.startTarget, error(nil)
		return nil, target, err
	case "Limit":
		t.limitTarget.Value = &t.Value.Limit
		target, err := &t.limitTarget, error(nil)
		return nil, target, err
	default:
		return nil, nil, fmt.Errorf("field %s not in struct v.io/x/ref/services/syncbase/store/watchable.ScanOp", name)
	}
}
func (t *ScanOpTarget) FinishField(_, _ vdl.Target) error {
	return nil
}
func (t *ScanOpTarget) ZeroField(name string) error {
	switch name {
	case "Start":
		t.Value.Start = []byte(nil)
		return nil
	case "Limit":
		t.Value.Limit = []byte(nil)
		return nil
	default:
		return fmt.Errorf("field %s not in struct v.io/x/ref/services/syncbase/store/watchable.ScanOp", name)
	}
}
func (t *ScanOpTarget) FinishFields(_ vdl.FieldsTarget) error {

	return nil
}

func (x *ScanOp) VDLRead(dec vdl.Decoder) error {
	*x = ScanOp{}
	var err error
	if err = dec.StartValue(); err != nil {
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
		case "Start":
			if err = dec.StartValue(); err != nil {
				return err
			}
			if err = dec.DecodeBytes(-1, &x.Start); err != nil {
				return err
			}
			if err = dec.FinishValue(); err != nil {
				return err
			}
		case "Limit":
			if err = dec.StartValue(); err != nil {
				return err
			}
			if err = dec.DecodeBytes(-1, &x.Limit); err != nil {
				return err
			}
			if err = dec.FinishValue(); err != nil {
				return err
			}
		default:
			if err = dec.SkipValue(); err != nil {
				return err
			}
		}
	}
}

// PutOp represents a store put operation.  The new version is written instead
// of the value to avoid duplicating the user data in the store.  The version
// is used to access the user data of that specific mutation.
type PutOp struct {
	Key     []byte
	Version []byte
}

func (PutOp) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/syncbase/store/watchable.PutOp"`
}) {
}

func (m *PutOp) FillVDLTarget(t vdl.Target, tt *vdl.Type) error {
	fieldsTarget1, err := t.StartFields(tt)
	if err != nil {
		return err
	}
	var var4 bool
	if len(m.Key) == 0 {
		var4 = true
	}
	if var4 {
		if err := fieldsTarget1.ZeroField("Key"); err != nil && err != vdl.ErrFieldNoExist {
			return err
		}
	} else {
		keyTarget2, fieldTarget3, err := fieldsTarget1.StartField("Key")
		if err != vdl.ErrFieldNoExist {
			if err != nil {
				return err
			}

			if err := fieldTarget3.FromBytes([]byte(m.Key), tt.NonOptional().Field(0).Type); err != nil {
				return err
			}
			if err := fieldsTarget1.FinishField(keyTarget2, fieldTarget3); err != nil {
				return err
			}
		}
	}
	var var7 bool
	if len(m.Version) == 0 {
		var7 = true
	}
	if var7 {
		if err := fieldsTarget1.ZeroField("Version"); err != nil && err != vdl.ErrFieldNoExist {
			return err
		}
	} else {
		keyTarget5, fieldTarget6, err := fieldsTarget1.StartField("Version")
		if err != vdl.ErrFieldNoExist {
			if err != nil {
				return err
			}

			if err := fieldTarget6.FromBytes([]byte(m.Version), tt.NonOptional().Field(1).Type); err != nil {
				return err
			}
			if err := fieldsTarget1.FinishField(keyTarget5, fieldTarget6); err != nil {
				return err
			}
		}
	}
	if err := t.FinishFields(fieldsTarget1); err != nil {
		return err
	}
	return nil
}

func (m *PutOp) MakeVDLTarget() vdl.Target {
	return &PutOpTarget{Value: m}
}

type PutOpTarget struct {
	Value         *PutOp
	keyTarget     vdl.BytesTarget
	versionTarget vdl.BytesTarget
	vdl.TargetBase
	vdl.FieldsTargetBase
}

func (t *PutOpTarget) StartFields(tt *vdl.Type) (vdl.FieldsTarget, error) {

	if ttWant := vdl.TypeOf((*PutOp)(nil)).Elem(); !vdl.Compatible(tt, ttWant) {
		return nil, fmt.Errorf("type %v incompatible with %v", tt, ttWant)
	}
	return t, nil
}
func (t *PutOpTarget) StartField(name string) (key, field vdl.Target, _ error) {
	switch name {
	case "Key":
		t.keyTarget.Value = &t.Value.Key
		target, err := &t.keyTarget, error(nil)
		return nil, target, err
	case "Version":
		t.versionTarget.Value = &t.Value.Version
		target, err := &t.versionTarget, error(nil)
		return nil, target, err
	default:
		return nil, nil, fmt.Errorf("field %s not in struct v.io/x/ref/services/syncbase/store/watchable.PutOp", name)
	}
}
func (t *PutOpTarget) FinishField(_, _ vdl.Target) error {
	return nil
}
func (t *PutOpTarget) ZeroField(name string) error {
	switch name {
	case "Key":
		t.Value.Key = []byte(nil)
		return nil
	case "Version":
		t.Value.Version = []byte(nil)
		return nil
	default:
		return fmt.Errorf("field %s not in struct v.io/x/ref/services/syncbase/store/watchable.PutOp", name)
	}
}
func (t *PutOpTarget) FinishFields(_ vdl.FieldsTarget) error {

	return nil
}

func (x *PutOp) VDLRead(dec vdl.Decoder) error {
	*x = PutOp{}
	var err error
	if err = dec.StartValue(); err != nil {
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
		case "Key":
			if err = dec.StartValue(); err != nil {
				return err
			}
			if err = dec.DecodeBytes(-1, &x.Key); err != nil {
				return err
			}
			if err = dec.FinishValue(); err != nil {
				return err
			}
		case "Version":
			if err = dec.StartValue(); err != nil {
				return err
			}
			if err = dec.DecodeBytes(-1, &x.Version); err != nil {
				return err
			}
			if err = dec.FinishValue(); err != nil {
				return err
			}
		default:
			if err = dec.SkipValue(); err != nil {
				return err
			}
		}
	}
}

// DeleteOp represents a store delete operation.
type DeleteOp struct {
	Key []byte
}

func (DeleteOp) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/syncbase/store/watchable.DeleteOp"`
}) {
}

func (m *DeleteOp) FillVDLTarget(t vdl.Target, tt *vdl.Type) error {
	fieldsTarget1, err := t.StartFields(tt)
	if err != nil {
		return err
	}
	var var4 bool
	if len(m.Key) == 0 {
		var4 = true
	}
	if var4 {
		if err := fieldsTarget1.ZeroField("Key"); err != nil && err != vdl.ErrFieldNoExist {
			return err
		}
	} else {
		keyTarget2, fieldTarget3, err := fieldsTarget1.StartField("Key")
		if err != vdl.ErrFieldNoExist {
			if err != nil {
				return err
			}

			if err := fieldTarget3.FromBytes([]byte(m.Key), tt.NonOptional().Field(0).Type); err != nil {
				return err
			}
			if err := fieldsTarget1.FinishField(keyTarget2, fieldTarget3); err != nil {
				return err
			}
		}
	}
	if err := t.FinishFields(fieldsTarget1); err != nil {
		return err
	}
	return nil
}

func (m *DeleteOp) MakeVDLTarget() vdl.Target {
	return &DeleteOpTarget{Value: m}
}

type DeleteOpTarget struct {
	Value     *DeleteOp
	keyTarget vdl.BytesTarget
	vdl.TargetBase
	vdl.FieldsTargetBase
}

func (t *DeleteOpTarget) StartFields(tt *vdl.Type) (vdl.FieldsTarget, error) {

	if ttWant := vdl.TypeOf((*DeleteOp)(nil)).Elem(); !vdl.Compatible(tt, ttWant) {
		return nil, fmt.Errorf("type %v incompatible with %v", tt, ttWant)
	}
	return t, nil
}
func (t *DeleteOpTarget) StartField(name string) (key, field vdl.Target, _ error) {
	switch name {
	case "Key":
		t.keyTarget.Value = &t.Value.Key
		target, err := &t.keyTarget, error(nil)
		return nil, target, err
	default:
		return nil, nil, fmt.Errorf("field %s not in struct v.io/x/ref/services/syncbase/store/watchable.DeleteOp", name)
	}
}
func (t *DeleteOpTarget) FinishField(_, _ vdl.Target) error {
	return nil
}
func (t *DeleteOpTarget) ZeroField(name string) error {
	switch name {
	case "Key":
		t.Value.Key = []byte(nil)
		return nil
	default:
		return fmt.Errorf("field %s not in struct v.io/x/ref/services/syncbase/store/watchable.DeleteOp", name)
	}
}
func (t *DeleteOpTarget) FinishFields(_ vdl.FieldsTarget) error {

	return nil
}

func (x *DeleteOp) VDLRead(dec vdl.Decoder) error {
	*x = DeleteOp{}
	var err error
	if err = dec.StartValue(); err != nil {
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
		case "Key":
			if err = dec.StartValue(); err != nil {
				return err
			}
			if err = dec.DecodeBytes(-1, &x.Key); err != nil {
				return err
			}
			if err = dec.FinishValue(); err != nil {
				return err
			}
		default:
			if err = dec.SkipValue(); err != nil {
				return err
			}
		}
	}
}

// LogEntry represents a single store operation. This operation may have been
// part of a transaction, as signified by the Continued boolean. Read-only
// operations (and read-only transactions) are not logged.
type LogEntry struct {
	// The store operation that was performed.
	Op *vom.RawBytes
	// Time when the operation was committed in nanoseconds since the epoch.
	// Note: We don't use time.Time here because VDL's time.Time consists of
	// {Seconds int64, Nanos int32}, which is more expensive than a single int64.
	CommitTimestamp int64
	// Operation came from sync (used for echo suppression).
	// TODO(razvanm): this field is specific to syncbase. We should add a
	// generic way to add fields and use that instead.
	FromSync bool
	// If true, this entry is followed by more entries that belong to the same
	// commit as this entry.
	Continued bool
}

func (LogEntry) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/syncbase/store/watchable.LogEntry"`
}) {
}

func (m *LogEntry) FillVDLTarget(t vdl.Target, tt *vdl.Type) error {
	fieldsTarget1, err := t.StartFields(tt)
	if err != nil {
		return err
	}
	var4 := m.Op == nil || m.Op.IsNilAny()
	if var4 {
		if err := fieldsTarget1.ZeroField("Op"); err != nil && err != vdl.ErrFieldNoExist {
			return err
		}
	} else {
		keyTarget2, fieldTarget3, err := fieldsTarget1.StartField("Op")
		if err != vdl.ErrFieldNoExist {
			if err != nil {
				return err
			}

			if err := m.Op.FillVDLTarget(fieldTarget3, tt.NonOptional().Field(0).Type); err != nil {
				return err
			}
			if err := fieldsTarget1.FinishField(keyTarget2, fieldTarget3); err != nil {
				return err
			}
		}
	}
	var7 := (m.CommitTimestamp == int64(0))
	if var7 {
		if err := fieldsTarget1.ZeroField("CommitTimestamp"); err != nil && err != vdl.ErrFieldNoExist {
			return err
		}
	} else {
		keyTarget5, fieldTarget6, err := fieldsTarget1.StartField("CommitTimestamp")
		if err != vdl.ErrFieldNoExist {
			if err != nil {
				return err
			}
			if err := fieldTarget6.FromInt(int64(m.CommitTimestamp), tt.NonOptional().Field(1).Type); err != nil {
				return err
			}
			if err := fieldsTarget1.FinishField(keyTarget5, fieldTarget6); err != nil {
				return err
			}
		}
	}
	var10 := (m.FromSync == false)
	if var10 {
		if err := fieldsTarget1.ZeroField("FromSync"); err != nil && err != vdl.ErrFieldNoExist {
			return err
		}
	} else {
		keyTarget8, fieldTarget9, err := fieldsTarget1.StartField("FromSync")
		if err != vdl.ErrFieldNoExist {
			if err != nil {
				return err
			}
			if err := fieldTarget9.FromBool(bool(m.FromSync), tt.NonOptional().Field(2).Type); err != nil {
				return err
			}
			if err := fieldsTarget1.FinishField(keyTarget8, fieldTarget9); err != nil {
				return err
			}
		}
	}
	var13 := (m.Continued == false)
	if var13 {
		if err := fieldsTarget1.ZeroField("Continued"); err != nil && err != vdl.ErrFieldNoExist {
			return err
		}
	} else {
		keyTarget11, fieldTarget12, err := fieldsTarget1.StartField("Continued")
		if err != vdl.ErrFieldNoExist {
			if err != nil {
				return err
			}
			if err := fieldTarget12.FromBool(bool(m.Continued), tt.NonOptional().Field(3).Type); err != nil {
				return err
			}
			if err := fieldsTarget1.FinishField(keyTarget11, fieldTarget12); err != nil {
				return err
			}
		}
	}
	if err := t.FinishFields(fieldsTarget1); err != nil {
		return err
	}
	return nil
}

func (m *LogEntry) MakeVDLTarget() vdl.Target {
	return &LogEntryTarget{Value: m}
}

type LogEntryTarget struct {
	Value *LogEntry

	commitTimestampTarget vdl.Int64Target
	fromSyncTarget        vdl.BoolTarget
	continuedTarget       vdl.BoolTarget
	vdl.TargetBase
	vdl.FieldsTargetBase
}

func (t *LogEntryTarget) StartFields(tt *vdl.Type) (vdl.FieldsTarget, error) {

	if ttWant := vdl.TypeOf((*LogEntry)(nil)).Elem(); !vdl.Compatible(tt, ttWant) {
		return nil, fmt.Errorf("type %v incompatible with %v", tt, ttWant)
	}
	return t, nil
}
func (t *LogEntryTarget) StartField(name string) (key, field vdl.Target, _ error) {
	switch name {
	case "Op":
		target, err := vdl.ReflectTarget(reflect.ValueOf(&t.Value.Op))
		return nil, target, err
	case "CommitTimestamp":
		t.commitTimestampTarget.Value = &t.Value.CommitTimestamp
		target, err := &t.commitTimestampTarget, error(nil)
		return nil, target, err
	case "FromSync":
		t.fromSyncTarget.Value = &t.Value.FromSync
		target, err := &t.fromSyncTarget, error(nil)
		return nil, target, err
	case "Continued":
		t.continuedTarget.Value = &t.Value.Continued
		target, err := &t.continuedTarget, error(nil)
		return nil, target, err
	default:
		return nil, nil, fmt.Errorf("field %s not in struct v.io/x/ref/services/syncbase/store/watchable.LogEntry", name)
	}
}
func (t *LogEntryTarget) FinishField(_, _ vdl.Target) error {
	return nil
}
func (t *LogEntryTarget) ZeroField(name string) error {
	switch name {
	case "Op":
		t.Value.Op = vom.RawBytesOf(vdl.ZeroValue(vdl.AnyType))
		return nil
	case "CommitTimestamp":
		t.Value.CommitTimestamp = int64(0)
		return nil
	case "FromSync":
		t.Value.FromSync = false
		return nil
	case "Continued":
		t.Value.Continued = false
		return nil
	default:
		return fmt.Errorf("field %s not in struct v.io/x/ref/services/syncbase/store/watchable.LogEntry", name)
	}
}
func (t *LogEntryTarget) FinishFields(_ vdl.FieldsTarget) error {

	return nil
}

func (x *LogEntry) VDLRead(dec vdl.Decoder) error {
	*x = LogEntry{
		Op: vom.RawBytesOf(vdl.ZeroValue(vdl.AnyType)),
	}
	var err error
	if err = dec.StartValue(); err != nil {
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
		case "Op":
			// TODO(toddw): implement any
		case "CommitTimestamp":
			if err = dec.StartValue(); err != nil {
				return err
			}
			if x.CommitTimestamp, err = dec.DecodeInt(64); err != nil {
				return err
			}
			if err = dec.FinishValue(); err != nil {
				return err
			}
		case "FromSync":
			if err = dec.StartValue(); err != nil {
				return err
			}
			if x.FromSync, err = dec.DecodeBool(); err != nil {
				return err
			}
			if err = dec.FinishValue(); err != nil {
				return err
			}
		case "Continued":
			if err = dec.StartValue(); err != nil {
				return err
			}
			if x.Continued, err = dec.DecodeBool(); err != nil {
				return err
			}
			if err = dec.FinishValue(); err != nil {
				return err
			}
		default:
			if err = dec.SkipValue(); err != nil {
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
	vdl.Register((*GetOp)(nil))
	vdl.Register((*ScanOp)(nil))
	vdl.Register((*PutOp)(nil))
	vdl.Register((*DeleteOp)(nil))
	vdl.Register((*LogEntry)(nil))

	return struct{}{}
}
