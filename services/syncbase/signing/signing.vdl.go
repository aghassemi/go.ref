// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: signing

package signing

import (
	"fmt"
	"v.io/v23/security"
	"v.io/v23/vdl"
)

var _ = __VDLInit() // Must be first; see __VDLInit comments for details.

//////////////////////////////////////////////////
// Type definitions

type (
	// Item represents any single field of the Item union type.
	//
	// An Item represents either a marshalled data item or its SHA-256 hash.
	// The Data field is a []byte, rather than an "any" to make signatures
	// determistic.  VOM encoding is not deterministic for two reasons:
	// - map elements may be marshalled in any order
	// - different versions of VOM may marshal in different ways.
	// Thus, the initial producer of a data item marshals the data once, and it is
	// this marshalled form that is transmitted from device to device.  If the
	// data were unmarshalled and then remarsahalled, the signatures might not
	// match.  The Hash field is used instead of the Data field when the recipient
	// of the DataWithSignature is not permitted to see certain Items' Data
	// fields.
	Item interface {
		// Index returns the field index.
		Index() int
		// Interface returns the field value as an interface.
		Interface() interface{}
		// Name returns the field name.
		Name() string
		// __VDLReflect describes the Item union type.
		__VDLReflect(__ItemReflect)
		FillVDLTarget(vdl.Target, *vdl.Type) error
	}
	// ItemData represents field Data of the Item union type.
	ItemData struct{ Value []byte } // Marshalled form of data.
	// ItemHash represents field Hash of the Item union type.
	ItemHash struct{ Value []byte } // Hash of what would have been in Data, as returned by SumByteVectorWithLength(Data).
	// __ItemReflect describes the Item union type.
	__ItemReflect struct {
		Name               string `vdl:"v.io/x/ref/services/syncbase/signing.Item"`
		Type               Item
		UnionTargetFactory itemTargetFactory
		Union              struct {
			Data ItemData
			Hash ItemHash
		}
	}
)

func (x ItemData) Index() int                 { return 0 }
func (x ItemData) Interface() interface{}     { return x.Value }
func (x ItemData) Name() string               { return "Data" }
func (x ItemData) __VDLReflect(__ItemReflect) {}

func (m ItemData) FillVDLTarget(t vdl.Target, tt *vdl.Type) error {
	fieldsTarget1, err := t.StartFields(tt)
	if err != nil {
		return err
	}
	keyTarget2, fieldTarget3, err := fieldsTarget1.StartField("Data")
	if err != nil {
		return err
	}

	if err := fieldTarget3.FromBytes([]byte(m.Value), tt.NonOptional().Field(0).Type); err != nil {
		return err
	}
	if err := fieldsTarget1.FinishField(keyTarget2, fieldTarget3); err != nil {
		return err
	}
	if err := t.FinishFields(fieldsTarget1); err != nil {
		return err
	}

	return nil
}

func (m ItemData) MakeVDLTarget() vdl.Target {
	return nil
}

func (x ItemHash) Index() int                 { return 1 }
func (x ItemHash) Interface() interface{}     { return x.Value }
func (x ItemHash) Name() string               { return "Hash" }
func (x ItemHash) __VDLReflect(__ItemReflect) {}

func (m ItemHash) FillVDLTarget(t vdl.Target, tt *vdl.Type) error {
	fieldsTarget1, err := t.StartFields(tt)
	if err != nil {
		return err
	}
	keyTarget2, fieldTarget3, err := fieldsTarget1.StartField("Hash")
	if err != nil {
		return err
	}

	if err := fieldTarget3.FromBytes([]byte(m.Value), tt.NonOptional().Field(1).Type); err != nil {
		return err
	}
	if err := fieldsTarget1.FinishField(keyTarget2, fieldTarget3); err != nil {
		return err
	}
	if err := t.FinishFields(fieldsTarget1); err != nil {
		return err
	}

	return nil
}

