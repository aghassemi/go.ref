package server

import (
	"v.io/core/veyron2/context"
	"v.io/core/veyron2/ipc"
	"v.io/core/veyron2/naming"
	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/vdl/vdlroot/src/signature"
	"v.io/core/veyron2/verror"
)

var typedNil []int

const pkgPath = "v.io/wspr/veyron/services/wsprd/ipc/server"

// Errors.
var (
	ErrWrongNumberOfArgs         = verror.Register(pkgPath+".ErrWrongNumberOfArgs", verror.NoRetry, "{1:}{2:} Method {3} got {4} args, want {5}{:_}")
	ErrMethodNotFoundInSignature = verror.Register(pkgPath+".ErrMethodNotFoundInSignature", verror.NoRetry, "{1:}{2:} Method {3} not found in signature{:_}")
)

// invoker holds a delegate function to call on invoke and a list of methods that
// are available for be called.
type invoker struct {
	// delegate function to call when an invoke request comes in
	invokeFunc remoteInvokeFunc

	signature []signature.Interface

	globFunc remoteGlobFunc
}

var _ ipc.Invoker = (*invoker)(nil)

// newInvoker is an invoker factory
func newInvoker(signature []signature.Interface, invokeFunc remoteInvokeFunc, globFunc remoteGlobFunc) ipc.Invoker {
	i := &invoker{invokeFunc, signature, globFunc}
	return i
}

// Prepare implements the Invoker interface.
func (i *invoker) Prepare(methodName string, numArgs int) ([]interface{}, []*vdl.Value, error) {
	method, err := i.MethodSignature(nil, methodName)
	if err != nil {
		return nil, nil, err
	}
	if got, want := numArgs, len(method.InArgs); got != want {
		return nil, nil, verror.New(ErrWrongNumberOfArgs, nil, methodName, got, want)
	}
	argptrs := make([]interface{}, len(method.InArgs))
	for ix, arg := range method.InArgs {
		argptrs[ix] = vdl.ZeroValue(arg.Type)
	}
	return argptrs, method.Tags, nil
}

// Invoke implements the Invoker interface.
func (i *invoker) Invoke(methodName string, call ipc.ServerCall, argptrs []interface{}) ([]interface{}, error) {
	replychan := i.invokeFunc(methodName, argptrs, call)

	// Wait for the result
	reply := <-replychan

	if reply.Err != nil {
		return nil, reply.Err
	}

	// Convert the reply.Results from []*vdl.Value to []interface{}
	results := make([]interface{}, len(reply.Results))
	for i, r := range reply.Results {
		results[i] = r
	}
	return results, nil
}

// TODO(bjornick,rthellend): Find a reasonable way to implement this for JS.
func (i *invoker) Globber() *ipc.GlobState {
	if i.globFunc == nil {
		return nil
	}
	return &ipc.GlobState{AllGlobber: i}
}

func (i *invoker) Glob__(ctx ipc.ServerContext, pattern string) (<-chan naming.VDLGlobReply, error) {
	return i.globFunc(pattern, ctx)
}

func (i *invoker) Signature(ctx ipc.ServerContext) ([]signature.Interface, error) {
	return i.signature, nil
}

func (i *invoker) MethodSignature(ctx ipc.ServerContext, method string) (signature.Method, error) {
	if methodSig, ok := signature.FirstMethod(i.signature, method); ok {
		return methodSig, nil
	}

	var innerContext *context.T
	if ctx != nil {
		innerContext = ctx.Context()
	}

	return signature.Method{}, verror.New(ErrMethodNotFoundInSignature, innerContext, method)
}
