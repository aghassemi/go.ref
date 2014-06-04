// +build android

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"veyron2"
	"veyron2/ipc"
)

// #include <stdlib.h>
// #include "jni_wrapper.h"
import "C"

type client struct {
	client ipc.Client
}

func newClient(c ipc.Client) *client {
	return &client{
		client: c,
	}
}

func (c *client) StartCall(env *C.JNIEnv, jContext C.jobject, name, method string, jArgs C.jobjectArray, jPath C.jstring, jTimeout C.jlong) (*clientCall, error) {
	// NOTE(spetrovic): In the long-term, we will decode JSON arguments into an
	// array of vom.Value instances and send this array across the wire.

	// Convert Java argument array into []string.
	argStrs := make([]string, int(C.GetArrayLength(env, C.jarray(jArgs))))
	for i := 0; i < len(argStrs); i++ {
		argStrs[i] = goString(env, C.jstring(C.GetObjectArrayElement(env, jArgs, C.jsize(i))))
	}
	// Get argument instances that correspond to the provided method.
	getter := newArgGetter(strings.Join(strings.Split(goString(env, jPath), ".")[1:], "/"))
	if getter == nil {
		return nil, fmt.Errorf("couldn't find IDL interface corresponding to path %q", goString(env, jPath))
	}
	mArgs := getter.FindMethod(method, len(argStrs))
	if mArgs == nil {
		return nil, fmt.Errorf("couldn't find method %s with %d args in IDL interface at path %q, getter: %v", method, len(argStrs), goString(env, jPath), getter)
	}
	argptrs := mArgs.InPtrs()
	if len(argptrs) != len(argStrs) {
		return nil, fmt.Errorf("invalid number of arguments for method %s, want %d, have %d", method, len(argStrs), len(argptrs))
	}
	// JSON decode.
	args := make([]interface{}, len(argptrs))
	for i, argStr := range argStrs {
		if err := json.Unmarshal([]byte(argStr), argptrs[i]); err != nil {
			return nil, err
		}
		// Remove the pointer from the argument.  Simply *argptr[i] doesn't work
		// as argptr[i] is of type interface{}.
		args[i] = derefOrDie(argptrs[i])
	}
	// Process options.
	options := []ipc.CallOpt{}
	if int(jTimeout) >= 0 {
		options = append(options, veyron2.CallTimeout(time.Duration(int(jTimeout))*time.Millisecond))
	}
	context, err := newContext(env, jContext)
	if err != nil {
		return nil, err
	}
	// Invoke StartCall
	call, err := c.client.StartCall(context, name, method, args, options...)
	if err != nil {
		return nil, err
	}
	return &clientCall{
		stream: newStream(call, mArgs),
		call:   call,
	}, nil
}

func (c *client) Close() {
	c.client.Close()
}

type clientCall struct {
	stream
	call ipc.Call
}

func (c *clientCall) Finish(env *C.JNIEnv) (C.jobjectArray, error) {
	var resultptrs []interface{}
	if c.mArgs.IsStreaming() {
		resultptrs = c.mArgs.StreamFinishPtrs()
	} else {
		resultptrs = c.mArgs.OutPtrs()
	}
	// argGetter doesn't store the (mandatory) error result, so we add it here.
	var appErr error
	if err := c.call.Finish(append(resultptrs, &appErr)...); err != nil {
		// invocation error
		return nil, fmt.Errorf("Invocation error: %v", err)
	}
	if appErr != nil { // application error
		return nil, appErr
	}
	// JSON encode the results.
	jsonResults := make([][]byte, len(resultptrs))
	for i, resultptr := range resultptrs {
		// Remove the pointer from the result.  Simply *resultptr doesn't work
		// as resultptr is of type interface{}.
		result := derefOrDie(resultptr)
		var err error
		jsonResults[i], err = json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("error marshalling %q into JSON", resultptr)
		}
	}
	// Convert to Java array of C.jstring.
	ret := C.NewObjectArray(env, C.jsize(len(jsonResults)), jStringClass, nil)
	for i, result := range jsonResults {
		C.SetObjectArrayElement(env, ret, C.jsize(i), C.jobject(jString(env, string(result))))
	}
	return ret, nil
}

func (c *clientCall) Cancel() {
	c.call.Cancel()
}
