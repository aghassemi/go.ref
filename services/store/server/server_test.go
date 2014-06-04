package server

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"veyron/services/store/raw"

	"veyron2/ipc"
	"veyron2/security"
	"veyron2/services/store"
	"veyron2/services/watch"
	"veyron2/storage"
	"veyron2/vom"
)

var (
	rootPublicID    security.PublicID = security.FakePublicID("root")
	rootName                          = fmt.Sprintf("%s", rootPublicID)
	blessedPublicId security.PublicID = security.FakePublicID("root/blessed")

	nextTransactionID store.TransactionID = 1

	rootCtx    ipc.ServerContext = &testContext{rootPublicID}
	blessedCtx ipc.ServerContext = &testContext{blessedPublicId}
)

type testContext struct {
	id security.PublicID
}

func (*testContext) Server() ipc.Server {
	return nil
}

func (*testContext) Method() string {
	return ""
}

func (*testContext) Name() string {
	return ""
}

func (*testContext) Suffix() string {
	return ""
}

func (*testContext) Label() (l security.Label) {
	return
}

func (*testContext) CaveatDischarges() security.CaveatDischargeMap {
	return nil
}

func (ctx *testContext) LocalID() security.PublicID {
	return ctx.id
}

func (ctx *testContext) RemoteID() security.PublicID {
	return ctx.id
}

func (*testContext) LocalAddr() net.Addr {
	return nil
}

func (*testContext) RemoteAddr() net.Addr {
	return nil
}

func (*testContext) Deadline() (t time.Time) {
	return
}

func (testContext) IsClosed() bool {
	return false
}

func (testContext) Closed() <-chan struct{} {
	return nil
}

// Dir is a simple directory.
type Dir struct {
	Entries map[string]storage.ID
}

func init() {
	vom.Register(&Dir{})
}

func newValue() interface{} {
	return &Dir{}
}

func newTransaction() store.TransactionID {
	nextTransactionID++
	return nextTransactionID
}

func closeTest(config ServerConfig, s *Server) {
	s.Close()
	os.Remove(config.DBName)
}

func newServer() (*Server, func()) {
	dbName, err := ioutil.TempDir(os.TempDir(), "test_server_test.db")
	if err != nil {
		log.Fatal("ioutil.TempDir() failed: ", err)
	}
	config := ServerConfig{
		Admin:  rootPublicID,
		DBName: dbName,
	}
	s, err := New(config)
	if err != nil {
		log.Fatal("server.New() failed: ", err)
	}
	closer := func() { closeTest(config, s) }
	return s, closer
}

type watchResult struct {
	changes chan watch.Change
	err     error
}

func (wr *watchResult) Send(cb watch.ChangeBatch) error {
	for _, change := range cb.Changes {
		wr.changes <- change
	}
	return nil
}

// doWatch executes a watch request and returns a new watchResult.
// Change events may be received on the channel "changes".
// Once "changes" is closed, any error that occurred is stored to "err".
func doWatch(s *Server, ctx ipc.ServerContext, req watch.Request) *watchResult {
	wr := &watchResult{changes: make(chan watch.Change)}
	go func() {
		defer close(wr.changes)
		if err := s.Watch(ctx, req, wr); err != nil {
			wr.err = err
		}
	}()
	return wr
}

func expectExists(t *testing.T, changes []watch.Change, id storage.ID, value string) {
	change := findChange(t, changes, id)
	if change.State != watch.Exists {
		t.Fatalf("Expected id to exist: %v", id)
	}
	cv := change.Value.(*raw.Mutation)
	if cv.Value != value {
		t.Fatalf("Expected Value to be: %v, but was: %v", value, cv.Value)
	}
}

func expectDoesNotExist(t *testing.T, changes []watch.Change, id storage.ID) {
	change := findChange(t, changes, id)
	if change.State != watch.DoesNotExist {
		t.Fatalf("Expected id to not exist: %v", id)
	}
}

