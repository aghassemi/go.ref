package ipc_test

import (
	"io"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/ipc"
)

type simple struct {
	done <-chan struct{}
}

func (s *simple) Sleep(call ipc.ServerCall) error {
	select {
	case <-s.done:
	case <-time.After(time.Hour):
	}
	return nil
}

func (s *simple) Ping(call ipc.ServerCall) (string, error) {
	return "pong", nil
}

func (s *simple) Source(call ipc.StreamServerCall, start int) error {
	i := start
	backoff := 25 * time.Millisecond
	for {
		select {
		case <-s.done:
			return nil
		case <-time.After(backoff):
			call.Send(i)
			i++
		}
		backoff *= 2
	}
}

func (s *simple) Sink(call ipc.StreamServerCall) (int, error) {
	i := 0
	for {
		if err := call.Recv(&i); err != nil {
			if err == io.EOF {
				return i, nil
			}
			return 0, err
		}
	}
}

func (s *simple) Inc(call ipc.StreamServerCall, inc int) (int, error) {
	i := 0
	for {
		if err := call.Recv(&i); err != nil {
			if err == io.EOF {
				return i, nil
			}
			return 0, err
		}
		call.Send(i + inc)
	}
}

func TestSimpleRPC(t *testing.T) {
	ctx, shutdown := newCtx()
	defer shutdown()
	name, fn := initServer(t, ctx)
	defer fn()

	client := v23.GetClient(ctx)
	call, err := client.StartCall(ctx, name, "Ping", nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	response := ""
	if err := call.Finish(&response); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if got, want := response, "pong"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSimpleStreaming(t *testing.T) {
	ctx, shutdown := newCtx()
	defer shutdown()
	name, fn := initServer(t, ctx)
	defer fn()

	inc := 1
	call, err := v23.GetClient(ctx).StartCall(ctx, name, "Inc", []interface{}{inc})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	want := 10
	for i := 0; i <= want; i++ {
		if err := call.Send(i); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		got := -1
		if err = call.Recv(&got); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if want := i + inc; got != want {
			t.Fatalf("got %d, want %d")
		}
	}
	call.CloseSend()
	final := -1
	err = call.Finish(&final)
	if err != nil {
		t.Errorf("unexpected error: %#v", err)
	}
	if got := final; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
}
