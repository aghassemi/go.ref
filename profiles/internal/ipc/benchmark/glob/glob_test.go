package glob_test

import (
	"fmt"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/ipc"
	"v.io/v23/naming"
	"v.io/v23/security"

	_ "v.io/x/ref/profiles"
	"v.io/x/ref/test"
)

func TestNothing(t *testing.T) {
}

func RunBenchmarkChan(b *testing.B, bufferSize int) {
	ch := make(chan string, bufferSize)
	go func() {
		for i := 0; i < b.N; i++ {
			ch <- fmt.Sprintf("%09d", i)
		}
		close(ch)
	}()
	for _ = range ch {
		continue
	}
}

func BenchmarkChan0(b *testing.B) {
	RunBenchmarkChan(b, 0)
}

func BenchmarkChan1(b *testing.B) {
	RunBenchmarkChan(b, 1)
}

func BenchmarkChan2(b *testing.B) {
	RunBenchmarkChan(b, 2)
}

func BenchmarkChan4(b *testing.B) {
	RunBenchmarkChan(b, 4)
}

func BenchmarkChan8(b *testing.B) {
	RunBenchmarkChan(b, 8)
}

func BenchmarkChan16(b *testing.B) {
	RunBenchmarkChan(b, 16)
}

func BenchmarkChan32(b *testing.B) {
	RunBenchmarkChan(b, 32)
}

func BenchmarkChan64(b *testing.B) {
	RunBenchmarkChan(b, 64)
}

func BenchmarkChan128(b *testing.B) {
	RunBenchmarkChan(b, 128)
}

func BenchmarkChan256(b *testing.B) {
	RunBenchmarkChan(b, 256)
}

type disp struct {
	obj interface{}
}

func (d *disp) Lookup(suffix string) (interface{}, security.Authorizer, error) {
	return d.obj, nil, nil
}

func startServer(b *testing.B, ctx *context.T, obj interface{}) (string, func(), error) {
	server, err := v23.NewServer(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("failed to start server: %v", err)
	}
	endpoints, err := server.Listen(v23.GetListenSpec(ctx))
	if err != nil {
		return "", nil, fmt.Errorf("failed to listen: %v", err)
	}
	if err := server.ServeDispatcher("", &disp{obj}); err != nil {
		return "", nil, err
	}
	return endpoints[0].Name(), func() { server.Stop() }, nil
}

type globObject struct {
	b          *testing.B
	bufferSize int
}

func (o *globObject) Glob__(call ipc.ServerCall, pattern string) (<-chan naming.GlobReply, error) {
	if pattern != "*" {
		panic("this benchmark only works with pattern='*'")
	}
	ch := make(chan naming.GlobReply, o.bufferSize)
	go func() {
		for i := 0; i < o.b.N; i++ {
			name := fmt.Sprintf("%09d", i)
			ch <- naming.GlobReplyEntry{naming.MountEntry{Name: name}}
		}
		close(ch)
	}()
	return ch, nil
}

type globChildrenObject struct {
	b          *testing.B
	bufferSize int
}

func (o *globChildrenObject) GlobChildren__(call ipc.ServerCall) (<-chan string, error) {
	if call.Suffix() != "" {
		return nil, nil
	}
	ch := make(chan string, o.bufferSize)
	go func() {
		for i := 0; i < o.b.N; i++ {
			ch <- fmt.Sprintf("%09d", i)
		}
		close(ch)
	}()
	return ch, nil
}

func globClient(b *testing.B, ctx *context.T, name string) (int, error) {
	client := v23.GetClient(ctx)
	call, err := client.StartCall(ctx, name, ipc.GlobMethod, []interface{}{"*"})
	if err != nil {
		return 0, err
	}
	var me naming.MountEntry
	b.ResetTimer()
	count := 0
	for {
		if err := call.Recv(&me); err != nil {
			break
		}
		count++
	}
	b.StopTimer()
	if err := call.Finish(); err != nil {
		return 0, err
	}
	return count, nil
}

func RunBenchmarkGlob(b *testing.B, obj interface{}) {
	ctx, shutdown := test.InitForTest()
	defer shutdown()

	addr, stop, err := startServer(b, ctx, obj)
	if err != nil {
		b.Fatalf("startServer failed: %v", err)
	}
	defer stop()

	count, err := globClient(b, ctx, addr)
	if err != nil {
		b.Fatalf("globClient failed: %v", err)
	}
	if count != b.N {
		b.Fatalf("unexpected number of results: got %d, expected %d", count, b.N)
	}
}

func BenchmarkGlob0(b *testing.B) {
	RunBenchmarkGlob(b, &globObject{b, 0})
}

func BenchmarkGlob1(b *testing.B) {
	RunBenchmarkGlob(b, &globObject{b, 1})
}

func BenchmarkGlob2(b *testing.B) {
	RunBenchmarkGlob(b, &globObject{b, 2})
}

func BenchmarkGlob4(b *testing.B) {
	RunBenchmarkGlob(b, &globObject{b, 4})
}

func BenchmarkGlob8(b *testing.B) {
	RunBenchmarkGlob(b, &globObject{b, 8})
}

func BenchmarkGlob16(b *testing.B) {
	RunBenchmarkGlob(b, &globObject{b, 16})
}

func BenchmarkGlob32(b *testing.B) {
	RunBenchmarkGlob(b, &globObject{b, 32})
}

func BenchmarkGlob64(b *testing.B) {
	RunBenchmarkGlob(b, &globObject{b, 64})
}

func BenchmarkGlob128(b *testing.B) {
	RunBenchmarkGlob(b, &globObject{b, 128})
}

func BenchmarkGlob256(b *testing.B) {
	RunBenchmarkGlob(b, &globObject{b, 256})
}

func BenchmarkGlobChildren0(b *testing.B) {
	RunBenchmarkGlob(b, &globChildrenObject{b, 0})
}

func BenchmarkGlobChildren1(b *testing.B) {
	RunBenchmarkGlob(b, &globChildrenObject{b, 1})
}

func BenchmarkGlobChildren2(b *testing.B) {
	RunBenchmarkGlob(b, &globChildrenObject{b, 2})
}

func BenchmarkGlobChildren4(b *testing.B) {
	RunBenchmarkGlob(b, &globChildrenObject{b, 4})
}

func BenchmarkGlobChildren8(b *testing.B) {
	RunBenchmarkGlob(b, &globChildrenObject{b, 8})
}

func BenchmarkGlobChildren16(b *testing.B) {
	RunBenchmarkGlob(b, &globChildrenObject{b, 16})
}

func BenchmarkGlobChildren32(b *testing.B) {
	RunBenchmarkGlob(b, &globChildrenObject{b, 32})
}

func BenchmarkGlobChildren64(b *testing.B) {
	RunBenchmarkGlob(b, &globChildrenObject{b, 64})
}

func BenchmarkGlobChildren128(b *testing.B) {
	RunBenchmarkGlob(b, &globChildrenObject{b, 128})
}

func BenchmarkGlobChildren256(b *testing.B) {
	RunBenchmarkGlob(b, &globChildrenObject{b, 256})
}