func findChange(t *testing.T, changes []watch.Change, id storage.ID) watch.Change {
	for _, change := range changes {
		cv, ok := change.Value.(*raw.Mutation)
		if !ok {
			t.Fatal("Expected a Mutation")
		}
		if cv.ID == id {
			return change
		}
	}
	t.Fatalf("Expected a change for id: %v", id)
	panic("should not reach here")
}

func TestPutGetRemoveRoot(t *testing.T) {
	s, c := newServer()
	defer c()

	o := s.lookupObject("/")
	testPutGetRemove(t, s, o)
}

func TestPutGetRemoveChild(t *testing.T) {
	s, c := newServer()
	defer c()

	{
		// Create a root.
		o := s.lookupObject("/")
		value := newValue()
		tr1 := newTransaction()
		if err := s.CreateTransaction(rootCtx, tr1, nil); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if _, err := o.Put(rootCtx, tr1, value); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if err := s.Commit(rootCtx, tr1); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}

		tr2 := newTransaction()
		if err := s.CreateTransaction(rootCtx, tr2, nil); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(rootCtx, tr2); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(rootCtx, tr2); err != nil {
			t.Errorf("Object should exist: %s", err)
		}
		if err := s.Abort(rootCtx, tr2); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
	}

	o := s.lookupObject("/Entries/a")
	testPutGetRemove(t, s, o)
}

func testPutGetRemove(t *testing.T, s *Server, o *object) {
	value := newValue()
	{
		// Check that the object does not exist.
		tr := newTransaction()
		if err := s.CreateTransaction(rootCtx, tr, nil); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(rootCtx, tr); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := o.Get(rootCtx, tr); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}
	}

	{
		// Add the object.
		tr1 := newTransaction()
		if err := s.CreateTransaction(rootCtx, tr1, nil); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if _, err := o.Put(rootCtx, tr1, value); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(rootCtx, tr1); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(rootCtx, tr1); err != nil {
			t.Errorf("Object should exist: %s", err)
		}

		// Transactions are isolated.
		tr2 := newTransaction()
		if err := s.CreateTransaction(rootCtx, tr2, nil); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(rootCtx, tr2); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := o.Get(rootCtx, tr2); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}

		// Apply tr1.
		if err := s.Commit(rootCtx, tr1); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}

		// tr2 is still isolated.
		if ok, err := o.Exists(rootCtx, tr2); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := o.Get(rootCtx, tr2); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}

		// tr3 observes the commit.
		tr3 := newTransaction()
		if err := s.CreateTransaction(rootCtx, tr3, nil); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(rootCtx, tr3); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(rootCtx, tr3); err != nil {
			t.Errorf("Object should exist: %s", err)
		}
	}

	{
		// Remove the object.
		tr1 := newTransaction()
		if err := s.CreateTransaction(rootCtx, tr1, nil); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if err := o.Remove(rootCtx, tr1); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(rootCtx, tr1); ok || err != nil {
			t.Errorf("Should not exist: %s", err)
		}
		if v, err := o.Get(rootCtx, tr1); v.Stat.ID.IsValid() || err == nil {
			t.Errorf("Object should not exist: %T, %v, %s", v, v, err)
		}

		// The removal is isolated.
		tr2 := newTransaction()
		if err := s.CreateTransaction(rootCtx, tr2, nil); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(rootCtx, tr2); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(rootCtx, tr2); err != nil {
			t.Errorf("Object should exist: %s", err)
		}

		// Apply tr1.
		if err := s.Commit(rootCtx, tr1); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}

		// The removal is isolated.
		if ok, err := o.Exists(rootCtx, tr2); !ok || err != nil {
			t.Errorf("Should exist: %s", err)
		}
		if _, err := o.Get(rootCtx, tr2); err != nil {
			t.Errorf("Object should exist: %s", err)
		}
	}

	{
		// Check that the object does not exist.
		tr1 := newTransaction()
		if err := s.CreateTransaction(rootCtx, tr1, nil); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if ok, err := o.Exists(rootCtx, tr1); ok || err != nil {
			t.Errorf("Should not exist")
		}
		if v, err := o.Get(rootCtx, tr1); v.Stat.ID.IsValid() && err == nil {
			t.Errorf("Should not exist: %v, %s", v, err)
		}
	}
}

