// This file was auto-generated by the veyron idl tool.
// Source: application.idl

// Package application contains implementation of the interface for
// serving application metadata.
package application

import (
	"veyron2/security"

	"veyron2/services/mgmt/application"

	// The non-user imports are prefixed with "_gen_" to prevent collisions.
	_gen_veyron2 "veyron2"
	_gen_idl "veyron2/idl"
	_gen_ipc "veyron2/ipc"
	_gen_naming "veyron2/naming"
	_gen_rt "veyron2/rt"
	_gen_wiretype "veyron2/wiretype"
)

// Repository describes an application repository internally. Besides
// the public Repository interface, it allows to manage the actual
// application metadata.
// Repository is the interface the client binds and uses.
// Repository_InternalNoTagGetter is the interface without the TagGetter
// and UnresolveStep methods (both framework-added, rathern than user-defined),
// to enable embedding without method collisions.  Not to be used directly by
// clients.
type Repository_InternalNoTagGetter interface {
	application.Repository_InternalNoTagGetter

	// Put adds the given tuple of application version (specified
	// through the veyron name suffix) and application envelope to all
	// of the given application profiles.
	Put(Profiles []string, Envelope application.Envelope, opts ..._gen_ipc.ClientCallOpt) (err error)

	// Remove removes the application envelope for the given profile
	// name and application version (specified through the veyron name
	// suffix). If no version is specified as part of the suffix, the
	// method removes all versions for the given profile.
	//
	// TODO(jsimsa): Add support for using "*" to specify all profiles
	// when Matt implements Globing (or Ken implements querying).
	Remove(Profile string, opts ..._gen_ipc.ClientCallOpt) (err error)
}
type Repository interface {
	_gen_idl.TagGetter
	// UnresolveStep returns the names for the remote service, rooted at the
	// service's immediate namespace ancestor.
	UnresolveStep(opts ..._gen_ipc.ClientCallOpt) ([]string, error)
	Repository_InternalNoTagGetter
}

// RepositoryService is the interface the server implements.
type RepositoryService interface {
	application.RepositoryService

	// Put adds the given tuple of application version (specified
	// through the veyron name suffix) and application envelope to all
	// of the given application profiles.
	Put(context _gen_ipc.Context, Profiles []string, Envelope application.Envelope) (err error)

	// Remove removes the application envelope for the given profile
	// name and application version (specified through the veyron name
	// suffix). If no version is specified as part of the suffix, the
	// method removes all versions for the given profile.
	//
	// TODO(jsimsa): Add support for using "*" to specify all profiles
	// when Matt implements Globing (or Ken implements querying).
	Remove(context _gen_ipc.Context, Profile string) (err error)
}

// BindRepository returns the client stub implementing the Repository
// interface.
//
// If no _gen_ipc.Client is specified, the default _gen_ipc.Client in the
// global Runtime is used.
func BindRepository(name string, opts ..._gen_ipc.BindOpt) (Repository, error) {
	var client _gen_ipc.Client
	switch len(opts) {
	case 0:
		client = _gen_rt.R().Client()
	case 1:
		switch o := opts[0].(type) {
		case _gen_veyron2.Runtime:
			client = o.Client()
		case _gen_ipc.Client:
			client = o
		default:
			return nil, _gen_idl.ErrUnrecognizedOption
		}
	default:
		return nil, _gen_idl.ErrTooManyOptionsToBind
	}
	stub := &clientStubRepository{client: client, name: name}
	stub.Repository_InternalNoTagGetter, _ = application.BindRepository(name, client)

	return stub, nil
}

// NewServerRepository creates a new server stub.
//
// It takes a regular server implementing the RepositoryService
// interface, and returns a new server stub.
func NewServerRepository(server RepositoryService) interface{} {
	return &ServerStubRepository{
		ServerStubRepository: *application.NewServerRepository(server).(*application.ServerStubRepository),
		service:              server,
	}
}

// clientStubRepository implements Repository.
type clientStubRepository struct {
	application.Repository_InternalNoTagGetter

	client _gen_ipc.Client
	name   string
}

func (c *clientStubRepository) GetMethodTags(method string) []interface{} {
	return GetRepositoryMethodTags(method)
}