func (m ItemHash) MakeVDLTarget() vdl.Target {
	return nil
}

type ItemTarget struct {
	Value     *Item
	fieldName string

	vdl.TargetBase
	vdl.FieldsTargetBase
}

func (t *ItemTarget) StartFields(tt *vdl.Type) (vdl.FieldsTarget, error) {
	if ttWant := vdl.TypeOf((*Item)(nil)); !vdl.Compatible(tt, ttWant) {
		return nil, fmt.Errorf("type %v incompatible with %v", tt, ttWant)
	}

	return t, nil
}
func (t *ItemTarget) StartField(name string) (key, field vdl.Target, _ error) {
	t.fieldName = name
	switch name {
	case "Data":
		val := []byte(nil)
		return nil, &vdl.BytesTarget{Value: &val}, nil
	case "Hash":
		val := []byte(nil)
		return nil, &vdl.BytesTarget{Value: &val}, nil
	default:
		return nil, nil, fmt.Errorf("field %s not in union v.io/x/ref/services/syncbase/signing.Item", name)
	}
}
func (t *ItemTarget) FinishField(_, fieldTarget vdl.Target) error {
	switch t.fieldName {
	case "Data":
		*t.Value = ItemData{*(fieldTarget.(*vdl.BytesTarget)).Value}
	case "Hash":
		*t.Value = ItemHash{*(fieldTarget.(*vdl.BytesTarget)).Value}
	}
	return nil
}
func (t *ItemTarget) FinishFields(_ vdl.FieldsTarget) error {

	return nil
}

type itemTargetFactory struct{}

func (t itemTargetFactory) VDLMakeUnionTarget(union interface{}) (vdl.Target, error) {
	if typedUnion, ok := union.(*Item); ok {
		return &ItemTarget{Value: typedUnion}, nil
	}
	return nil, fmt.Errorf("got %T, want *Item", union)
}

