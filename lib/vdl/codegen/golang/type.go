// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package golang

import (
	"fmt"
	"strconv"
	"strings"

	"v.io/v23/vdl"
	"v.io/v23/vdlroot/vdltool"
	"v.io/x/ref/lib/vdl/compile"
)

func localIdent(data goData, file *compile.File, ident string) string {
	if testingMode {
		return ident
	}
	return data.Pkg(file.Package.GenPath) + ident
}

func nativeIdent(data goData, native vdltool.GoType, wirePkg *compile.Package) string {
	ident := native.Type
	for _, imp := range native.Imports {
		// Translate the packages specified in the native type into local package
		// identifiers.  E.g. if the native type is "foo.Type" with import
		// "path/foo", we need replace "foo." in the native type with the local
		// package identifier for "path/foo".
		pkg := data.Pkg(imp.Path)
		ident = strings.Replace(ident, imp.Name+".", pkg, -1)
	}
	data.AddForcedPkg(wirePkg.GenPath)
	return ident
}

func packageIdent(file *compile.File, ident string) string {
	if testingMode {
		return ident
	}
	return file.Package.Name + "." + ident
}

func qualifiedIdent(file *compile.File, ident string) string {
	if testingMode {
		return ident
	}
	return file.Package.QualifiedName(ident)
}

// typeGo translates vdl.Type into a Go type.
func typeGo(data goData, t *vdl.Type) string {
	if testingMode {
		if t.Name() != "" {
			return t.Name()
		}
	}
	// Terminate recursion at defined types, which include both user-defined types
	// (enum, struct, union) and built-in types.
	if def := data.Env.FindTypeDef(t); def != nil {
		switch {
		case t == vdl.AnyType:
			if shouldUseVdlValueForAny(data.File.Package) {
				return "*" + data.Pkg("v.io/v23/vdl") + "Value"
			} else {
				return "*" + data.Pkg("v.io/v23/vom") + "RawBytes"
			}
		case t == vdl.TypeObjectType:
			return "*" + data.Pkg("v.io/v23/vdl") + "Type"
		case def.File == compile.BuiltInFile:
			// Built-in primitives just use their name.
			return def.Name
		}
		pkg := def.File.Package
		if native, ok := pkg.Config.Go.WireToNativeTypes[def.Name]; ok {
			// There is a Go native type configured for this defined type.
			return nativeIdent(data, native, pkg)
		}
		return localIdent(data, def.File, def.Name)
	}
	// Otherwise recurse through the type.
	switch t.Kind() {
	case vdl.Optional:
		return "*" + typeGo(data, t.Elem())
	case vdl.Array:
		return "[" + strconv.Itoa(t.Len()) + "]" + typeGo(data, t.Elem())
	case vdl.List:
		return "[]" + typeGo(data, t.Elem())
	case vdl.Set:
		return "map[" + typeGo(data, t.Key()) + "]struct{}"
	case vdl.Map:
		return "map[" + typeGo(data, t.Key()) + "]" + typeGo(data, t.Elem())
	default:
		panic(fmt.Errorf("vdl: typeGo unhandled type %v %v", t.Kind(), t))
	}
}

func shouldUseVdlValueForAny(pkg *compile.Package) bool {
	// The vdl package uses vdl.Value due to an import cycle:  vom imports vdl
	if pkg.Path == "v.io/v23/vdl" {
		return true
	}
	// The signature package uses vdl.Value for two reasons:
	// - an import cycle (vom imports vdlroot imports signature) to register
	// important types
	// - any is only used in the signature package for method tags, and these
	// will likely be used in a reflective manner anyways (though interface{}
	// may be a good alternative for this case)
	if pkg.Path == "signature" {
		return true
	}
	// The testdata package uses vdl.Value due to an import cycle:
	// vom imports testdata/*
	if strings.HasPrefix(pkg.Path, "v.io/v23/vom/testdata") {
		return true
	}
	return false
}

