package benchmark

import (
	"v.io/core/veyron/security/flag"

	"v.io/core/veyron2"
	"v.io/core/veyron2/context"
	"v.io/core/veyron2/ipc"
	"v.io/core/veyron2/vlog"
)

type impl struct {
}

func (i *impl) Echo(ctx ipc.ServerContext, payload []byte) ([]byte, error) {
	return payload, nil
}

func (i *impl) EchoStream(ctx BenchmarkEchoStreamContext) error {
	rStream := ctx.RecvStream()
	sStream := ctx.SendStream()
	for rStream.Advance() {
		sStream.Send(rStream.Value())
	}
	if err := rStream.Err(); err != nil {
		return err
	}
	return nil
}

// StartServer starts a server that implements the Benchmark service. The
// server listens to the given protocol and address, and returns the veyron
// address of the server and a callback function to stop the server.
func StartServer(ctx *context.T, listenSpec ipc.ListenSpec) (string, func()) {
	server, err := veyron2.NewServer(ctx)
	if err != nil {
		vlog.Fatalf("NewServer failed: %v", err)
	}
	eps, err := server.Listen(listenSpec)
	if err != nil {
		vlog.Fatalf("Listen failed: %v", err)
	}

	if err := server.Serve("", BenchmarkServer(&impl{}), flag.NewAuthorizerOrDie()); err != nil {
		vlog.Fatalf("Serve failed: %v", err)
	}
	return eps[0].Name(), func() {
		if err := server.Stop(); err != nil {
			vlog.Fatalf("Stop() failed: %v", err)
		}
	}
}