func VDLReadItem(dec vdl.Decoder, x *Item) error {
	var err error
	if err = dec.StartValue(); err != nil {
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
	case "Data":
		var field ItemData
		if err = dec.StartValue(); err != nil {
			return err
		}
		if err = dec.DecodeBytes(-1, &field.Value); err != nil {
			return err
		}
		if err = dec.FinishValue(); err != nil {
			return err
		}
		*x = field
	case "Hash":
		var field ItemHash
		if err = dec.StartValue(); err != nil {
			return err
		}
		if err = dec.DecodeBytes(-1, &field.Value); err != nil {
			return err
		}
		if err = dec.FinishValue(); err != nil {
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

// A DataWithSignature represents a signed, and possibily validated, collection
// of Item structs.
//
// If IsValidated==false and the AuthorSigned signature is valid, it means:
//    The signer whose Blessings have hash BlessingsHash asserts Data.
//
// If IsValidated==true and both AuthorSigned and ValidatorSigned signatures are is valid,
// it means both:
// 1) The signer whose Blessings b have hash BlessingsHash asserts Data.
// 2) If vd is the ValidatorData with hash ValidatorDataHash, the owner of
//    vd.PublicKey asserts that it checked that at least the names vd.Names[] were
//    valid in b.
//
// The sender obtains:
// - BlessingsHash (and the wire form of the blessings) with ValidationCache.AddBlessings().
// - ValidatorDataHash (and the wire form of the ValidataData)  with ValidationCache.AddValidatorData().
//
// The receiver looks up:
// - BlessingsHash with ValidationCache.LookupBlessingsData()
// - ValidatorDataHash with ValidationCache.LookupValidatorData()
//
// If not yet there, the receiver inserts the valus into its ValidationCache with:
// - ValidationCache.AddWireBlessings()
// - ValidationCache.AddValidatorData()
type DataWithSignature struct {
	Data []Item
	// BlessingsHash is a key for the validation cache; the corresponding
	// cached value is a security.Blessings.
	BlessingsHash []byte
	// AuthorSigned is the signature of Data and BlessingsHash using the
	// private key associated with the blessings hashed in BlessingsHash.
	AuthorSigned security.Signature
	IsValidated  bool // Whether fields below are meaningful.
	// ValidatorDataHash is a key for the validation cache returned by
	// ValidatorData.Hash(); the corresponding cached value is the
	// ValidatorData.
	ValidatorDataHash []byte
	ValidatorSigned   security.Signature
}

func (DataWithSignature) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/syncbase/signing.DataWithSignature"`
}) {
}

func (m *DataWithSignature) FillVDLTarget(t vdl.Target, tt *vdl.Type) error {
	fieldsTarget1, err := t.StartFields(tt)
	if err != nil {
		return err
	}
	var var4 bool
	if len(m.Data) == 0 {
		var4 = true
	}
	if var4 {
		if err := fieldsTarget1.ZeroField("Data"); err != nil && err != vdl.ErrFieldNoExist {
			return err
		}
	} else {
		keyTarget2, fieldTarget3, err := fieldsTarget1.StartField("Data")
		if err != vdl.ErrFieldNoExist {
			if err != nil {
				return err
			}

			listTarget5, err := fieldTarget3.StartList(tt.NonOptional().Field(0).Type, len(m.Data))
			if err != nil {
				return err
			}
			for i, elem7 := range m.Data {
				elemTarget6, err := listTarget5.StartElem(i)
				if err != nil {
					return err
				}

				unionValue8 := elem7
				if unionValue8 == nil {
					unionValue8 = ItemData{}
				}
				if err := unionValue8.FillVDLTarget(elemTarget6, tt.NonOptional().Field(0).Type.Elem()); err != nil {
					return err
				}
				if err := listTarget5.FinishElem(elemTarget6); err != nil {
					return err
				}
			}
			if err := fieldTarget3.FinishList(listTarget5); err != nil {
				return err
			}
			if err := fieldsTarget1.FinishField(keyTarget2, fieldTarget3); err != nil {
				return err
			}
		}
	}
	var var11 bool
	if len(m.BlessingsHash) == 0 {
		var11 = true
	}
	if var11 {
		if err := fieldsTarget1.ZeroField("BlessingsHash"); err != nil && err != vdl.ErrFieldNoExist {
			return err
		}
	} else {
		keyTarget9, fieldTarget10, err := fieldsTarget1.StartField("BlessingsHash")
		if err != vdl.ErrFieldNoExist {
			if err != nil {
				return err
			}

			if err := fieldTarget10.FromBytes([]byte(m.BlessingsHash), tt.NonOptional().Field(1).Type); err != nil {
				return err
			}
			if err := fieldsTarget1.FinishField(keyTarget9, fieldTarget10); err != nil {
				return err
			}
		}
	}
	var14 := true
	var var15 bool
	if len(m.AuthorSigned.Purpose) == 0 {
		var15 = true
	}
	var14 = var14 && var15
	var16 := (m.AuthorSigned.Hash == security.Hash(""))
	var14 = var14 && var16
	var var17 bool
	if len(m.AuthorSigned.R) == 0 {
		var17 = true
	}
	var14 = var14 && var17
	var var18 bool
	if len(m.AuthorSigned.S) == 0 {
		var18 = true
	}
	var14 = var14 && var18
	if var14 {
		if err := fieldsTarget1.ZeroField("AuthorSigned"); err != nil && err != vdl.ErrFieldNoExist {
			return err
		}
	} else {
		keyTarget12, fieldTarget13, err := fieldsTarget1.StartField("AuthorSigned")
		if err != vdl.ErrFieldNoExist {
			if err != nil {
				return err
			}

			if err := m.AuthorSigned.FillVDLTarget(fieldTarget13, tt.NonOptional().Field(2).Type); err != nil {
				return err
			}
			if err := fieldsTarget1.FinishField(keyTarget12, fieldTarget13); err != nil {
				return err
			}
		}
	}
	var21 := (m.IsValidated == false)
	if var21 {
		if err := fieldsTarget1.ZeroField("IsValidated"); err != nil && err != vdl.ErrFieldNoExist {
			return err
		}
	} else {
		keyTarget19, fieldTarget20, err := fieldsTarget1.StartField("IsValidated")
		if err != vdl.ErrFieldNoExist {
			if err != nil {
				return err
			}
			if err := fieldTarget20.FromBool(bool(m.IsValidated), tt.NonOptional().Field(3).Type); err != nil {
				return err
			}
			if err := fieldsTarget1.FinishField(keyTarget19, fieldTarget20); err != nil {
				return err
			}
		}
	}
	var var24 bool
	if len(m.ValidatorDataHash) == 0 {
		var24 = true
	}
	if var24 {
		if err := fieldsTarget1.ZeroField("ValidatorDataHash"); err != nil && err != vdl.ErrFieldNoExist {
			return err
		}
	} else {
		keyTarget22, fieldTarget23, err := fieldsTarget1.StartField("ValidatorDataHash")
		if err != vdl.ErrFieldNoExist {
			if err != nil {
				return err
			}

			if err := fieldTarget23.FromBytes([]byte(m.ValidatorDataHash), tt.NonOptional().Field(4).Type); err != nil {
				return err
			}
			if err := fieldsTarget1.FinishField(keyTarget22, fieldTarget23); err != nil {
				return err
			}
		}
	}
	var27 := true
	var var28 bool
	if len(m.ValidatorSigned.Purpose) == 0 {
		var28 = true
	}
	var27 = var27 && var28
	var29 := (m.ValidatorSigned.Hash == security.Hash(""))
	var27 = var27 && var29
	var var30 bool
	if len(m.ValidatorSigned.R) == 0 {
		var30 = true
	}
	var27 = var27 && var30
	var var31 bool
	if len(m.ValidatorSigned.S) == 0 {
		var31 = true
	}
	var27 = var27 && var31
	if var27 {
		if err := fieldsTarget1.ZeroField("ValidatorSigned"); err != nil && err != vdl.ErrFieldNoExist {
			return err
		}
	} else {
		keyTarget25, fieldTarget26, err := fieldsTarget1.StartField("ValidatorSigned")
		if err != vdl.ErrFieldNoExist {
			if err != nil {
				return err
			}

			if err := m.ValidatorSigned.FillVDLTarget(fieldTarget26, tt.NonOptional().Field(5).Type); err != nil {
				return err
			}
			if err := fieldsTarget1.FinishField(keyTarget25, fieldTarget26); err != nil {
				return err
			}
		}
	}
	if err := t.FinishFields(fieldsTarget1); err != nil {
		return err
	}
	return nil
}

func (m *DataWithSignature) MakeVDLTarget() vdl.Target {
	return &DataWithSignatureTarget{Value: m}
}

type DataWithSignatureTarget struct {
	Value                   *DataWithSignature
	dataTarget              __VDLTarget1_list
	blessingsHashTarget     vdl.BytesTarget
	authorSignedTarget      security.SignatureTarget
	isValidatedTarget       vdl.BoolTarget
	validatorDataHashTarget vdl.BytesTarget
	validatorSignedTarget   security.SignatureTarget
	vdl.TargetBase
	vdl.FieldsTargetBase
}

func (t *DataWithSignatureTarget) StartFields(tt *vdl.Type) (vdl.FieldsTarget, error) {

	if ttWant := vdl.TypeOf((*DataWithSignature)(nil)).Elem(); !vdl.Compatible(tt, ttWant) {
		return nil, fmt.Errorf("type %v incompatible with %v", tt, ttWant)
	}
	return t, nil
}
func (t *DataWithSignatureTarget) StartField(name string) (key, field vdl.Target, _ error) {
	switch name {
	case "Data":
		t.dataTarget.Value = &t.Value.Data
		target, err := &t.dataTarget, error(nil)
		return nil, target, err
	case "BlessingsHash":
		t.blessingsHashTarget.Value = &t.Value.BlessingsHash
		target, err := &t.blessingsHashTarget, error(nil)
		return nil, target, err
	case "AuthorSigned":
		t.authorSignedTarget.Value = &t.Value.AuthorSigned
		target, err := &t.authorSignedTarget, error(nil)
		return nil, target, err
	case "IsValidated":
		t.isValidatedTarget.Value = &t.Value.IsValidated
		target, err := &t.isValidatedTarget, error(nil)
		return nil, target, err
	case "ValidatorDataHash":
		t.validatorDataHashTarget.Value = &t.Value.ValidatorDataHash
		target, err := &t.validatorDataHashTarget, error(nil)
		return nil, target, err
	case "ValidatorSigned":
		t.validatorSignedTarget.Value = &t.Value.ValidatorSigned
		target, err := &t.validatorSignedTarget, error(nil)
		return nil, target, err
	default:
		return nil, nil, fmt.Errorf("field %s not in struct v.io/x/ref/services/syncbase/signing.DataWithSignature", name)
	}
}
func (t *DataWithSignatureTarget) FinishField(_, _ vdl.Target) error {
	return nil
}
func (t *DataWithSignatureTarget) ZeroField(name string) error {
	switch name {
	case "Data":
		t.Value.Data = []Item(nil)
		return nil
	case "BlessingsHash":
		t.Value.BlessingsHash = []byte(nil)
		return nil
	case "AuthorSigned":
		t.Value.AuthorSigned = security.Signature{}
		return nil
	case "IsValidated":
		t.Value.IsValidated = false
		return nil
	case "ValidatorDataHash":
		t.Value.ValidatorDataHash = []byte(nil)
		return nil
	case "ValidatorSigned":
		t.Value.ValidatorSigned = security.Signature{}
		return nil
	default:
		return fmt.Errorf("field %s not in struct v.io/x/ref/services/syncbase/signing.DataWithSignature", name)
	}
}
func (t *DataWithSignatureTarget) FinishFields(_ vdl.FieldsTarget) error {

	return nil
}

// []Item
type __VDLTarget1_list struct {
	Value      *[]Item
	elemTarget ItemTarget
	vdl.TargetBase
	vdl.ListTargetBase
}

func (t *__VDLTarget1_list) StartList(tt *vdl.Type, len int) (vdl.ListTarget, error) {

	if ttWant := vdl.TypeOf((*[]Item)(nil)); !vdl.Compatible(tt, ttWant) {
		return nil, fmt.Errorf("type %v incompatible with %v", tt, ttWant)
	}
	if cap(*t.Value) < len {
		*t.Value = make([]Item, len)
	} else {
		*t.Value = (*t.Value)[:len]
	}
	return t, nil
}
func (t *__VDLTarget1_list) StartElem(index int) (elem vdl.Target, _ error) {
	t.elemTarget.Value = &(*t.Value)[index]
	target, err := &t.elemTarget, error(nil)
	return target, err
}
func (t *__VDLTarget1_list) FinishElem(elem vdl.Target) error {
	return nil
}
func (t *__VDLTarget1_list) FinishList(elem vdl.ListTarget) error {

	return nil
}

func (x *DataWithSignature) VDLRead(dec vdl.Decoder) error {
	*x = DataWithSignature{}
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
		case "Data":
			if err = __VDLRead1_list(dec, &x.Data); err != nil {
				return err
			}
		case "BlessingsHash":
			if err = dec.StartValue(); err != nil {
				return err
			}
			if err = dec.DecodeBytes(-1, &x.BlessingsHash); err != nil {
				return err
			}
			if err = dec.FinishValue(); err != nil {
				return err
			}
		case "AuthorSigned":
			if err = x.AuthorSigned.VDLRead(dec); err != nil {
				return err
			}
		case "IsValidated":
			if err = dec.StartValue(); err != nil {
				return err
			}
			if x.IsValidated, err = dec.DecodeBool(); err != nil {
				return err
			}
			if err = dec.FinishValue(); err != nil {
				return err
			}
		case "ValidatorDataHash":
			if err = dec.StartValue(); err != nil {
				return err
			}
			if err = dec.DecodeBytes(-1, &x.ValidatorDataHash); err != nil {
				return err
			}
			if err = dec.FinishValue(); err != nil {
				return err
			}
		case "ValidatorSigned":
			if err = x.ValidatorSigned.VDLRead(dec); err != nil {
				return err
			}
		default:
			if err = dec.SkipValue(); err != nil {
				return err
			}
		}
	}
}

