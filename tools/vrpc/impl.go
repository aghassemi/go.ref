package main

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"veyron.io/lib/cmdline"
	_ "veyron.io/veyron/veyron/profiles/static"
	idl_test_base "veyron.io/veyron/veyron/tools/vrpc/test_base"
	"veyron.io/veyron/veyron2"
	"veyron.io/veyron/veyron2/context"
	"veyron.io/veyron/veyron2/ipc"
	"veyron.io/veyron/veyron2/naming"
	"veyron.io/veyron/veyron2/rt"
	"veyron.io/veyron/veyron2/vdl/vdlutil"
	"veyron.io/veyron/veyron2/vom"
	"veyron.io/veyron/veyron2/wiretype"

	idl_binary "veyron.io/veyron/veyron2/services/mgmt/binary"
	idl_node "veyron.io/veyron/veyron2/services/mgmt/node"
)

const serverDesc = `
<server> identifies the Veyron RPC server. It can either be the object address of
the server or an Object name in which case the vrpc will use Veyron's name
resolution to match this name to an end-point.
`
const methodDesc = `
<method> identifies the name of the method to be invoked.
`
const argsDesc = `
<args> identifies the arguments of the method to be invoked. It should be a list
of values in a VOM JSON format that can be reflected to the correct type
using Go's reflection.
`

var cmdDescribe = &cmdline.Command{
	Run:   runDescribe,
	Name:  "describe",
	Short: "Describe the API of an Veyron RPC server",
	Long: `
Describe connects to the Veyron RPC server identified by <server>, finds out what
its API is, and outputs a succint summary of this API to the standard output.
`,
	ArgsName: "<server>",
	ArgsLong: serverDesc,
}

func runDescribe(cmd *cmdline.Command, args []string) error {
	if len(args) != 1 {
		return cmd.UsageErrorf("describe: incorrect number of arguments, expected 1, got %d", len(args))
	}

	runtime := rt.R()

	client, err := setupClient(cmd, runtime)
	if err != nil {
		return err
	}
	defer client.Close()

	ctx, cancel := runtime.NewContext().WithTimeout(time.Minute)
	defer cancel()
	signature, err := getSignature(ctx, cmd, args[0], client)
	if err != nil {
		return err
	}

	for methodName, methodSignature := range signature.Methods {
		fmt.Fprintf(cmd.Stdout(), "%s\n", formatSignature(methodName, methodSignature, signature.TypeDefs))
	}
	return nil
}

var cmdInvoke = &cmdline.Command{
	Run:   runInvoke,
	Name:  "invoke",
	Short: "Invoke a method of an Veyron RPC server",
	Long: `
Invoke connects to the Veyron RPC server identified by <server>, invokes the method
identified by <method>, supplying the arguments identified by <args>, and outputs
the results of the invocation to the standard output.
`,
	ArgsName: "<server> <method> <args>",
	ArgsLong: serverDesc + methodDesc + argsDesc,
}

func runInvoke(cmd *cmdline.Command, args []string) error {
	if len(args) < 2 {
		return cmd.UsageErrorf("invoke: incorrect number of arguments, expected at least 2, got %d", len(args))
	}
	server, method, args := args[0], args[1], args[2:]

	runtime := rt.R()

	client, err := setupClient(cmd, runtime)
	if err != nil {
		return err
	}
	defer client.Close()

	ctx, cancel := runtime.NewContext().WithTimeout(time.Minute)
	defer cancel()
	signature, err := getSignature(ctx, cmd, server, client)
	if err != nil {
		return fmt.Errorf("invoke: failed to get signature for %v: %v", server, err)
	}
	if len(signature.Methods) == 0 {
		return fmt.Errorf("invoke: empty signature for %v", server)
	}

	methodSignature, found := signature.Methods[method]
	if !found {
		return fmt.Errorf("invoke: method %s not found", method)
	}

	if len(args) != len(methodSignature.InArgs) {
		return cmd.UsageErrorf("invoke: incorrect number of arguments, expected %d, got %d", len(methodSignature.InArgs), len(args))
	}

	// Register all user-defined types you would like to use.
	//
	// TODO(jsimsa): This is a temporary hack to get vrpc to work. When
	// Benj implements support for decoding arbitrary structs to an
	// empty interface, this will no longer be needed.
	var x1 idl_test_base.Struct
	vom.Register(x1)
	var x2 idl_node.Description
	vom.Register(x2)
	var x3 idl_binary.Description
	vom.Register(x3)
	var x4 naming.VDLMountedServer
	vom.Register(x4)
	var x5 naming.VDLMountEntry
	vom.Register(x5)

	// Decode the inputs from vomJSON-formatted command-line arguments.
	inputs := make([]interface{}, len(args))
	for i := range args {
		var buf bytes.Buffer
		buf.WriteString(args[i])
		buf.WriteByte(0)
		decoder := vom.NewDecoder(&buf)
		if err := decoder.Decode(&inputs[i]); err != nil {
			return fmt.Errorf("decoder.Decode() failed for %s with %v", args[i], err)
		}
	}

	// Initiate the method invocation.
	call, err := client.StartCall(ctx, server, method, inputs)
	if err != nil {
		return fmt.Errorf("client.StartCall(%s, %q, %v) failed with %v", server, method, inputs, err)
	}

	fmt.Fprintf(cmd.Stdout(), "%s = ", formatInput(method, inputs))

	// Handle streaming results, if the method happens to be streaming.
	var item interface{}
	nStream := 0
forloop:
	for ; ; nStream++ {
		switch err := call.Recv(&item); err {
		case io.EOF:
			break forloop
		case nil:
			if nStream == 0 {
				fmt.Fprintln(cmd.Stdout(), "<<")
			}
			fmt.Fprintf(cmd.Stdout(), "%d: %v\n", nStream, item)
		default:
			return fmt.Errorf("call.Recv failed with %v", err)
		}
	}
	if nStream > 0 {
		fmt.Fprintf(cmd.Stdout(), ">> ")
	}

	// Receive the outputs of the method invocation.
	outputs := make([]interface{}, len(methodSignature.OutArgs))
	outputPtrs := make([]interface{}, len(methodSignature.OutArgs))
	for i := range outputs {
		outputPtrs[i] = &outputs[i]
	}
	if err := call.Finish(outputPtrs...); err != nil {
		return fmt.Errorf("call.Finish() failed with %v", err)
	}
	fmt.Fprintf(cmd.Stdout(), "%s\n", formatOutput(outputs))

	return nil
}

