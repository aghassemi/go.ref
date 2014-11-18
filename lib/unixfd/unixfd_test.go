package unixfd

import (
	"bytes"
	"io"
	"net"
	"reflect"
	"testing"
)

type nothing struct{}

func dial(fd *fileDescriptor) (net.Conn, net.Addr, error) {
	addr := fd.releaseAddr()
	conn, err := unixFDConn(addr.String())
	return conn, addr, err
}

func listen(fd *fileDescriptor) (net.Listener, net.Addr, error) {
	addr := fd.releaseAddr()
	l, err := unixFDListen(addr.String())
	return l, addr, err
}

func testWrite(t *testing.T, c net.Conn, data string) {
	n, err := c.Write([]byte(data))
	if err != nil {
		t.Errorf("Write: %v", err)
		return
	}
	if n != len(data) {
		t.Errorf("Wrote %d bytes, expected %d", n, len(data))
	}
}

func testRead(t *testing.T, c net.Conn, expected string) {
	buf := make([]byte, len(expected)+2)
	n, err := c.Read(buf)
	if err != nil {
		t.Errorf("Read: %v", err)
		return
	}
	if n != len(expected) || !bytes.Equal(buf[0:n], []byte(expected)) {
		t.Errorf("got %q, expected %q", buf[0:n], expected)
	}
}

func TestDial(t *testing.T) {
	local, remote, err := socketpair(true)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	a, a_addr, err := dial(local)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	b, b_addr, err := dial(remote)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	testWrite(t, a, "TEST1")
	testRead(t, b, "TEST1")
	testWrite(t, b, "TEST2")
	testRead(t, a, "TEST2")

	if !reflect.DeepEqual(a.LocalAddr(), a_addr) {
		t.Errorf("Invalid address %v, expected %v", a.LocalAddr(), a_addr)
	}
	if !reflect.DeepEqual(a.RemoteAddr(), a_addr) {
		t.Errorf("Invalid address %v, expected %v", a.RemoteAddr(), a_addr)
	}
	if !reflect.DeepEqual(b.LocalAddr(), b_addr) {
		t.Errorf("Invalid address %v, expected %v", a.LocalAddr(), b_addr)
	}
	if !reflect.DeepEqual(b.RemoteAddr(), b_addr) {
		t.Errorf("Invalid address %v, expected %v", a.RemoteAddr(), b_addr)
	}
}

func TestListen(t *testing.T) {
	local, remote, err := socketpair(true)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	a, _, err := dial(local)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	l, _, err := listen(remote)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	b, err := l.Accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	start := make(chan nothing, 0)
	done := make(chan nothing)
	go func() {
		defer close(done)
		<-start
		if _, err := l.Accept(); err != io.EOF {
			t.Fatalf("accept: expected EOF, got %v", err)
		}
	}()

	// block until the goroutine starts running
	start <- nothing{}
	testWrite(t, a, "LISTEN")
	testRead(t, b, "LISTEN")

	err = l.Close()
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	<-done

	// After closed, accept should fail immediately
	_, err = l.Accept()
	if err == nil {
		t.Fatalf("Accept succeeded after close")
	}
	err = l.Close()
	if err == nil {
		t.Fatalf("Close succeeded twice")
	}
}

func TestSendConnection(t *testing.T) {
	server, client, err := Socketpair()
	if err != nil {
		t.Fatalf("Socketpair: %v", err)
	}
	uclient, err := net.FileConn(client)
	if err != nil {
		t.Fatalf("FileConn: %v", err)
	}
	var readErr error
	var n int
	var saddr net.Addr
	done := make(chan struct{})
	buf := make([]byte, 10)
	go func() {
		saddr, n, readErr = ReadConnection(server, buf)
		close(done)
	}()
	caddr, err := SendConnection(uclient.(*net.UnixConn), []byte("hello"), true)
	if err != nil {
		t.Fatalf("SendConnection: %v", err)
	}
	<-done
	if readErr != nil {
		t.Fatalf("ReadConnection: %v", readErr)
	}
	if saddr == nil {
		t.Fatalf("ReadConnection returned nil, %d", n)
	}
	data := buf[0:n]
	if !bytes.Equal([]byte("hello"), data) {
		t.Fatalf("unexpected data %q", data)
	}

	a, err := unixFDConn(caddr.String())
	if err != nil {
		t.Fatalf("dial %v: %v", caddr, err)
	}
	b, err := unixFDConn(saddr.String())
	if err != nil {
		t.Fatalf("dial %v: %v", saddr, err)
	}

	testWrite(t, a, "TEST1")
	testRead(t, b, "TEST1")
	testWrite(t, b, "TEST2")
	testRead(t, a, "TEST2")
}