func __VDLRead1_list(dec vdl.Decoder, x *[]Item) error {
	var err error
	if err = dec.StartValue(); err != nil {
		return err
	}
	if (dec.StackDepth() == 1 || dec.IsAny()) && !vdl.Compatible(vdl.TypeOf(*x), dec.Type()) {
		return fmt.Errorf("incompatible list %T, from %v", *x, dec.Type())
	}
	switch len := dec.LenHint(); {
	case len > 0:
		*x = make([]Item, 0, len)
	default:
		*x = nil
	}
	for {
		switch done, err := dec.NextEntry(); {
		case err != nil:
			return err
		case done:
			return dec.FinishValue()
		}
		var elem Item
		if err = VDLReadItem(dec, &elem); err != nil {
			return err
		}
		*x = append(*x, elem)
	}
}

// WireValidatorData is the wire form of ValidatorData.
// It excludes the unmarshalled form of the public key.
type WireValidatorData struct {
	Names               []string // Names of valid signing blessings in the Blessings referred to by BlessingsHash.
	MarshalledPublicKey []byte   // PublicKey, marshalled with MarshalBinary().
}

func (WireValidatorData) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/syncbase/signing.WireValidatorData"`
}) {
}

func (m *WireValidatorData) FillVDLTarget(t vdl.Target, tt *vdl.Type) error {
	fieldsTarget1, err := t.StartFields(tt)
	if err != nil {
		return err
	}
	var var4 bool
	if len(m.Names) == 0 {
		var4 = true
	}
	if var4 {
		if err := fieldsTarget1.ZeroField("Names"); err != nil && err != vdl.ErrFieldNoExist {
			return err
		}
	} else {
		keyTarget2, fieldTarget3, err := fieldsTarget1.StartField("Names")
		if err != vdl.ErrFieldNoExist {
			if err != nil {
				return err
			}

			listTarget5, err := fieldTarget3.StartList(tt.NonOptional().Field(0).Type, len(m.Names))
			if err != nil {
				return err
			}
			for i, elem7 := range m.Names {
				elemTarget6, err := listTarget5.StartElem(i)
				if err != nil {
					return err
				}
				if err := elemTarget6.FromString(string(elem7), tt.NonOptional().Field(0).Type.Elem()); err != nil {
					return err
				}
				if err := listTarget5.FinishElem(elemTarget6); err != nil {
					return err
				}
			}
			if err := fieldTarget3.FinishList(listTarget5); err != nil {
				return err
			}
			if err := fieldsTarget1.FinishField(keyTarget2, fieldTarget3); err != nil {
				return err
			}
		}
	}
	var var10 bool
	if len(m.MarshalledPublicKey) == 0 {
		var10 = true
	}
	if var10 {
		if err := fieldsTarget1.ZeroField("MarshalledPublicKey"); err != nil && err != vdl.ErrFieldNoExist {
			return err
		}
	} else {
		keyTarget8, fieldTarget9, err := fieldsTarget1.StartField("MarshalledPublicKey")
		if err != vdl.ErrFieldNoExist {
			if err != nil {
				return err
			}

			if err := fieldTarget9.FromBytes([]byte(m.MarshalledPublicKey), tt.NonOptional().Field(1).Type); err != nil {
				return err
			}
			if err := fieldsTarget1.FinishField(keyTarget8, fieldTarget9); err != nil {
				return err
			}
		}
	}
	if err := t.FinishFields(fieldsTarget1); err != nil {
		return err
	}
	return nil
}

