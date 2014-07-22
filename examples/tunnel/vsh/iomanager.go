package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"veyron/examples/tunnel"
	"veyron/examples/tunnel/lib"
	"veyron2/vlog"
)

func runIOManager(stdin io.Reader, stdout, stderr io.Writer, stream tunnel.TunnelShellStream) error {
	m := ioManager{stdin: stdin, stdout: stdout, stderr: stderr, stream: stream}
	return m.run()
}

// ioManager manages the forwarding of all the data between the shell and the
// stream.
type ioManager struct {
	stdin          io.Reader
	stdout, stderr io.Writer
	stream         tunnel.TunnelShellStream

	// done receives any error from chan2stream, user2outchan, or
	// stream2user.
	done chan error
	// outchan is used to serialize the output to the stream. This is
	// needed because stream.Send is not thread-safe.
	outchan chan tunnel.ClientShellPacket
}

func (m *ioManager) run() error {
	m.done = make(chan error, 3)
	// outchan is used to serialize the output to the stream.
	// chan2stream() receives data sent by handleWindowResize() and
	// user2outchan() and sends it to the stream.
	m.outchan = make(chan tunnel.ClientShellPacket)
	defer close(m.outchan)
	go m.chan2stream()
	// When the terminal window is resized, we receive a SIGWINCH. Then we
	// send the new window size to the server.
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	defer signal.Stop(winch)
	go m.handleWindowResize(winch)
	// Forward data between the user and the remote shell.
	go m.user2outchan()
	go m.stream2user()
	// Block until something reports an error.
	return <-m.done
}

// chan2stream receives ClientShellPacket from outchan and sends it to stream.
func (m *ioManager) chan2stream() {
	for packet := range m.outchan {
		if err := m.stream.Send(packet); err != nil {
			m.done <- err
			return
		}
	}
	m.done <- io.EOF
}

func (m *ioManager) handleWindowResize(winch chan os.Signal) {
	for _ = range winch {
		ws, err := lib.GetWindowSize()
		if err != nil {
			vlog.Infof("GetWindowSize failed: %v", err)
			continue
		}
		m.outchan <- tunnel.ClientShellPacket{Rows: uint32(ws.Row), Cols: uint32(ws.Col)}
	}
}

// user2stream reads input from stdin and sends it to the outchan.
func (m *ioManager) user2outchan() {
	for {
		buf := make([]byte, 2048)
		n, err := m.stdin.Read(buf[:])
		if err != nil {
			vlog.VI(2).Infof("user2stream: %v", err)
			m.done <- err
			return
		}
		m.outchan <- tunnel.ClientShellPacket{Stdin: buf[:n]}
	}
}

// stream2user reads data from the stream and sends it to either stdout or stderr.
func (m *ioManager) stream2user() {
	for m.stream.Advance() {
		packet := m.stream.Value()

		if len(packet.Stdout) > 0 {
			if n, err := m.stdout.Write(packet.Stdout); n != len(packet.Stdout) || err != nil {
				m.done <- fmt.Errorf("stdout.Write returned (%d, %v) want (%d, nil)", n, err, len(packet.Stdout))
				return
			}
		}
		if len(packet.Stderr) > 0 {
			if n, err := m.stderr.Write(packet.Stderr); n != len(packet.Stderr) || err != nil {
				m.done <- fmt.Errorf("stderr.Write returned (%d, %v) want (%d, nil)", n, err, len(packet.Stderr))
				return
			}
		}
	}
	err := m.stream.Err()
	if err == nil {
		err = io.EOF
	}
	vlog.VI(2).Infof("stream2user: %v", err)
	m.done <- err
}
