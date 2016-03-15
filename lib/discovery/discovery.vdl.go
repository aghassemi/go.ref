// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: discovery

package discovery

import (
	"fmt"
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/discovery"
	"v.io/v23/i18n"
	"v.io/v23/rpc"
	"v.io/v23/security/access"
	"v.io/v23/vdl"
	"v.io/v23/vdl/vdlconv"
	"v.io/v23/verror"
)

type EncryptionAlgorithm int32

func (EncryptionAlgorithm) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/discovery.EncryptionAlgorithm"`
}) {
}

func (m *EncryptionAlgorithm) FillVDLTarget(t vdl.Target, tt *vdl.Type) error {
	if err := t.FromInt(int64((*m)), __VDLType_v_io_x_ref_lib_discovery_EncryptionAlgorithm); err != nil {
		return err
	}
	return nil
}

func (m *EncryptionAlgorithm) MakeVDLTarget() vdl.Target {
	return &EncryptionAlgorithmTarget{Value: m}
}

type EncryptionAlgorithmTarget struct {
	Value *EncryptionAlgorithm
	vdl.TargetBase
}

func (t *EncryptionAlgorithmTarget) FromUint(src uint64, tt *vdl.Type) error {
	val, err := vdlconv.Uint64ToInt32(src)
	if err != nil {
		return err
	}
	*t.Value = EncryptionAlgorithm(val)

	return nil
}
func (t *EncryptionAlgorithmTarget) FromInt(src int64, tt *vdl.Type) error {
	val, err := vdlconv.Int64ToInt32(src)
	if err != nil {
		return err
	}
	*t.Value = EncryptionAlgorithm(val)

	return nil
}
func (t *EncryptionAlgorithmTarget) FromFloat(src float64, tt *vdl.Type) error {
	val, err := vdlconv.Float64ToInt32(src)
	if err != nil {
		return err
	}
	*t.Value = EncryptionAlgorithm(val)

	return nil
}
func (t *EncryptionAlgorithmTarget) FromComplex(src complex128, tt *vdl.Type) error {
	val, err := vdlconv.Complex128ToInt32(src)
	if err != nil {
		return err
	}
	*t.Value = EncryptionAlgorithm(val)

	return nil
}

type EncryptionKey []byte

func (EncryptionKey) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/discovery.EncryptionKey"`
}) {
}

func (m *EncryptionKey) FillVDLTarget(t vdl.Target, tt *vdl.Type) error {
	if err := t.FromBytes([]byte((*m)), __VDLType_v_io_x_ref_lib_discovery_EncryptionKey); err != nil {
		return err
	}
	return nil
}

func (m *EncryptionKey) MakeVDLTarget() vdl.Target {
	return &EncryptionKeyTarget{Value: m}
}

type EncryptionKeyTarget struct {
	Value *EncryptionKey
	vdl.TargetBase
}

func (t *EncryptionKeyTarget) FromBytes(src []byte, tt *vdl.Type) error {
	if !vdl.Compatible(tt, __VDLType_v_io_x_ref_lib_discovery_EncryptionKey) {
		return fmt.Errorf("type %v incompatible with %v", tt, __VDLType_v_io_x_ref_lib_discovery_EncryptionKey)
	}
	if len(src) == 0 {
		*t.Value = nil
	} else {
		*t.Value = make([]byte, len(src))
		copy(*t.Value, src)
	}

	return nil
}

type Uuid []byte

func (Uuid) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/discovery.Uuid"`
}) {
}

func (m *Uuid) FillVDLTarget(t vdl.Target, tt *vdl.Type) error {
	if err := t.FromBytes([]byte((*m)), __VDLType_v_io_x_ref_lib_discovery_Uuid); err != nil {
		return err
	}
	return nil
}

func (m *Uuid) MakeVDLTarget() vdl.Target {
	return &UuidTarget{Value: m}
}

type UuidTarget struct {
	Value *Uuid
	vdl.TargetBase
}