func TestNilTransaction(t *testing.T) {
	s, c := newServer()
	defer c()

	if err := s.Commit(rootCtx, nullTransactionID); err != errTransactionDoesNotExist {
		t.Errorf("Unexpected error: %v", err)
	}

	if err := s.Abort(rootCtx, nullTransactionID); err != errTransactionDoesNotExist {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestWatch(t *testing.T) {
	s, c := newServer()
	defer c()

	path1 := "/"
	value1 := "v1"
	var id1 storage.ID

	// Before the watch request has been made, commit a transaction that puts /.
	{
		tr := newTransaction()
		if err := s.CreateTransaction(rootCtx, tr, nil); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		o := s.lookupObject(path1)
		st, err := o.Put(rootCtx, tr, value1)
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		id1 = st.ID
		if err := s.Commit(rootCtx, tr); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
	}

	// Start a watch request.
	req := watch.Request{}
	wr := doWatch(s, rootCtx, req)

	// Check that watch detects the changes in the first transaction.
	{
		change, ok := <-wr.changes
		if !ok {
			t.Error("Expected a change.")
		}
		if change.Continued {
			t.Error("Expected change to be the last in this transaction")
		}
		expectExists(t, []watch.Change{change}, id1, value1)
	}

	path2 := "/a"
	value2 := "v2"
	var id2 storage.ID

	// Commit a second transaction that puts /a.
	{
		tr := newTransaction()
		if err := s.CreateTransaction(rootCtx, tr, nil); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		o := s.lookupObject(path2)
		st, err := o.Put(rootCtx, tr, value2)
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		id2 = st.ID
		if err := s.Commit(rootCtx, tr); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
	}

	// Check that watch detects the changes in the second transaction.
	{
		changes := make([]watch.Change, 0, 0)
		change, ok := <-wr.changes
		if !ok {
			t.Error("Expected a change.")
		}
		if !change.Continued {
			t.Error("Expected change to continue the transaction")
		}
		changes = append(changes, change)
		change, ok = <-wr.changes
		if !ok {
			t.Error("Expected a change.")
		}
		if change.Continued {
			t.Error("Expected change to be the last in this transaction")
		}
		changes = append(changes, change)
		expectExists(t, changes, id1, value1)
		expectExists(t, changes, id2, value2)
	}

	// Check that no errors were encountered.
	if err := wr.err; err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
}

func TestGarbageCollectionOnCommit(t *testing.T) {
	s, c := newServer()
	defer c()

	path1 := "/"
	value1 := "v1"
	var id1 storage.ID

	// Before the watch request has been made, commit a transaction that puts /.
	{
		tr := newTransaction()
		if err := s.CreateTransaction(rootCtx, tr, nil); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		o := s.lookupObject(path1)
		st, err := o.Put(rootCtx, tr, value1)
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		id1 = st.ID
		if err := s.Commit(rootCtx, tr); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
	}

	// Start a watch request.
	req := watch.Request{}
	wr := doWatch(s, rootCtx, req)

	// Check that watch detects the changes in the first transaction.
	{
		change, ok := <-wr.changes
		if !ok {
			t.Error("Expected a change.")
		}
		if change.Continued {
			t.Error("Expected change to be the last in this transaction")
		}
		expectExists(t, []watch.Change{change}, id1, value1)
	}

	path2 := "/a"
	value2 := "v2"
	var id2 storage.ID

	// Commit a second transaction that puts /a.
	{
		tr := newTransaction()
		if err := s.CreateTransaction(rootCtx, tr, nil); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		o := s.lookupObject(path2)
		st, err := o.Put(rootCtx, tr, value2)
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		id2 = st.ID
		if err := s.Commit(rootCtx, tr); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
	}

	// Check that watch detects the changes in the second transaction.
	{
		changes := make([]watch.Change, 0, 0)
		change, ok := <-wr.changes
		if !ok {
			t.Error("Expected a change.")
		}
		if !change.Continued {
			t.Error("Expected change to continue the transaction")
		}
		changes = append(changes, change)
		change, ok = <-wr.changes
		if !ok {
			t.Error("Expected a change.")
		}
		if change.Continued {
			t.Error("Expected change to be the last in this transaction")
		}
		changes = append(changes, change)
		expectExists(t, changes, id1, value1)
		expectExists(t, changes, id2, value2)
	}

	// Commit a third transaction that removes /a.
	{
		tr := newTransaction()
		if err := s.CreateTransaction(rootCtx, tr, nil); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		o := s.lookupObject("/a")
		if err := o.Remove(rootCtx, tr); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if err := s.Commit(rootCtx, tr); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
	}

	// Check that watch detects the changes in the third transaction.
	{
		changes := make([]watch.Change, 0, 0)
		change, ok := <-wr.changes
		if !ok {
			t.Error("Expected a change.")
		}
		if change.Continued {
			t.Error("Expected change to be the last in this transaction")
		}
		changes = append(changes, change)
		expectExists(t, changes, id1, value1)
	}

	// Check that watch detects the garbage collection of /a.
	{
		changes := make([]watch.Change, 0, 0)
		change, ok := <-wr.changes
		if !ok {
			t.Error("Expected a change.")
		}
		if change.Continued {
			t.Error("Expected change to be the last in this transaction")
		}
		changes = append(changes, change)
		expectDoesNotExist(t, changes, id2)
	}
}

func TestTransactionSecurity(t *testing.T) {
	s, c := newServer()
	defer c()

	// Create a root.
	o := s.lookupObject("/")
	value := newValue()

	// Create a transaction in the root's session.
	tr := newTransaction()
	if err := s.CreateTransaction(rootCtx, tr, nil); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	// Check that the transaction cannot be created or accessed by the blessee.
	if err := s.CreateTransaction(blessedCtx, tr, nil); err != errTransactionAlreadyExists {
		t.Errorf("Unexpected error: %v", err)
	}
	if _, err := o.Exists(blessedCtx, tr); err != errPermissionDenied {
		t.Errorf("Unexpected error: %v", err)
	}
	if _, err := o.Get(blessedCtx, tr); err != errPermissionDenied {
		t.Errorf("Unexpected error: %v", err)
	}
	if _, err := o.Put(blessedCtx, tr, value); err != errPermissionDenied {
		t.Errorf("Unexpected error: %v", err)
	}
	if err := o.Remove(blessedCtx, tr); err != errPermissionDenied {
		t.Errorf("Unexpected error: %v", err)
	}
	if err := s.Abort(blessedCtx, tr); err != errPermissionDenied {
		t.Errorf("Unexpected error: %v", err)
	}
	if err := s.Commit(blessedCtx, tr); err != errPermissionDenied {
		t.Errorf("Unexpected error: %v", err)
	}

	// Create a transaction in the blessee's session.
	tr = newTransaction()
	if err := s.CreateTransaction(blessedCtx, tr, nil); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	// Check that the transaction cannot be created or accessed by the root.
	if err := s.CreateTransaction(rootCtx, tr, nil); err != errTransactionAlreadyExists {
		t.Errorf("Unexpected error: %v", err)
	}
	if _, err := o.Exists(rootCtx, tr); err != errPermissionDenied {
		t.Errorf("Unexpected error: %v", err)
	}
	if _, err := o.Get(rootCtx, tr); err != errPermissionDenied {
		t.Errorf("Unexpected error: %v", err)
	}
	if _, err := o.Put(rootCtx, tr, value); err != errPermissionDenied {
		t.Errorf("Unexpected error: %v", err)
	}
	if err := o.Remove(rootCtx, tr); err != errPermissionDenied {
		t.Errorf("Unexpected error: %v", err)
	}
	if err := s.Abort(rootCtx, tr); err != errPermissionDenied {
		t.Errorf("Unexpected error: %v", err)
	}
	if err := s.Commit(rootCtx, tr); err != errPermissionDenied {
		t.Errorf("Unexpected error: %v", err)
	}
}
