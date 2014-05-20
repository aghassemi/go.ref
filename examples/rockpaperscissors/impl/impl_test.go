package impl_test

import (
	"testing"

	rps "veyron/examples/rockpaperscissors"
	"veyron/examples/rockpaperscissors/impl"
	"veyron/runtimes/google/naming/mounttable"
	mtlib "veyron/services/mounttable/lib"

	"veyron2"
	"veyron2/ipc"
	"veyron2/naming"
	"veyron2/rt"
	"veyron2/vlog"
)

func startMountTable(t *testing.T, runtime veyron2.Runtime) (string, func()) {
	server, err := runtime.NewServer()
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}
	suffix, dispatcher := "mt", mtlib.NewMountTable()
	if err := server.Register(suffix, dispatcher); err != nil {
		t.Fatalf("Register(%v, %v) failed: %v", suffix, dispatcher, err)
	}
	protocol, hostname := "tcp", "localhost:0"
	endpoint, err := server.Listen(protocol, hostname)
	if err != nil {
		t.Fatalf("Listen(%v, %v) failed: %v", protocol, hostname, err)
	}
	address := naming.JoinAddressName(endpoint.String(), suffix)
	vlog.VI(1).Infof("Mount table running at endpoint: %s", address)
	return address, func() {
		if err := server.Stop(); err != nil {
			t.Fatalf("Stop() failed: %v", err)
		}
	}
}

func startRockPaperScissors(t *testing.T, rt veyron2.Runtime, mtAddress string) (*impl.RPS, func()) {
	mt, err := mounttable.New(rt, mtAddress)
	if err != nil {
		t.Fatalf("mounttable.New failed: %v", err)
	}
	server, err := rt.NewServer(veyron2.MountTable(mt))
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	rpsService := impl.NewRPS(mt)
	if err := server.Register("", ipc.SoloDispatcher(rps.NewServerRockPaperScissors(rpsService), nil)); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if _, err = server.Listen("tcp", "localhost:0"); err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	names := []string{"rps/judge/test", "rps/player/test", "rps/scorekeeper/test"}
	for _, n := range names {
		if err := server.Publish(n); err != nil {
			t.Fatalf("Publish(%v) failed: %v", n, err)
		}
	}
	return rpsService, func() {
		if err := server.Stop(); err != nil {
			t.Fatalf("Stop() failed: %v", err)
		}
	}
}

// TestRockPaperScissorsImpl runs one rock-paper-scissors game and verifies
// that all the counters are consistent.
func TestRockPaperScissorsImpl(t *testing.T) {
	runtime := rt.Init()
	defer runtime.Shutdown()
	mtAddress, mtStop := startMountTable(t, runtime)
	defer mtStop()
	rpsService, rpsStop := startRockPaperScissors(t, runtime, mtAddress)
	defer rpsStop()

	if err := rpsService.Player().InitiateGame(); err != nil {
		t.Errorf("Failed to initiate game: %v", err)
	}

	// There was only one game, but the player played twice. So, we
	// expected the player to show that it played 2 games, and won 1.
	played, won := rpsService.Player().Stats()
	if want, got := int64(2), played; want != got {
		t.Errorf("Unexpected number of played games. Got %d, want %d", got, want)
	}
	if want, got := int64(1), won; want != got {
		t.Errorf("Unexpected number of won games. Got %d, want %d", got, want)
	}

	// The Judge ran exactly one game.
	if want, got := int64(1), rpsService.Judge().Stats(); want != got {
		t.Errorf("Unexpected number of games run. Got %d, want %d", got, want)
	}

	// The Score Keeper received one score card.
	if want, got := int64(1), rpsService.ScoreKeeper().Stats(); want != got {
		t.Errorf("Unexpected number of score cards. Got %d, want %d", got, want)
	}
}
