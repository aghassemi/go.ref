package benchmarks

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"v.io/core/veyron/lib/testutil"

	"v.io/core/veyron2/context"
	"v.io/core/veyron2/vlog"
)

// CallEcho calls 'Echo' method 'iterations' times with the given payload
// size, and optionally updates the stats.
func CallEcho(b *testing.B, ctx context.T, address string, iterations, payloadSize int, stats *testutil.BenchStats) {
	stub := BenchmarkClient(address)
	payload := make([]byte, payloadSize)
	for i := range payload {
		payload[i] = byte(i & 0xff)
	}

	if stats != nil {
		stats.Clear()
	}

	b.SetBytes(int64(payloadSize) * 2) // 2 for round trip of each payload.
	b.ResetTimer()                     // Exclude setup time from measurement.

	for i := 0; i < iterations; i++ {
		b.StartTimer()
		start := time.Now()

		r, err := stub.Echo(ctx, payload)

		elapsed := time.Since(start)
		b.StopTimer()

		if err != nil {
			vlog.Fatalf("Echo failed: %v", err)
		}
		if !bytes.Equal(r, payload) {
			vlog.Fatalf("Echo returned %v, but expected %v", r, payload)
		}

		if stats != nil {
			stats.Add(elapsed)
		}
	}
}

// CallEchoStream calls 'EchoStream' method 'iterations' times. Each iteration
// sends 'chunkCnt' chunks on the stream and receives the same number of chunks
// back. Each chunk has the given payload size. Optionally updates the stats.
func CallEchoStream(b *testing.B, ctx context.T, address string, iterations, chunkCnt, payloadSize int, stats *testutil.BenchStats) {
	done, _ := StartEchoStream(b, ctx, address, iterations, chunkCnt, payloadSize, stats)
	<-done
}

// StartEchoStream starts to call 'EchoStream' method 'iterations' times.
// This does not block, and returns a channel that will receive the number
// of iterations when it's done. It also returns a callback function to stop
// the streaming. Each iteration requests 'chunkCnt' chunks on the stream and
// receives that number of chunks back. Each chunk has the given payload size.
// Optionally updates the stats. Zero 'iterations' means unlimited.
func StartEchoStream(b *testing.B, ctx context.T, address string, iterations, chunkCnt, payloadSize int, stats *testutil.BenchStats) (<-chan int, func()) {
	stub := BenchmarkClient(address)
	payload := make([]byte, payloadSize)
	for i := range payload {
		payload[i] = byte(i & 0xff)
	}

	if stats != nil {
		stats.Clear()
	}

	done, stop := make(chan int, 1), make(chan struct{})
	stopped := func() bool {
		select {
		case <-stop:
			return true
		default:
			return false
		}
	}

	if b.N > 0 {
		// 2 for round trip of each payload.
		b.SetBytes(int64((iterations*chunkCnt/b.N)*payloadSize) * 2)
	}
	b.ResetTimer() // Exclude setup time from measurement.

	go func() {
		defer close(done)

		n := 0
		for ; !stopped() && (iterations == 0 || n < iterations); n++ {
			b.StartTimer()
			start := time.Now()

			stream, err := stub.EchoStream(ctx)
			if err != nil {
				vlog.Fatalf("EchoStream failed: %v", err)
			}

			rDone := make(chan error, 1)
			go func() {
				defer close(rDone)

				rStream := stream.RecvStream()
				i := 0
				for ; rStream.Advance(); i++ {
					r := rStream.Value()
					if !bytes.Equal(r, payload) {
						rDone <- fmt.Errorf("EchoStream returned %v, but expected %v", r, payload)
						return
					}
				}
				if i != chunkCnt {
					rDone <- fmt.Errorf("EchoStream returned %d chunks, but expected %d", n, chunkCnt)
					return
				}
				rDone <- rStream.Err()
			}()

			sStream := stream.SendStream()
			for i := 0; i < chunkCnt; i++ {
				if err = sStream.Send(payload); err != nil {
					vlog.Fatalf("EchoStream Send failed: %v", err)
				}
			}
			if err = sStream.Close(); err != nil {
				vlog.Fatalf("EchoStream Send failed: %v", err)
			}

			if err = <-rDone; err != nil {
				vlog.Fatalf("%v", err)
			}

			if err = stream.Finish(); err != nil {
				vlog.Fatalf("Finish failed: %v", err)
			}

			elapsed := time.Since(start)
			b.StopTimer()

			if stats != nil {
				stats.Add(elapsed)
			}
		}

		done <- n
	}()

	return done, func() {
		close(stop)
		<-done
	}
}