func (t *UuidTarget) FromBytes(src []byte, tt *vdl.Type) error {
	if !vdl.Compatible(tt, __VDLType_v_io_x_ref_lib_discovery_Uuid) {
		return fmt.Errorf("type %v incompatible with %v", tt, __VDLType_v_io_x_ref_lib_discovery_Uuid)
	}
	if len(src) == 0 {
		*t.Value = nil
	} else {
		*t.Value = make([]byte, len(src))
		copy(*t.Value, src)
	}

	return nil
}

// AdInfo represents advertisement information for discovery.
type AdInfo struct {
	Ad discovery.Advertisement
	// Type of encryption applied to the advertisement so that it can
	// only be decoded by authorized principals.
	EncryptionAlgorithm EncryptionAlgorithm
	// If the advertisement is encrypted, then the data required to
	// decrypt it. The format of this data is a function of the algorithm.
	EncryptionKeys []EncryptionKey
	// Hash of the current advertisement.
	Hash AdHash
	// The addresses (vanadium object names) that the advertisement directory service
	// is served on. See directory.vdl.
	DirAddrs []string
	// TODO(jhahn): Add proximity.
	// TODO(jhahn): Use proximity for Lost.
	Lost bool
}

func (AdInfo) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/discovery.AdInfo"`
}) {
}

func (m *AdInfo) FillVDLTarget(t vdl.Target, tt *vdl.Type) error {
	if __VDLType_v_io_x_ref_lib_discovery_AdInfo == nil || __VDLType0 == nil {
		panic("Initialization order error: types generated for FillVDLTarget not initialized. Consider moving caller to an init() block.")
	}
	fieldsTarget1, err := t.StartFields(tt)
	if err != nil {
		return err
	}

	keyTarget2, fieldTarget3, err := fieldsTarget1.StartField("Ad")
	if err != vdl.ErrFieldNoExist && err != nil {
		return err
	}
	if err != vdl.ErrFieldNoExist {

		if err := m.Ad.FillVDLTarget(fieldTarget3, __VDLType_v_io_v23_discovery_Advertisement); err != nil {
			return err
		}
		if err := fieldsTarget1.FinishField(keyTarget2, fieldTarget3); err != nil {
			return err
		}
	}
	keyTarget4, fieldTarget5, err := fieldsTarget1.StartField("EncryptionAlgorithm")
	if err != vdl.ErrFieldNoExist && err != nil {
		return err
	}
	if err != vdl.ErrFieldNoExist {

		if err := m.EncryptionAlgorithm.FillVDLTarget(fieldTarget5, __VDLType_v_io_x_ref_lib_discovery_EncryptionAlgorithm); err != nil {
			return err
		}
		if err := fieldsTarget1.FinishField(keyTarget4, fieldTarget5); err != nil {
			return err
		}
	}
	keyTarget6, fieldTarget7, err := fieldsTarget1.StartField("EncryptionKeys")
	if err != vdl.ErrFieldNoExist && err != nil {
		return err
	}
	if err != vdl.ErrFieldNoExist {

		listTarget8, err := fieldTarget7.StartList(__VDLType1, len(m.EncryptionKeys))
		if err != nil {
			return err
		}
		for i, elem10 := range m.EncryptionKeys {
			elemTarget9, err := listTarget8.StartElem(i)
			if err != nil {
				return err
			}

			if err := elem10.FillVDLTarget(elemTarget9, __VDLType_v_io_x_ref_lib_discovery_EncryptionKey); err != nil {
				return err
			}
			if err := listTarget8.FinishElem(elemTarget9); err != nil {
				return err
			}
		}
		if err := fieldTarget7.FinishList(listTarget8); err != nil {
			return err
		}
		if err := fieldsTarget1.FinishField(keyTarget6, fieldTarget7); err != nil {
			return err
		}
	}
	keyTarget11, fieldTarget12, err := fieldsTarget1.StartField("Hash")
	if err != vdl.ErrFieldNoExist && err != nil {
		return err
	}
	if err != vdl.ErrFieldNoExist {

		if err := m.Hash.FillVDLTarget(fieldTarget12, __VDLType_v_io_x_ref_lib_discovery_AdHash); err != nil {
			return err
		}
		if err := fieldsTarget1.FinishField(keyTarget11, fieldTarget12); err != nil {
			return err
		}
	}
	keyTarget13, fieldTarget14, err := fieldsTarget1.StartField("DirAddrs")
	if err != vdl.ErrFieldNoExist && err != nil {
		return err
	}
	if err != vdl.ErrFieldNoExist {

		listTarget15, err := fieldTarget14.StartList(__VDLType2, len(m.DirAddrs))
		if err != nil {
			return err
		}
		for i, elem17 := range m.DirAddrs {
			elemTarget16, err := listTarget15.StartElem(i)
			if err != nil {
				return err
			}
			if err := elemTarget16.FromString(string(elem17), vdl.StringType); err != nil {
				return err
			}
			if err := listTarget15.FinishElem(elemTarget16); err != nil {
				return err
			}
		}
		if err := fieldTarget14.FinishList(listTarget15); err != nil {
			return err
		}
		if err := fieldsTarget1.FinishField(keyTarget13, fieldTarget14); err != nil {
			return err
		}
	}
	keyTarget18, fieldTarget19, err := fieldsTarget1.StartField("Lost")
	if err != vdl.ErrFieldNoExist && err != nil {
		return err
	}
	if err != vdl.ErrFieldNoExist {
		if err := fieldTarget19.FromBool(bool(m.Lost), vdl.BoolType); err != nil {
			return err
		}
		if err := fieldsTarget1.FinishField(keyTarget18, fieldTarget19); err != nil {
			return err
		}
	}
	if err := t.FinishFields(fieldsTarget1); err != nil {
		return err
	}
	return nil
}

