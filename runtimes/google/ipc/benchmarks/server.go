package benchmarks

import (
	"io"

	sflag "veyron.io/veyron/veyron/security/flag"

	"veyron.io/veyron/veyron2"
	"veyron.io/veyron/veyron2/ipc"
	"veyron.io/veyron/veyron2/naming"
	"veyron.io/veyron/veyron2/vlog"
)

type impl struct {
}

func (i *impl) Echo(ctx ipc.ServerContext, payload []byte) ([]byte, error) {
	return payload, nil
}

func (i *impl) EchoStream(ctx ipc.ServerCall) error {
	for {
		var chunk []byte
		if err := ctx.Recv(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if err := ctx.Send(chunk); err != nil {
			return err
		}
	}
	return nil
}

// StartServer starts a server that implements the Benchmark service. The
// server listens to the given protocol and address, and returns the veyron
// address of the server and a callback function to stop the server.
func StartServer(runtime veyron2.Runtime, listenSpec ipc.ListenSpec) (string, func()) {
	server, err := runtime.NewServer()
	if err != nil {
		vlog.Fatalf("NewServer failed: %v", err)
	}
	ep, err := server.Listen(listenSpec)
	if err != nil {
		vlog.Fatalf("Listen failed: %v", err)
	}
	if err := server.Serve("", &impl{}, sflag.NewAuthorizerOrDie()); err != nil {
		vlog.Fatalf("Serve failed: %v", err)
	}
	return naming.JoinAddressName(ep.String(), ""), func() {
		if err := server.Stop(); err != nil {
			vlog.Fatalf("Stop() failed: %v", err)
		}
	}
}
