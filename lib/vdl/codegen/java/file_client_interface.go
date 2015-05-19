// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"bytes"
	"fmt"
	"log"
	"path"

	"v.io/x/ref/lib/vdl/compile"
	"v.io/x/ref/lib/vdl/vdlutil"
)

const clientInterfaceTmpl = header + `
// Source: {{ .Source }}
package {{ .PackagePath }};

{{ .ServiceDoc }}
public interface {{ .ServiceName }}Client {{ .Extends }} {
{{ range $method := .Methods }}
    {{/* If this method has multiple return arguments, generate the class. */}}
    {{ if $method.IsMultipleRet }}
    /**
     * Multi-return value for method {@link #{{$method.Name}}}.
     */
    @io.v.v23.vdl.MultiReturn
    public static class {{ $method.UppercaseMethodName }}Out {
        {{ range $retArg := $method.RetArgs }}
        public {{ $retArg.Type }} {{ $retArg.Name }};
        {{ end }}
    }
    {{ end }}

    {{/* Generate the method signature. */}}
    {{ $method.Doc }}
    {{ $method.RetType }} {{ $method.Name }}(io.v.v23.context.VContext context{{ $method.Args }}) throws io.v.v23.verror.VException;
    {{ $method.RetType }} {{ $method.Name }}(io.v.v23.context.VContext context{{ $method.Args }}, io.v.v23.Options vOpts) throws io.v.v23.verror.VException;
{{ end }}
}
`

type clientInterfaceArg struct {
	Type string
	Name string
}

type clientInterfaceMethod struct {
	Args                string
	Doc                 string
	Name                string
	IsMultipleRet       bool
	RetArgs             []clientInterfaceArg
	RetType             string
	UppercaseMethodName string
}

func clientInterfaceNonStreamingOutArg(iface *compile.Interface, method *compile.Method, useClass bool, env *compile.Env) string {
	switch len(method.OutArgs) {
	case 0:
		// "void" or "Void"
		return javaType(nil, useClass, env)
	case 1:
		return javaType(method.OutArgs[0].Type, useClass, env)
	default:
		return javaPath(path.Join(interfaceFullyQualifiedName(iface)+"Client", method.Name+"Out"))
	}
}

func clientInterfaceOutArg(iface *compile.Interface, method *compile.Method, env *compile.Env) string {
	if isStreamingMethod(method) {
		return fmt.Sprintf("io.v.v23.vdl.TypedClientStream<%s, %s, %s>", javaType(method.InStream, true, env), javaType(method.OutStream, true, env), clientInterfaceNonStreamingOutArg(iface, method, true, env))
	}
	return clientInterfaceNonStreamingOutArg(iface, method, false, env)
}

func processClientInterfaceMethod(iface *compile.Interface, method *compile.Method, env *compile.Env) clientInterfaceMethod {
	retArgs := make([]clientInterfaceArg, len(method.OutArgs))
	for i := 0; i < len(method.OutArgs); i++ {
		if method.OutArgs[i].Name != "" {
			retArgs[i].Name = vdlutil.FirstRuneToLower(method.OutArgs[i].Name)
		} else {
			retArgs[i].Name = fmt.Sprintf("ret%d", i+1)
		}
		retArgs[i].Type = javaType(method.OutArgs[i].Type, false, env)
	}
	return clientInterfaceMethod{
		Args:                javaDeclarationArgStr(method.InArgs, env, true),
		Doc:                 javaDoc(method.Doc, method.DocSuffix),
		Name:                vdlutil.FirstRuneToLower(method.Name),
		IsMultipleRet:       len(retArgs) > 1,
		RetArgs:             retArgs,
		RetType:             clientInterfaceOutArg(iface, method, env),
		UppercaseMethodName: method.Name,
	}
}

// genJavaClientInterfaceFile generates the Java interface file for the provided
// interface.
func genJavaClientInterfaceFile(iface *compile.Interface, env *compile.Env) JavaFileInfo {
	javaServiceName := vdlutil.FirstRuneToUpper(iface.Name)
	methods := make([]clientInterfaceMethod, len(iface.Methods))
	for i, method := range iface.Methods {
		methods[i] = processClientInterfaceMethod(iface, method, env)
	}
	data := struct {
		FileDoc     string
		Extends     string
		Methods     []clientInterfaceMethod
		PackagePath string
		ServiceDoc  string
		ServiceName string
		Source      string
	}{
		FileDoc:     iface.File.Package.FileDoc,
		Extends:     javaClientExtendsStr(iface.Embeds),
		Methods:     methods,
		PackagePath: javaPath(javaGenPkgPath(iface.File.Package.GenPath)),
		ServiceDoc:  javaDoc(iface.Doc, iface.DocSuffix),
		ServiceName: javaServiceName,
		Source:      iface.File.BaseName,
	}
	var buf bytes.Buffer
	err := parseTmpl("client interface", clientInterfaceTmpl).Execute(&buf, data)
	if err != nil {
		log.Fatalf("vdl: couldn't execute struct template: %v", err)
	}
	return JavaFileInfo{
		Name: javaServiceName + "Client.java",
		Data: buf.Bytes(),
	}
}