func (m *WireValidatorData) MakeVDLTarget() vdl.Target {
	return &WireValidatorDataTarget{Value: m}
}

type WireValidatorDataTarget struct {
	Value                     *WireValidatorData
	namesTarget               vdl.StringSliceTarget
	marshalledPublicKeyTarget vdl.BytesTarget
	vdl.TargetBase
	vdl.FieldsTargetBase
}

func (t *WireValidatorDataTarget) StartFields(tt *vdl.Type) (vdl.FieldsTarget, error) {

	if ttWant := vdl.TypeOf((*WireValidatorData)(nil)).Elem(); !vdl.Compatible(tt, ttWant) {
		return nil, fmt.Errorf("type %v incompatible with %v", tt, ttWant)
	}
	return t, nil
}
func (t *WireValidatorDataTarget) StartField(name string) (key, field vdl.Target, _ error) {
	switch name {
	case "Names":
		t.namesTarget.Value = &t.Value.Names
		target, err := &t.namesTarget, error(nil)
		return nil, target, err
	case "MarshalledPublicKey":
		t.marshalledPublicKeyTarget.Value = &t.Value.MarshalledPublicKey
		target, err := &t.marshalledPublicKeyTarget, error(nil)
		return nil, target, err
	default:
		return nil, nil, fmt.Errorf("field %s not in struct v.io/x/ref/services/syncbase/signing.WireValidatorData", name)
	}
}
func (t *WireValidatorDataTarget) FinishField(_, _ vdl.Target) error {
	return nil
}
func (t *WireValidatorDataTarget) ZeroField(name string) error {
	switch name {
	case "Names":
		t.Value.Names = []string(nil)
		return nil
	case "MarshalledPublicKey":
		t.Value.MarshalledPublicKey = []byte(nil)
		return nil
	default:
		return fmt.Errorf("field %s not in struct v.io/x/ref/services/syncbase/signing.WireValidatorData", name)
	}
}
func (t *WireValidatorDataTarget) FinishFields(_ vdl.FieldsTarget) error {

	return nil
}

