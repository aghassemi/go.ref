package keymgr

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	_ "v.io/core/veyron/profiles"
	"v.io/core/veyron/security/agent"
	"v.io/core/veyron/security/agent/server"

	"v.io/core/veyron2/context"
	"v.io/core/veyron2/rt"
	"v.io/core/veyron2/security"
)

func createAgent(ctx *context.T, path string) (*Agent, func(), error) {
	var defers []func()
	cleanup := func() {
		for _, f := range defers {
			f()
		}
	}
	sock, err := server.RunKeyManager(ctx, path, nil)
	var agent *Agent
	if sock != nil {
		defers = append(defers, func() { os.RemoveAll(path) })
		defers = append(defers, func() { sock.Close() })
		fd, err := syscall.Dup(int(sock.Fd()))
		if err != nil {
			return nil, cleanup, err
		}
		agent, err = newAgent(fd)
	}
	return agent, cleanup, err
}

func TestNoDeviceManager(t *testing.T) {
	runtime, err := rt.New()
	if err != nil {
		t.Fatalf("Could not initialize runtime: %s", err)
	}
	defer runtime.Cleanup()

	agent, cleanup, err := createAgent(runtime.NewContext(), "")
	defer cleanup()
	if err == nil {
		t.Fatal(err)
	}
	if agent != nil {
		t.Fatal("No agent should be created when key path is empty")
	}
}

func createClient(ctx *context.T, deviceAgent *Agent, id []byte) (security.Principal, error) {
	file, err := deviceAgent.NewConnection(id)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return createClient2(ctx, file)
}

func createClient2(ctx *context.T, conn *os.File) (security.Principal, error) {
	fd, err := syscall.Dup(int(conn.Fd()))
	if err != nil {
		return nil, err
	}

	return agent.NewAgentPrincipal(ctx, fd)
}

func TestSigning(t *testing.T) {
	runtime, err := rt.New()
	if err != nil {
		t.Fatalf("Could not initialize runtime: %s", err)
	}
	defer runtime.Cleanup()
	ctx := runtime.NewContext()

	path, err := ioutil.TempDir("", "agent")
	if err != nil {
		t.Fatal(err)
	}
	agent, cleanup, err := createAgent(ctx, path)
	defer cleanup()
	if err != nil {
		t.Fatal(err)
	}

	id1, conn1, err := agent.NewPrincipal(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	conn1.Close()
	id2, conn2, err := agent.NewPrincipal(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	conn2.Close()

	dir, err := os.Open(filepath.Join(path, "keys"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := dir.Readdir(-1)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("Expected 2 files created, found %d", len(files))
	}

	a, err := createClient(ctx, agent, id1)
	if err != nil {
		t.Fatal(err)
	}
	b, err := createClient(ctx, agent, id2)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(a.PublicKey(), b.PublicKey()) {
		t.Fatal("Keys should not be equal")
	}
	sig1, err := a.Sign([]byte("foobar"))
	if err != nil {
		t.Fatal(err)
	}
	sig2, err := b.Sign([]byte("foobar"))
	if err != nil {
		t.Fatal(err)
	}
	if !sig1.Verify(a.PublicKey(), []byte("foobar")) {
		t.Errorf("Signature a fails verification")
	}
	if !sig2.Verify(b.PublicKey(), []byte("foobar")) {
		t.Errorf("Signature b fails verification")
	}
	if sig2.Verify(a.PublicKey(), []byte("foobar")) {
		t.Errorf("Signatures should not cross verify")
	}
}

func TestInMemorySigning(t *testing.T) {
	runtime, err := rt.New()
	if err != nil {
		t.Fatalf("Could not initialize runtime: %s", err)
	}
	defer runtime.Cleanup()
	ctx := runtime.NewContext()

	path, err := ioutil.TempDir("", "agent")
	if err != nil {
		t.Fatal(err)
	}
	agent, cleanup, err := createAgent(ctx, path)
	defer cleanup()
	if err != nil {
		t.Fatal(err)
	}

	id, conn, err := agent.NewPrincipal(runtime.NewContext(), true)
	if err != nil {
		t.Fatal(err)
	}

	dir, err := os.Open(filepath.Join(path, "keys"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := dir.Readdir(-1)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("Expected 0 files created, found %d", len(files))
	}

	c, err := createClient2(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := c.Sign([]byte("foobar"))
	if err != nil {
		t.Fatal(err)
	}
	if !sig.Verify(c.PublicKey(), []byte("foobar")) {
		t.Errorf("Signature a fails verification")
	}

	c2, err := createClient(ctx, agent, id)
	if err != nil {
		t.Fatal(err)
	}
	sig, err = c2.Sign([]byte("foobar"))
	if err != nil {
		t.Fatal(err)
	}
	if !sig.Verify(c.PublicKey(), []byte("foobar")) {
		t.Errorf("Signature a fails verification")
	}
}
