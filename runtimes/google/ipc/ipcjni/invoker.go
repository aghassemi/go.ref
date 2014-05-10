// +build android

package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"unsafe"

	"veyron2/ipc"
	"veyron2/security"
	"veyron2/verror"
)

// #cgo LDFLAGS: -llog
// #include <stdlib.h>
// #include <android/log.h>
// #include "jni_wrapper.h"
//
// // CGO doesn't support variadic functions so we have to hard-code these
// // functions to match the invoking code. Ugh!
// static jobject CallInvokeMethod(JNIEnv* env, jobject obj, jmethodID id, jstring method, jobject call, jobjectArray inArgs) {
//   return (*env)->CallObjectMethod(env, obj, id, method, call, inArgs);
// }
// static jobject CallNewInvokerObject(JNIEnv* env, jclass class, jmethodID id, jobject obj) {
//   return (*env)->NewObject(env, class, id, obj);
// }
// static jobject CallGetInterfacePath(JNIEnv* env, jobject obj, jmethodID id) {
//   return (*env)->CallObjectMethod(env, obj, id);
// }
import "C"

func newJNIInvoker(env *C.JNIEnv, jVM *C.JavaVM, jObj C.jobject) (ipc.Invoker, error) {
	// Create a new Java IDLInvoker object.
	cid := jMethodID(env, jIDLInvokerClass, "<init>", fmt.Sprintf("(%s)%s", objectSign, voidSign))
	jInvoker := C.CallNewInvokerObject(env, jIDLInvokerClass, cid, jObj)
	if err := jExceptionMsg(env); err != nil {
		return nil, fmt.Errorf("error creating Java IDLInvoker object: %v", err)
	}
	// Fetch the argGetter for the object.
	pid := jMethodID(env, jIDLInvokerClass, "getInterfacePath", fmt.Sprintf("()%s", stringSign))
	jPath := C.jstring(C.CallGetInterfacePath(env, jInvoker, pid))
	getter := newArgGetter(goString(env, jPath))
	if getter == nil {
		return nil, fmt.Errorf("couldn't find IDL interface corresponding to path %q", goString(env, jPath))
	}
	// Reference Java invoker; it will be de-referenced when the go invoker
	// created below is garbage-collected (through the finalizer callback we
	// also setup below).
	jInvoker = C.NewGlobalRef(env, jInvoker)
	i := &jniInvoker{
		jVM:       jVM,
		jInvoker:  jInvoker,
		argGetter: getter,
	}
	runtime.SetFinalizer(i, func(i *jniInvoker) {
		var env *C.JNIEnv
		C.AttachCurrentThread(i.jVM, &env, nil)
		defer C.DetachCurrentThread(i.jVM)
		C.DeleteGlobalRef(env, i.jInvoker)
	})
	return i, nil
}

type jniInvoker struct {
	jVM       *C.JavaVM
	jInvoker  C.jobject
	argGetter *argGetter
}

func (i *jniInvoker) Prepare(method string, numArgs int) (argptrs []interface{}, label security.Label, err error) {
	// NOTE(spetrovic): In the long-term, this method will return an array of
	// []vom.Value.  This will in turn result in VOM decoding all input
	// arguments into vom.Value objects, which we shall then de-serialize into
	// Java objects (see Invoke comments below).  This approach is blocked on
	// pending VOM encoder/decoder changes as well as Java (de)serializer.
	argptrs, err = i.argGetter.GetInArgPtrs(method, numArgs)
	return
}

func (i *jniInvoker) Invoke(method string, call ipc.ServerCall, argptrs []interface{}) (results []interface{}, err error) {
	// NOTE(spetrovic): In the long-term, all input arguments will be of
	// vom.Value type (see comments for Prepare() method above).  Through JNI,
	// we will call Java functions that transform a serialized vom.Value into
	// Java objects. We will then pass those Java objects to Java's Invoke
	// method.  The returned Java objects will be converted into serialized
	// vom.Values, which will then be returned.  This approach is blocked on VOM
	// encoder/decoder changes as well as Java's (de)serializer.
	var env *C.JNIEnv
	C.AttachCurrentThread(i.jVM, &env, nil)
	defer C.DetachCurrentThread(i.jVM)

	// Translate input args to JSON.
	jArgs, err := i.encodeArgs(env, argptrs)
	if err != nil {
		return
	}
	// Invoke the method.
	const callSign = "Lcom/veyron2/ipc/ServerCall;"
	const replySign = "Lcom/veyron/runtimes/google/ipc/IDLInvoker$InvokeReply;"
	mid := jMethodID(env, C.GetObjectClass(env, i.jInvoker), "invoke", fmt.Sprintf("(%s%s[%s)%s", stringSign, callSign, stringSign, replySign))
	jReply := C.CallInvokeMethod(env, i.jInvoker, mid, jString(env, camelCase(method)), nil, jArgs)
	if err := jExceptionMsg(env); err != nil {
		return nil, fmt.Errorf("error invoking Java method %q: %v", method, err)
	}
	// Decode and return results.
	return i.decodeResults(env, method, len(argptrs), jReply)
}

// encodeArgs JSON-encodes the provided argument pointers, converts them into
// Java strings, and returns a Java string array response.
func (*jniInvoker) encodeArgs(env *C.JNIEnv, argptrs []interface{}) (C.jobjectArray, error) {
	// JSON encode.
	jsonArgs := make([][]byte, len(argptrs))
	for i, argptr := range argptrs {
		// Remove the pointer from the argument.  Simply *argptr doesn't work
		// as argptr is of type interface{}.
		arg := reflect.ValueOf(argptr).Elem().Interface()
		var err error
		jsonArgs[i], err = json.Marshal(arg)
		if err != nil {
			return nil, fmt.Errorf("error marshalling %q into JSON", arg)
		}
	}

	// Convert to Java array of C.jstring.
	ret := C.NewObjectArray(env, C.jsize(len(argptrs)), jStringClass, nil)
	for i, arg := range jsonArgs {
		C.SetObjectArrayElement(env, ret, C.jsize(i), C.jobject(jString(env, string(arg))))
	}
	return ret, nil
}

// decodeResults JSON-decodes replies stored in the Java reply object and
// returns an array of Go reply objects.
func (i *jniInvoker) decodeResults(env *C.JNIEnv, method string, numArgs int, jReply C.jobject) ([]interface{}, error) {
	// Unpack the replies.
	results := jStringArrayField(env, jReply, "results")
	hasAppErr := jBoolField(env, jReply, "hasApplicationError")
	errorID := jStringField(env, jReply, "errorID")
	errorMsg := jStringField(env, jReply, "errorMsg")

	// Get Go result instances.
	ret, err := i.argGetter.GetOutArgs(method, numArgs)
	if err != nil {
		return nil, fmt.Errorf("couldn't get arguments for method %q with %d input args: %v", method, numArgs, err)
	}
	// Check for app error.
	if hasAppErr {
		// Last return argument must be an app error so append it here.
		return append(ret, verror.Make(verror.ID(errorID), errorMsg)), nil
	}
	// JSON-decode.
	if len(results) != len(ret) {
		return nil, fmt.Errorf("mismatch in number of output arguments, have: %d want: %d", len(results), len(ret))
	}
	for i, result := range results {
		if err := json.Unmarshal([]byte(result), &ret[i]); err != nil {
			return nil, err
		}
	}
	// Last return argument must be an app error, so append it (i.e., nil).
	var appErr error
	return append(ret, appErr), nil
}