func (__gen_c *clientStubRepository) Put(Profiles []string, Envelope application.Envelope, opts ..._gen_ipc.ClientCallOpt) (err error) {
	var call _gen_ipc.ClientCall
	if call, err = __gen_c.client.StartCall(__gen_c.name, "Put", []interface{}{Profiles, Envelope}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubRepository) Remove(Profile string, opts ..._gen_ipc.ClientCallOpt) (err error) {
	var call _gen_ipc.ClientCall
	if call, err = __gen_c.client.StartCall(__gen_c.name, "Remove", []interface{}{Profile}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&err); ierr != nil {
		err = ierr
	}
	return
}

func (c *clientStubRepository) UnresolveStep(opts ..._gen_ipc.ClientCallOpt) (reply []string, err error) {
	var call _gen_ipc.ClientCall
	if call, err = c.client.StartCall(c.name, "UnresolveStep", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

// ServerStubRepository wraps a server that implements
// RepositoryService and provides an object that satisfies
// the requirements of veyron2/ipc.ReflectInvoker.
type ServerStubRepository struct {
	application.ServerStubRepository

	service RepositoryService
}

func (s *ServerStubRepository) GetMethodTags(method string) []interface{} {
	return GetRepositoryMethodTags(method)
}

func (s *ServerStubRepository) Signature(call _gen_ipc.ServerCall) (_gen_ipc.ServiceSignature, error) {
	result := _gen_ipc.ServiceSignature{Methods: make(map[string]_gen_ipc.MethodSignature)}
	result.Methods["Put"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "Profiles", Type: 61},
			{Name: "Envelope", Type: 65},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "", Type: 66},
		},
	}
	result.Methods["Remove"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "Profile", Type: 3},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "", Type: 66},
		},
	}

	result.TypeDefs = []_gen_idl.AnyData{
		_gen_wiretype.StructType{
			[]_gen_wiretype.FieldType{
				_gen_wiretype.FieldType{Type: 0x3d, Name: "Args"},
				_gen_wiretype.FieldType{Type: 0x3, Name: "Binary"},
				_gen_wiretype.FieldType{Type: 0x3d, Name: "Env"},
			},
			"public.Envelope", []string(nil)},
		_gen_wiretype.NamedPrimitiveType{Type: 0x1, Name: "error", Tags: []string(nil)}}
	var ss _gen_ipc.ServiceSignature
	var firstAdded int
	ss, _ = s.ServerStubRepository.Signature(call)
	firstAdded = len(result.TypeDefs)
	for k, v := range ss.Methods {
		for i, _ := range v.InArgs {
			if v.InArgs[i].Type >= _gen_wiretype.TypeIDFirst {
				v.InArgs[i].Type += _gen_wiretype.TypeID(firstAdded)
			}
		}
		for i, _ := range v.OutArgs {
			if v.OutArgs[i].Type >= _gen_wiretype.TypeIDFirst {
				v.OutArgs[i].Type += _gen_wiretype.TypeID(firstAdded)
			}
		}
		if v.InStream >= _gen_wiretype.TypeIDFirst {
			v.InStream += _gen_wiretype.TypeID(firstAdded)
		}
		if v.OutStream >= _gen_wiretype.TypeIDFirst {
			v.OutStream += _gen_wiretype.TypeID(firstAdded)
		}
		result.Methods[k] = v
	}
	//TODO(bprosnitz) combine type definitions from embeded interfaces in a way that doesn't cause duplication.
	for _, d := range ss.TypeDefs {
		switch wt := d.(type) {
		case _gen_wiretype.SliceType:
			if wt.Elem >= _gen_wiretype.TypeIDFirst {
				wt.Elem += _gen_wiretype.TypeID(firstAdded)
			}
			d = wt
		case _gen_wiretype.ArrayType:
			if wt.Elem >= _gen_wiretype.TypeIDFirst {
				wt.Elem += _gen_wiretype.TypeID(firstAdded)
			}
			d = wt
		case _gen_wiretype.MapType:
			if wt.Key >= _gen_wiretype.TypeIDFirst {
				wt.Key += _gen_wiretype.TypeID(firstAdded)
			}
			if wt.Elem >= _gen_wiretype.TypeIDFirst {
				wt.Elem += _gen_wiretype.TypeID(firstAdded)
			}
			d = wt
		case _gen_wiretype.StructType:
			for _, fld := range wt.Fields {
				if fld.Type >= _gen_wiretype.TypeIDFirst {
					fld.Type += _gen_wiretype.TypeID(firstAdded)
				}
			}
			d = wt
		}
		result.TypeDefs = append(result.TypeDefs, d)
	}

	return result, nil
}

func (s *ServerStubRepository) UnresolveStep(call _gen_ipc.ServerCall) (reply []string, err error) {
	if unresolver, ok := s.service.(_gen_ipc.Unresolver); ok {
		return unresolver.UnresolveStep(call)
	}
	if call.Server() == nil {
		return
	}
	var published []string
	if published, err = call.Server().Published(); err != nil || published == nil {
		return
	}
	reply = make([]string, len(published))
	for i, p := range published {
		reply[i] = _gen_naming.Join(p, call.Name())
	}
	return
}

func (__gen_s *ServerStubRepository) Put(call _gen_ipc.ServerCall, Profiles []string, Envelope application.Envelope) (err error) {
	err = __gen_s.service.Put(call, Profiles, Envelope)
	return
}

func (__gen_s *ServerStubRepository) Remove(call _gen_ipc.ServerCall, Profile string) (err error) {
	err = __gen_s.service.Remove(call, Profile)
	return
}

func GetRepositoryMethodTags(method string) []interface{} {
	if resp := application.GetRepositoryMethodTags(method); resp != nil {
		return resp
	}
	switch method {
	case "Put":
		return []interface{}{security.Label(2)}
	case "Remove":
		return []interface{}{security.Label(2)}
	default:
		return nil
	}
}