func (m *AdInfo) MakeVDLTarget() vdl.Target {
	return &AdInfoTarget{Value: m}
}

type AdInfoTarget struct {
	Value                     *AdInfo
	adTarget                  discovery.AdvertisementTarget
	encryptionAlgorithmTarget EncryptionAlgorithmTarget
	encryptionKeysTarget      unnamed_5b5d762e696f2f782f7265662f6c69622f646973636f766572792e456e6372797074696f6e4b6579205b5d62797465Target
	hashTarget                AdHashTarget
	dirAddrsTarget            vdl.StringSliceTarget
	lostTarget                vdl.BoolTarget
	vdl.TargetBase
	vdl.FieldsTargetBase
}

func (t *AdInfoTarget) StartFields(tt *vdl.Type) (vdl.FieldsTarget, error) {
	if !vdl.Compatible(tt, __VDLType_v_io_x_ref_lib_discovery_AdInfo) {
		return nil, fmt.Errorf("type %v incompatible with %v", tt, __VDLType_v_io_x_ref_lib_discovery_AdInfo)
	}
	return t, nil
}
func (t *AdInfoTarget) StartField(name string) (key, field vdl.Target, _ error) {
	switch name {
	case "Ad":
		t.adTarget.Value = &t.Value.Ad
		target, err := &t.adTarget, error(nil)
		return nil, target, err
	case "EncryptionAlgorithm":
		t.encryptionAlgorithmTarget.Value = &t.Value.EncryptionAlgorithm
		target, err := &t.encryptionAlgorithmTarget, error(nil)
		return nil, target, err
	case "EncryptionKeys":
		t.encryptionKeysTarget.Value = &t.Value.EncryptionKeys
		target, err := &t.encryptionKeysTarget, error(nil)
		return nil, target, err
	case "Hash":
		t.hashTarget.Value = &t.Value.Hash
		target, err := &t.hashTarget, error(nil)
		return nil, target, err
	case "DirAddrs":
		t.dirAddrsTarget.Value = &t.Value.DirAddrs
		target, err := &t.dirAddrsTarget, error(nil)
		return nil, target, err
	case "Lost":
		t.lostTarget.Value = &t.Value.Lost
		target, err := &t.lostTarget, error(nil)
		return nil, target, err
	default:
		return nil, nil, fmt.Errorf("field %s not in struct %v", name, __VDLType_v_io_x_ref_lib_discovery_AdInfo)
	}
}
func (t *AdInfoTarget) FinishField(_, _ vdl.Target) error {
	return nil
}
func (t *AdInfoTarget) FinishFields(_ vdl.FieldsTarget) error {

	return nil
}

