package watch

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"time"

	"veyron/services/store/memstore"
	"veyron/services/store/service"

	"veyron2/ipc"
	"veyron2/security"
	"veyron2/services/watch"
)

var (
	ErrWatchClosed            = io.EOF
	ErrUnknownResumeMarker    = errors.New("Unknown ResumeMarker")
	nowResumeMarker           = []byte("now") // UTF-8 conversion.
	initialStateSkippedChange = watch.Change{
		Name:  "",
		State: watch.InitialStateSkipped,
	}
)

type watcher struct {
	// admin is the public id of the store administrator.
	admin security.PublicID
	// dbName is the name of the store's database directory.
	dbName string
	// closed is a channel that is closed when the watcher is closed.
	// Watch invocations finish as soon as possible once the channel is closed.
	closed chan struct{}
	// pending records the number of Watch invocations on this watcher that
	// have not yet finished.
	pending sync.WaitGroup
}

// New returns a new watcher. The returned watcher supports repeated and
// concurrent invocations of Watch until it is closed.
// admin is the public id of the store administrator. dbName is the name of the
// of the store's database directory.
func New(admin security.PublicID, dbName string) (service.Watcher, error) {
	return &watcher{
		admin:  admin,
		dbName: dbName,
		closed: make(chan struct{}),
	}, nil
}

// Watch handles the specified request, processing records in the store log and
// sending changes to the specified watch stream. If the call is cancelled or
// otherwise closed early, Watch will terminate and return an error.
// Watch implements the service.Watcher interface.
func (w *watcher) Watch(ctx ipc.ServerContext, req watch.Request, stream watch.WatcherServiceWatchStream) error {
	// Closing cancel terminates processRequest.
	cancel := make(chan struct{})
	defer close(cancel)

	done := make(chan error, 1)

	w.pending.Add(1)
	// This goroutine does not leak because processRequest is always terminated.
	go func() {
		defer w.pending.Done()
		done <- w.processRequest(cancel, ctx, req, stream)
		close(done)
	}()

	select {
	case err := <-done:
		return err
	// Close cancel and terminate processRequest if:
	// 1) The watcher has been closed.
	// 2) The call closes. This is signalled on the context's closed channel.
	case <-w.closed:
	case <-ctx.Closed():
	}
	return ErrWatchClosed
}

func (w *watcher) processRequest(cancel <-chan struct{}, ctx ipc.ServerContext, req watch.Request, stream watch.WatcherServiceWatchStream) error {
	log, err := memstore.OpenLog(w.dbName, true)
	if err != nil {
		return err
	}
	// This goroutine does not leak because cancel is always closed.
	go func() {
		<-cancel

		// Closing the log terminates any ongoing read, and processRequest
		// returns an error.
		log.Close()

		// stream.Send() is automatically cancelled when the call completes,
		// so we don't explicitly cancel sendChanges.

		// TODO(tilaks): cancel processState(), processTransaction().
	}()

	processor, err := w.findProcessor(ctx.RemoteID(), req)
	if err != nil {
		return err
	}

	// Retrieve the initial timestamp. Changes that occured at or before the
	// initial timestamp will not be sent.
	resumeMarker := req.ResumeMarker
	initialTimestamp, err := resumeMarkerToTimestamp(resumeMarker)
	if err != nil {
		return err
	}
	if isNowResumeMarker(resumeMarker) {
		sendChanges(stream, []watch.Change{initialStateSkippedChange})
	}

	// Process initial state.
	store, err := log.ReadState(w.admin)
	if err != nil {
		return err
	}
	st := store.State
	changes, err := processor.processState(st)
	if err != nil {
		return err
	}
	err = processChanges(stream, changes, initialTimestamp, st.Timestamp())
	if err != nil {
		return err
	}

	for {
		// Process transactions.
		mu, err := log.ReadTransaction()
		if err != nil {
			return err
		}
		changes, err = processor.processTransaction(mu)
		if err != nil {
			return err
		}
		err = processChanges(stream, changes, initialTimestamp, mu.Timestamp)
		if err != nil {
			return err
		}
	}
}

// Close implements the service.Watcher interface.
func (w *watcher) Close() error {
	close(w.closed)
	w.pending.Wait()
	return nil
}

// IsClosed returns true iff the watcher has been closed.
func (w *watcher) isClosed() bool {
	select {
	case <-w.closed:
		return true
	default:
		return false
	}
}

func (w *watcher) findProcessor(client security.PublicID, req watch.Request) (reqProcessor, error) {
	// TODO(tilaks): verify Sync requests.
	// TODO(tilaks): handle application requests.
	return newSyncProcessor(client)
}

func processChanges(stream watch.WatcherServiceWatchStream, changes []watch.Change, initialTimestamp, timestamp uint64) error {
	if timestamp <= initialTimestamp {
		return nil
	}
	addContinued(changes)
	addResumeMarkers(changes, timestampToResumeMarker(timestamp))
	return sendChanges(stream, changes)
}

func sendChanges(stream watch.WatcherServiceWatchStream, changes []watch.Change) error {
	if len(changes) == 0 {
		return nil
	}
	// TODO(tilaks): batch more aggressively.
	return stream.Send(watch.ChangeBatch{Changes: changes})
}

func addContinued(changes []watch.Change) {
	// Last change marks the end of the processed atomic group.
	for i, _ := range changes {
		changes[i].Continued = true
	}
	if len(changes) > 0 {
		changes[len(changes)-1].Continued = false
	}
}

func addResumeMarkers(changes []watch.Change, resumeMarker []byte) {
	for i, _ := range changes {
		changes[i].ResumeMarker = resumeMarker
	}
}

func isNowResumeMarker(resumeMarker []byte) bool {
	return bytes.Equal(resumeMarker, nowResumeMarker)
}

func resumeMarkerToTimestamp(resumeMarker []byte) (uint64, error) {
	if len(resumeMarker) == 0 {
		return 0, nil
	}
	if isNowResumeMarker(resumeMarker) {
		// TODO(tilaks): Get the current resume marker from the log.
		return uint64(time.Now().UnixNano()), nil
	}
	if len(resumeMarker) != 8 {
		return 0, ErrUnknownResumeMarker
	}
	return binary.BigEndian.Uint64(resumeMarker), nil
}

func timestampToResumeMarker(timestamp uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, timestamp)
	return buf
}