// root returns the root command for the vrpc tool.
func root() *cmdline.Command {
	return &cmdline.Command{
		Name:  "vrpc",
		Short: "Tool for interacting with Veyron RPC servers.",
		Long: `
The vrpc tool facilitates interaction with Veyron RPC servers. In particular,
it can be used to 1) find out what API a Veyron RPC server exports and
2) send requests to a Veyron RPC server.
`,
		Children: []*cmdline.Command{cmdDescribe, cmdInvoke},
	}
}

func setupClient(cmd *cmdline.Command, r veyron2.Runtime) (ipc.Client, error) {
	client, err := r.NewClient()
	if err != nil {
		return nil, fmt.Errorf("NewClient failed: %v", err)
	}
	return client, nil
}

func getSignature(ctx context.T, cmd *cmdline.Command, server string, client ipc.Client) (ipc.ServiceSignature, error) {
	call, err := client.StartCall(ctx, server, "Signature", nil)
	if err != nil {
		return ipc.ServiceSignature{}, fmt.Errorf("client.StartCall(%s, Signature, nil) failed with %v", server, err)
	}
	var signature ipc.ServiceSignature
	var sigerr error
	if err = call.Finish(&signature, &sigerr); err != nil {
		return ipc.ServiceSignature{}, fmt.Errorf("client.Finish(&signature, &sigerr) failed with %v", err)
	}
	return signature, sigerr
}

// formatWiretype generates a string representation of the specified type.
func formatWiretype(td []vdlutil.Any, tid wiretype.TypeID) string {
	var wt vdlutil.Any
	if tid >= wiretype.TypeIDFirst {
		wt = td[tid-wiretype.TypeIDFirst]
	} else {
		for _, pair := range wiretype.BootstrapTypes {
			if pair.TID == tid {
				wt = pair.WT
			}
		}
		if wt == nil {
			return fmt.Sprintf("UNKNOWN_TYPE[%v]", tid)
		}
	}

	switch t := wt.(type) {
	case wiretype.NamedPrimitiveType:
		if t.Name != "" {
			return t.Name
		}
		return tid.Name()
	case wiretype.SliceType:
		return fmt.Sprintf("[]%s", formatWiretype(td, t.Elem))
	case wiretype.ArrayType:
		return fmt.Sprintf("[%d]%s", t.Len, formatWiretype(td, t.Elem))
	case wiretype.MapType:
		return fmt.Sprintf("map[%s]%s", formatWiretype(td, t.Key), formatWiretype(td, t.Elem))
	case wiretype.StructType:
		var buf bytes.Buffer
		buf.WriteString("struct{")
		for i, fld := range t.Fields {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(fld.Name)
			buf.WriteString(" ")
			buf.WriteString(formatWiretype(td, fld.Type))
		}
		buf.WriteString("}")
		return buf.String()
	default:
		panic(fmt.Sprintf("unknown writetype: %T", wt))
	}
}

func formatSignature(name string, ms ipc.MethodSignature, defs []vdlutil.Any) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "func %s(", name)
	for index, arg := range ms.InArgs {
		if index > 0 {
			buf.WriteString(", ")
		}
		fmt.Fprintf(&buf, "%s %s", arg.Name, formatWiretype(defs, arg.Type))
	}
	buf.WriteString(") ")
	if ms.InStream != wiretype.TypeIDInvalid || ms.OutStream != wiretype.TypeIDInvalid {
		buf.WriteString("stream<")
		if ms.InStream == wiretype.TypeIDInvalid {
			buf.WriteString("_")
		} else {
			fmt.Fprintf(&buf, "%s", formatWiretype(defs, ms.InStream))
		}
		buf.WriteString(", ")
		if ms.OutStream == wiretype.TypeIDInvalid {
			buf.WriteString("_")
		} else {
			fmt.Fprintf(&buf, "%s", formatWiretype(defs, ms.OutStream))
		}
		buf.WriteString("> ")
	}
	buf.WriteString("(")
	for index, arg := range ms.OutArgs {
		if index > 0 {
			buf.WriteString(", ")
		}
		if arg.Name != "" {
			fmt.Fprintf(&buf, "%s ", arg.Name)
		}
		fmt.Fprintf(&buf, "%s", formatWiretype(defs, arg.Type))
	}
	buf.WriteString(")")
	return buf.String()
}

func formatInput(name string, inputs []interface{}) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s(", name)
	for index, value := range inputs {
		if index > 0 {
			buf.WriteString(", ")
		}
		fmt.Fprintf(&buf, "%v", value)
	}
	buf.WriteString(")")
	return buf.String()
}

func formatOutput(outputs []interface{}) string {
	var buf bytes.Buffer
	buf.WriteString("[")
	for index, value := range outputs {
		if index > 0 {
			buf.WriteString(", ")
		}
		fmt.Fprintf(&buf, "%v", value)
	}
	buf.WriteString("]")
	return buf.String()
}
