package appcycle

import (
	"fmt"
	"os"
	"sync"

	"v.io/core/veyron2"
	"v.io/core/veyron2/ipc"
	"v.io/core/veyron2/vlog"

	stub "v.io/core/veyron2/services/mgmt/appcycle"
)

type AppCycle struct {
	sync.RWMutex
	waiters      []chan<- string
	taskTrackers []chan<- veyron2.Task
	task         veyron2.Task
	shutDown     bool
	disp         *invoker
}

type invoker struct {
	ac *AppCycle
}

func New() *AppCycle {
	ac := new(AppCycle)
	ac.disp = &invoker{ac}
	return ac
}

func (m *AppCycle) Shutdown() {
	m.Lock()
	defer m.Unlock()
	if m.shutDown {
		return
	}
	m.shutDown = true
	for _, t := range m.taskTrackers {
		close(t)
	}
	m.taskTrackers = nil
}

func (m *AppCycle) stop(msg string) {
	m.RLock()
	defer m.RUnlock()
	if len(m.waiters) == 0 {
		os.Exit(veyron2.UnhandledStopExitCode)
	}
	for _, w := range m.waiters {
		select {
		case w <- msg:
		default:
		}
	}
}

func (m *AppCycle) Stop() {
	m.stop(veyron2.LocalStop)
}

func (*AppCycle) ForceStop() {
	os.Exit(veyron2.ForceStopExitCode)
}

func (m *AppCycle) WaitForStop(ch chan<- string) {
	m.Lock()
	defer m.Unlock()
	m.waiters = append(m.waiters, ch)
}

func (m *AppCycle) TrackTask(ch chan<- veyron2.Task) {
	m.Lock()
	defer m.Unlock()
	if m.shutDown {
		close(ch)
		return
	}
	m.taskTrackers = append(m.taskTrackers, ch)
}

func (m *AppCycle) advanceTask(progress, goal int32) {
	m.Lock()
	defer m.Unlock()
	m.task.Goal += goal
	m.task.Progress += progress
	for _, t := range m.taskTrackers {
		select {
		case t <- m.task:
		default:
			// TODO(caprita): Make it such that the latest task
			// update is always added to the channel even if channel
			// is full.  One way is to pull an element from t and
			// then re-try the push.
		}
	}
}

func (m *AppCycle) AdvanceGoal(delta int32) {
	if delta <= 0 {
		return
	}
	m.advanceTask(0, delta)
}

func (m *AppCycle) AdvanceProgress(delta int32) {
	if delta <= 0 {
		return
	}
	m.advanceTask(delta, 0)
}

func (m *AppCycle) Remote() interface{} {
	return stub.AppCycleServer(m.disp)
}

func (d *invoker) Stop(ctx stub.AppCycleStopContext) error {
	blessings, _ := ctx.RemoteBlessings().ForContext(ctx)
	vlog.Infof("AppCycle Stop request from %v", blessings)
	// The size of the channel should be reasonably sized to expect not to
	// miss updates while we're waiting for the stream to unblock.
	ch := make(chan veyron2.Task, 10)
	d.ac.TrackTask(ch)
	// TODO(caprita): Include identity of Stop issuer in message.
	d.ac.stop(veyron2.RemoteStop)
	for {
		task, ok := <-ch
		if !ok {
			// Channel closed, meaning process shutdown is imminent.
			break
		}
		actask := stub.Task{Progress: task.Progress, Goal: task.Goal}
		vlog.Infof("AppCycle Stop progress %d/%d", task.Progress, task.Goal)
		ctx.SendStream().Send(actask)
	}
	vlog.Infof("AppCycle Stop done")
	return nil
}

func (d *invoker) ForceStop(ipc.ServerContext) error {
	d.ac.ForceStop()
	return fmt.Errorf("ForceStop should not reply as the process should be dead")
}