// []EncryptionKey
type unnamed_5b5d762e696f2f782f7265662f6c69622f646973636f766572792e456e6372797074696f6e4b6579205b5d62797465Target struct {
	Value      *[]EncryptionKey
	elemTarget EncryptionKeyTarget
	vdl.TargetBase
	vdl.ListTargetBase
}

func (t *unnamed_5b5d762e696f2f782f7265662f6c69622f646973636f766572792e456e6372797074696f6e4b6579205b5d62797465Target) StartList(tt *vdl.Type, len int) (vdl.ListTarget, error) {
	if !vdl.Compatible(tt, __VDLType1) {
		return nil, fmt.Errorf("type %v incompatible with %v", tt, __VDLType1)
	}
	if cap(*t.Value) < len {
		*t.Value = make([]EncryptionKey, len)
	} else {
		*t.Value = (*t.Value)[:len]
	}
	return t, nil
}
func (t *unnamed_5b5d762e696f2f782f7265662f6c69622f646973636f766572792e456e6372797074696f6e4b6579205b5d62797465Target) StartElem(index int) (elem vdl.Target, _ error) {
	t.elemTarget.Value = &(*t.Value)[index]
	target, err := &t.elemTarget, error(nil)
	return target, err
}
func (t *unnamed_5b5d762e696f2f782f7265662f6c69622f646973636f766572792e456e6372797074696f6e4b6579205b5d62797465Target) FinishElem(elem vdl.Target) error {
	return nil
}
func (t *unnamed_5b5d762e696f2f782f7265662f6c69622f646973636f766572792e456e6372797074696f6e4b6579205b5d62797465Target) FinishList(elem vdl.ListTarget) error {

	return nil
}

type AdHashTarget struct {
	Value *AdHash
	vdl.TargetBase
}

func (t *AdHashTarget) FromBytes(src []byte, tt *vdl.Type) error {
	if !vdl.Compatible(tt, __VDLType_v_io_x_ref_lib_discovery_AdHash) {
		return fmt.Errorf("type %v incompatible with %v", tt, __VDLType_v_io_x_ref_lib_discovery_AdHash)
	}
	copy((*t.Value)[:], src)

	return nil
}

// An AdHash is a hash of an advertisement.
type AdHash [8]byte

func (AdHash) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/discovery.AdHash"`
}) {
}

func (m *AdHash) FillVDLTarget(t vdl.Target, tt *vdl.Type) error {
	if err := t.FromBytes([]byte((*m)[:]), __VDLType_v_io_x_ref_lib_discovery_AdHash); err != nil {
		return err
	}
	return nil
}

func (m *AdHash) MakeVDLTarget() vdl.Target {
	return &AdHashTarget{Value: m}
}

func init() {
	vdl.Register((*EncryptionAlgorithm)(nil))
	vdl.Register((*EncryptionKey)(nil))
	vdl.Register((*Uuid)(nil))
	vdl.Register((*AdInfo)(nil))
	vdl.Register((*AdHash)(nil))
}