func (x *WireValidatorData) VDLRead(dec vdl.Decoder) error {
	*x = WireValidatorData{}
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
		case "Names":
			if err = __VDLRead2_list(dec, &x.Names); err != nil {
				return err
			}
		case "MarshalledPublicKey":
			if err = dec.StartValue(); err != nil {
				return err
			}
			if err = dec.DecodeBytes(-1, &x.MarshalledPublicKey); err != nil {
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

func __VDLRead2_list(dec vdl.Decoder, x *[]string) error {
	var err error
	if err = dec.StartValue(); err != nil {
		return err
	}
	if (dec.StackDepth() == 1 || dec.IsAny()) && !vdl.Compatible(vdl.TypeOf(*x), dec.Type()) {
		return fmt.Errorf("incompatible list %T, from %v", *x, dec.Type())
	}
	switch len := dec.LenHint(); {
	case len > 0:
		*x = make([]string, 0, len)
	default:
		*x = nil
	}
	for {
		switch done, err := dec.NextEntry(); {
		case err != nil:
			return err
		case done:
			return dec.FinishValue()
		}
		var elem string
		if err = dec.StartValue(); err != nil {
			return err
		}
		if elem, err = dec.DecodeString(); err != nil {
			return err
		}
		if err = dec.FinishValue(); err != nil {
			return err
		}
		*x = append(*x, elem)
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
	vdl.Register((*Item)(nil))
	vdl.Register((*DataWithSignature)(nil))
	vdl.Register((*WireValidatorData)(nil))

	return struct{}{}
}