// typeDefGo prints the type definition for a type.
func typeDefGo(data goData, def *compile.TypeDef) string {
	s := fmt.Sprintf("%stype %s ", def.Doc, def.Name)
	switch t := def.Type; t.Kind() {
	case vdl.Enum:
		s += fmt.Sprintf("int%s\nconst (", def.DocSuffix)
		for ix := 0; ix < t.NumEnumLabel(); ix++ {
			s += fmt.Sprintf("\n\t%s%s%s", def.LabelDoc[ix], def.Name, t.EnumLabel(ix))
			if ix == 0 {
				s += fmt.Sprintf(" %s = iota", def.Name)
			}
			s += def.LabelDocSuffix[ix]
		}
		s += fmt.Sprintf("\n)"+
			"\n\n// %[1]sAll holds all labels for %[1]s."+
			"\nvar %[1]sAll = [...]%[1]s{%[2]s}"+
			"\n\n// %[1]sFromString creates a %[1]s from a string label."+
			"\nfunc %[1]sFromString(label string) (x %[1]s, err error) {"+
			"\n\terr = x.Set(label)"+
			"\n\treturn"+
			"\n}"+
			"\n\n// Set assigns label to x."+
			"\nfunc (x *%[1]s) Set(label string) error {"+
			"\n\tswitch label {",
			def.Name,
			commaEnumLabels(def.Name, t))
		for ix := 0; ix < t.NumEnumLabel(); ix++ {
			s += fmt.Sprintf("\n\tcase %[2]q, %[3]q:"+
				"\n\t\t*x = %[1]s%[2]s"+
				"\n\t\treturn nil", def.Name, t.EnumLabel(ix), strings.ToLower(t.EnumLabel(ix)))
		}
		s += fmt.Sprintf("\n\t}"+
			"\n\t*x = -1"+
			"\n\treturn "+data.Pkg("fmt")+"Errorf(\"unknown label %%q in %[2]s\", label)"+
			"\n}"+
			"\n\n// String returns the string label of x."+
			"\nfunc (x %[1]s) String() string {"+
			"\n\tswitch x {", def.Name, packageIdent(def.File, def.Name))
		for ix := 0; ix < t.NumEnumLabel(); ix++ {
			s += fmt.Sprintf("\n\tcase %[1]s%[2]s:"+
				"\n\t\treturn %[2]q", def.Name, t.EnumLabel(ix))
		}
		s += fmt.Sprintf("\n\t}"+
			"\n\treturn \"\""+
			"\n}"+
			"\n\nfunc (%[1]s) __VDLReflect(struct{"+
			"\n\tName string `vdl:%[3]q`"+
			"\n\tEnum struct{ %[2]s string }"+
			"\n}) {"+
			"\n}",
			def.Name, commaEnumLabels("", t), qualifiedIdent(def.File, def.Name))
		s += fmt.Sprintf("\n"+
			"\nfunc (m *%[1]s) FillVDLTarget(t %[2]sTarget, tt *%[2]sType) error {", def.Name, data.Pkg("v.io/v23/vdl")) +
			genEncDef(data, def.Type, "") +
			"\n\treturn nil" +
			"\n}"
	case vdl.Struct:
		s += "struct {"
		for ix := 0; ix < t.NumField(); ix++ {
			f := t.Field(ix)
			s += "\n\t" + def.FieldDoc[ix] + f.Name + " "
			s += typeGo(data, f.Type) + def.FieldDocSuffix[ix]
		}
		s += "\n}" + def.DocSuffix
		s += fmt.Sprintf("\n"+
			"\nfunc (%[1]s) __VDLReflect(struct{"+
			"\n\tName string `vdl:%[2]q`"+
			"\n}) {"+
			"\n}",
			def.Name, qualifiedIdent(def.File, def.Name))
		s += fmt.Sprintf("\n"+
			"\nfunc (m *%[1]s) FillVDLTarget(t %[2]sTarget, tt *%[2]sType) error {", def.Name, data.Pkg("v.io/v23/vdl")) +
			genEncDef(data, def.Type, "") +
			"\n\treturn nil" +
			"\n}"
	case vdl.Union:
		s = fmt.Sprintf("type ("+
			"\n\t// %[1]s represents any single field of the %[1]s union type."+
			"\n\t%[2]s%[1]s interface {"+
			"\n\t\t// Index returns the field index."+
			"\n\t\tIndex() int"+
			"\n\t\t// Interface returns the field value as an interface."+
			"\n\t\tInterface() interface{}"+
			"\n\t\t// Name returns the field name."+
			"\n\t\tName() string"+
			"\n\t\t// __VDLReflect describes the %[1]s union type."+
			"\n\t\t__VDLReflect(__%[1]sReflect)"+
			"\n\t\tFillVDLTarget(%[4]sTarget, *%[4]sType) error"+
			"\n\t}%[3]s", def.Name, docBreak(def.Doc), def.DocSuffix, data.Pkg("v.io/v23/vdl"))
		for ix := 0; ix < t.NumField(); ix++ {
			f := t.Field(ix)
			s += fmt.Sprintf("\n\t// %[1]s%[2]s represents field %[2]s of the %[1]s union type."+
				"\n\t%[4]s%[1]s%[2]s struct{ Value %[3]s }%[5]s",
				def.Name, f.Name, typeGo(data, f.Type),
				docBreak(def.FieldDoc[ix]), def.FieldDocSuffix[ix])
		}
		s += fmt.Sprintf("\n\t// __%[1]sReflect describes the %[1]s union type."+
			"\n\t__%[1]sReflect struct {"+
			"\n\t\tName string `vdl:%[2]q`"+
			"\n\t\tType %[1]s"+
			"\n\t\tUnion struct {", def.Name, qualifiedIdent(def.File, def.Name))
		for ix := 0; ix < t.NumField(); ix++ {
			s += fmt.Sprintf("\n\t\t\t%[2]s %[1]s%[2]s", def.Name, t.Field(ix).Name)
		}
		s += fmt.Sprintf("\n\t\t}\n\t}\n)")
		for ix := 0; ix < t.NumField(); ix++ {
			f := t.Field(ix)
			s += fmt.Sprintf("\n\nfunc (x %[1]s%[2]s) Index() int { return %[3]d }"+
				"\nfunc (x %[1]s%[2]s) Interface() interface{} { return x.Value }"+
				"\nfunc (x %[1]s%[2]s) Name() string { return \"%[2]s\" }"+
				"\nfunc (x %[1]s%[2]s) __VDLReflect(__%[1]sReflect) {}",
				def.Name, f.Name, ix)
			s += fmt.Sprintf("\n"+
				"\nfunc (m %[1]s%[2]s) FillVDLTarget(t %[3]sTarget, tt *%[3]sType) error {",
				def.Name, f.Name, data.Pkg("v.io/v23/vdl")) +
				genEncDef(data, t, f.Name) +
				"\n\treturn nil" +
				"\n}"
			s += fmt.Sprintf("\n"+
				"\nfunc (m %s%s) MakeVDLTarget() %sTarget {", def.Name, f.Name, data.Pkg("v.io/v23/vdl")) +
				"\n\treturn nil" +
				"\n}"
		}
	default:
		s += typeGo(data, def.BaseType) + def.DocSuffix
		s += fmt.Sprintf("\n"+
			"\nfunc (%[1]s) __VDLReflect(struct{"+
			"\n\tName string `vdl:%[2]q`"+
			"\n}) {"+
			"\n}",
			def.Name, qualifiedIdent(def.File, def.Name))
		s += fmt.Sprintf("\n"+
			"\nfunc (m *%[1]s) FillVDLTarget(t %[2]sTarget, tt *%[2]sType) error {", def.Name, data.Pkg("v.io/v23/vdl")) +
			genEncDef(data, def.Type, "") +
			"\n\treturn nil" +
			"\n}"
	}
	switch {
	case def.Type.Kind() == vdl.Union:
		// handled for specific field structs above
	case isNativeType(def.Type, def.File.Package):
		s += fmt.Sprintf("\n"+
			"\nfunc (m *%[1]s) MakeVDLTarget() %[2]sTarget {", def.Name, data.Pkg("v.io/v23/vdl")) +
			"\n\treturn nil" +
			"\n}"
	default:
		ref, body := genTargetRef(data, def.Type)
		s += fmt.Sprintf("\n"+
			"\nfunc (m *%[1]s) MakeVDLTarget() %[2]sTarget {", def.Name, data.Pkg("v.io/v23/vdl")) +
			fmt.Sprintf("\n\treturn &%s{Value: m}", ref) +
			"\n}"
		s += body
	}
	return s
}

func commaEnumLabels(prefix string, t *vdl.Type) (s string) {
	for ix := 0; ix < t.NumEnumLabel(); ix++ {
		if ix > 0 {
			s += ", "
		}
		s += prefix
		s += t.EnumLabel(ix)
	}
	return
}

func embedGo(data goData, embed *compile.Interface) string {
	return localIdent(data, embed.File, embed.Name)
}