var __VDLType0 *vdl.Type = vdl.TypeOf((*AdInfo)(nil))
var __VDLType2 *vdl.Type = vdl.TypeOf([]string(nil))
var __VDLType1 *vdl.Type = vdl.TypeOf([]EncryptionKey(nil))
var __VDLType_v_io_v23_discovery_Advertisement *vdl.Type = vdl.TypeOf(discovery.Advertisement{})
var __VDLType_v_io_x_ref_lib_discovery_AdHash *vdl.Type = vdl.TypeOf(AdHash{})
var __VDLType_v_io_x_ref_lib_discovery_AdInfo *vdl.Type = vdl.TypeOf(AdInfo{})
var __VDLType_v_io_x_ref_lib_discovery_EncryptionAlgorithm *vdl.Type = vdl.TypeOf(EncryptionAlgorithm(0))
var __VDLType_v_io_x_ref_lib_discovery_EncryptionKey *vdl.Type = vdl.TypeOf(EncryptionKey(nil))
var __VDLType_v_io_x_ref_lib_discovery_Uuid *vdl.Type = vdl.TypeOf(Uuid(nil))

func __VDLEnsureNativeBuilt() {
}

const NoEncryption = EncryptionAlgorithm(0)

const TestEncryption = EncryptionAlgorithm(1)

const IbeEncryption = EncryptionAlgorithm(2)

var (
	ErrAlreadyBeingAdvertised = verror.Register("v.io/x/ref/lib/discovery.AlreadyBeingAdvertised", verror.NoRetry, "{1:}{2:} already being advertised: {3}")
	ErrBadAdvertisement       = verror.Register("v.io/x/ref/lib/discovery.BadAdvertisement", verror.NoRetry, "{1:}{2:} invalid advertisement: {3}")
	ErrBadQuery               = verror.Register("v.io/x/ref/lib/discovery.BadQuery", verror.NoRetry, "{1:}{2:} invalid query: {3}")
	ErrDiscoveryClosed        = verror.Register("v.io/x/ref/lib/discovery.DiscoveryClosed", verror.NoRetry, "{1:}{2:} discovery closed")
	ErrNoDiscoveryPlugin      = verror.Register("v.io/x/ref/lib/discovery.NoDiscoveryPlugin", verror.NoRetry, "{1:}{2:} no discovery plugin")
)

func init() {
	i18n.Cat().SetWithBase(i18n.LangID("en"), i18n.MsgID(ErrAlreadyBeingAdvertised.ID), "{1:}{2:} already being advertised: {3}")
	i18n.Cat().SetWithBase(i18n.LangID("en"), i18n.MsgID(ErrBadAdvertisement.ID), "{1:}{2:} invalid advertisement: {3}")
	i18n.Cat().SetWithBase(i18n.LangID("en"), i18n.MsgID(ErrBadQuery.ID), "{1:}{2:} invalid query: {3}")
	i18n.Cat().SetWithBase(i18n.LangID("en"), i18n.MsgID(ErrDiscoveryClosed.ID), "{1:}{2:} discovery closed")
	i18n.Cat().SetWithBase(i18n.LangID("en"), i18n.MsgID(ErrNoDiscoveryPlugin.ID), "{1:}{2:} no discovery plugin")
}

// NewErrAlreadyBeingAdvertised returns an error with the ErrAlreadyBeingAdvertised ID.
func NewErrAlreadyBeingAdvertised(ctx *context.T, id discovery.AdId) error {
	return verror.New(ErrAlreadyBeingAdvertised, ctx, id)
}

// NewErrBadAdvertisement returns an error with the ErrBadAdvertisement ID.
func NewErrBadAdvertisement(ctx *context.T, err error) error {
	return verror.New(ErrBadAdvertisement, ctx, err)
}

// NewErrBadQuery returns an error with the ErrBadQuery ID.
func NewErrBadQuery(ctx *context.T, err error) error {
	return verror.New(ErrBadQuery, ctx, err)
}

// NewErrDiscoveryClosed returns an error with the ErrDiscoveryClosed ID.
func NewErrDiscoveryClosed(ctx *context.T) error {
	return verror.New(ErrDiscoveryClosed, ctx)
}

