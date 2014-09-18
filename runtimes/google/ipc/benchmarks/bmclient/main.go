// a simple command-line tool to run the benchmark client.
package main

import (
	"flag"
	"os"

	"veyron.io/veyron/veyron/runtimes/google/ipc/benchmarks"

	"veyron.io/veyron/veyron2/rt"
)

var (
	server      = flag.String("server", "", "object name of the server to connect to")
	count       = flag.Int("count", 1, "number of RPCs to send")
	chunkCount  = flag.Int("chunk_count", 0, "number of stream chunks to send")
	payloadSize = flag.Int("payload_size", 32, "the size of the payload")
)

func main() {
	r := rt.Init()
	ctx := r.NewContext()
	if *chunkCount == 0 {
		benchmarks.CallEcho(ctx, *server, *count, *payloadSize, os.Stdout)
	} else {
		benchmarks.CallEchoStream(*server, *count, *chunkCount, *payloadSize, os.Stdout)
	}
}