// NewErrNoDiscoveryPlugin returns an error with the ErrNoDiscoveryPlugin ID.
func NewErrNoDiscoveryPlugin(ctx *context.T) error {
	return verror.New(ErrNoDiscoveryPlugin, ctx)
}

// DirectoryClientMethods is the client interface
// containing Directory methods.
//
// Directory is the interface for advertisement directory service.
type DirectoryClientMethods interface {
	// Lookup returns the advertisement of the given service instance.
	Lookup(_ *context.T, id discovery.AdId, _ ...rpc.CallOpt) (AdInfo, error)
}

// DirectoryClientStub adds universal methods to DirectoryClientMethods.
type DirectoryClientStub interface {
	DirectoryClientMethods
	rpc.UniversalServiceMethods
}

// DirectoryClient returns a client stub for Directory.
func DirectoryClient(name string) DirectoryClientStub {
	return implDirectoryClientStub{name}
}

type implDirectoryClientStub struct {
	name string
}

func (c implDirectoryClientStub) Lookup(ctx *context.T, i0 discovery.AdId, opts ...rpc.CallOpt) (o0 AdInfo, err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Lookup", []interface{}{i0}, []interface{}{&o0}, opts...)
	return
}

// DirectoryServerMethods is the interface a server writer
// implements for Directory.
//
// Directory is the interface for advertisement directory service.
type DirectoryServerMethods interface {
	// Lookup returns the advertisement of the given service instance.
	Lookup(_ *context.T, _ rpc.ServerCall, id discovery.AdId) (AdInfo, error)
}

// DirectoryServerStubMethods is the server interface containing
// Directory methods, as expected by rpc.Server.
// There is no difference between this interface and DirectoryServerMethods
// since there are no streaming methods.
type DirectoryServerStubMethods DirectoryServerMethods

// DirectoryServerStub adds universal methods to DirectoryServerStubMethods.
type DirectoryServerStub interface {
	DirectoryServerStubMethods
	// Describe the Directory interfaces.
	Describe__() []rpc.InterfaceDesc
}

// DirectoryServer returns a server stub for Directory.
// It converts an implementation of DirectoryServerMethods into
// an object that may be used by rpc.Server.
func DirectoryServer(impl DirectoryServerMethods) DirectoryServerStub {
	stub := implDirectoryServerStub{
		impl: impl,
	}
	// Initialize GlobState; always check the stub itself first, to handle the
	// case where the user has the Glob method defined in their VDL source.
	if gs := rpc.NewGlobState(stub); gs != nil {
		stub.gs = gs
	} else if gs := rpc.NewGlobState(impl); gs != nil {
		stub.gs = gs
	}
	return stub
}

type implDirectoryServerStub struct {
	impl DirectoryServerMethods
	gs   *rpc.GlobState
}

func (s implDirectoryServerStub) Lookup(ctx *context.T, call rpc.ServerCall, i0 discovery.AdId) (AdInfo, error) {
	return s.impl.Lookup(ctx, call, i0)
}

func (s implDirectoryServerStub) Globber() *rpc.GlobState {
	return s.gs
}

func (s implDirectoryServerStub) Describe__() []rpc.InterfaceDesc {
	return []rpc.InterfaceDesc{DirectoryDesc}
}

// DirectoryDesc describes the Directory interface.
var DirectoryDesc rpc.InterfaceDesc = descDirectory

// descDirectory hides the desc to keep godoc clean.
var descDirectory = rpc.InterfaceDesc{
	Name:    "Directory",
	PkgPath: "v.io/x/ref/lib/discovery",
	Doc:     "// Directory is the interface for advertisement directory service.",
	Methods: []rpc.MethodDesc{
		{
			Name: "Lookup",
			Doc:  "// Lookup returns the advertisement of the given service instance.",
			InArgs: []rpc.ArgDesc{
				{"id", ``}, // discovery.AdId
			},
			OutArgs: []rpc.ArgDesc{
				{"", ``}, // AdInfo
			},
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Read"))},
		},
	},
}
