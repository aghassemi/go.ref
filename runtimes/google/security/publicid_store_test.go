package security

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"veyron2/security"
)

// verifyNamesAndPublicKey checks that the provided id has exactly the provided
// set of names and the provided public key. If the provided set is empty then
// the provided error must be errNoMatchingIDs.
func verifyNamesAndPublicKey(id security.PublicID, err error, names []string, pkey security.PublicKey) bool {
	if id == nil {
		return err == errNoMatchingIDs && len(names) == 0
	}
	idNamesMap := make(map[string]bool)
	namesMap := make(map[string]bool)
	for _, n := range id.Names() {
		idNamesMap[n] = true
	}
	for _, n := range names {
		namesMap[n] = true
	}
	return reflect.DeepEqual(idNamesMap, namesMap) && reflect.DeepEqual(id.PublicKey(), pkey)
}

func TestStoreAdd(t *testing.T) {
	var (
		// test principals
		cAlice       = newChain("alice")
		cBob         = newChain("bob")
		cVeyronAlice = derive(bless(cAlice.PublicID(), veyronChain, "alice", nil), cAlice)
		sAlice       = newSetPublicID(cAlice.PublicID(), cVeyronAlice.PublicID())
	)
	s, err := NewPublicIDStore(nil)
	if err != nil {
		t.Fatalf("NewPublicIDStore failed: %s", err)
	}
	// First Add should succeed for any PublicID (cAlice.PublicID() below)
	if err := s.Add(cAlice.PublicID(), "alice/..."); err != nil {
		t.Fatalf("%s.Add(%q, ...) failed unexpectedly: %s", s, cAlice.PublicID(), err)
	}
	// Subsequent Adds must succeed only for PublicIDs with cAlice's public key.
	if err := s.Add(cVeyronAlice.PublicID(), "..."); err != nil {
		t.Fatalf("%s.Add(%q, ...) failed unexpectedly: %s", s, cVeyronAlice.PublicID(), err)
	}
	if err := s.Add(sAlice, "alice/..."); err != nil {
		t.Fatalf("%s.Add(%q, ...) failed unexpectedly: %s", s, sAlice, err)
	}
	if got, want := s.Add(cBob.PublicID(), "bob/..."), errStoreAddMismatch; got != want {
		t.Fatalf("%s.Add(%q, ...): got: %s, want: %s", s, cBob, got, want)
	}
}

func TestSetDefaultPattern(t *testing.T) {
	s, err := NewPublicIDStore(nil)
	if err != nil {
		t.Fatalf("NewPublicIDStore failed: %s", err)
	}
	defaultPatterns := []struct {
		pattern security.BlessingPattern
		success bool
	}{
		{"veyron", true},
		{"veyron/alice@google", true},
		{"veyron/alice@google/bob", true},
		{"veyron/alice@google/...", true},
		{"", false},
		{"veyron...", false},
		{"...veyron", false},
		{"/veyron", false},
		{"veyron/", false},
		{"veyron/.../alice", false},
	}
	for _, d := range defaultPatterns {
		if got := s.SetDefaultBlessingPattern(d.pattern); d.success != (got == nil) {
			t.Errorf("%s.SetDefaultPattern(%q) returned: %v, expected it to succeed: %v", s, d.pattern, got, d.success)
		}
	}
}

func TestStoreGetters(t *testing.T) {
	add := func(s security.PublicIDStore, id security.PublicID, peers security.BlessingPattern) {
		if err := s.Add(id, peers); err != nil {
			t.Fatalf("%s.Add(%q, %q) failed unexpectedly: %s", s, id, peers, err)
		}
	}
	var (
		// test principals
		cAlice            = newChain("alice")
		cBob              = newChain("bob")
		cVService         = newChain("vservice")
		cGService         = newChain("gservice")
		cApp              = newChain("app")
		cVeyronService    = derive(bless(cVService.PublicID(), veyronChain, "service", nil), cVService)
		cGoogleService    = derive(bless(cGService.PublicID(), googleChain, "service", nil), cGService)
		cGoogleServiceApp = derive(bless(cApp.PublicID(), cGoogleService, "app", nil), cApp)
		// PublicIDs for Alice's PublicIDStore
		cGoogleAlice        = bless(cAlice.PublicID(), googleChain, "alice", nil)
		cVeyronAlice        = bless(cAlice.PublicID(), veyronChain, "alice", nil)
		cGoogleServiceAlice = bless(cAlice.PublicID(), cGoogleService, "user-42", nil)
		cVeyronServiceAlice = bless(cAlice.PublicID(), cVeyronService, "user-24", nil)

		sGoogleAlice = newSetPublicID(cGoogleServiceAlice, cGoogleAlice)
		sAllAlice    = newSetPublicID(sGoogleAlice, cVeyronAlice, cVeyronServiceAlice)
		// TODO(ataly): Test with SetPublicIDs as well.
	)

	// Create a new PublicIDStore and add Add Alice's PublicIDs to the store.
	s, err := NewPublicIDStore(nil)
	if err != nil {
		t.Fatalf("NewPublicIDStore failed: %s", err)
	}
	add(s, cGoogleAlice, "google")                    // use cGoogleAlice against all peers matching "google/..."
	add(s, cGoogleAlice, "veyron")                    // use cGoogleAlice against all peers matching "veyron/..." as well
	add(s, cVeyronAlice, "veyron/...")                // use cVeyronAlice against peers matching "veyron/..."
	add(s, cVeyronAlice, "google")                    // use cVeyronAlice against peers matching "veyron/..."
	add(s, cVeyronServiceAlice, "veyron/service/...") // use cVeyronAlice against peers matching "veyron/service/..."
	add(s, cGoogleServiceAlice, "google/service/...") // use cGoogleServiceAlice against peers matching "google/service/..."
	add(s, sGoogleAlice, "google/service")            // use any PublicID from sGoogleAlice against peers matching "google/service"
	add(s, sAllAlice, "veyron")                       // use any PublicID from sAllAlice against peers matching "veyron"

	pkey := cAlice.PublicID().PublicKey()

	// Test ForPeer.
	testDataForPeer := []struct {
		peer  security.PublicID
		names []string
	}{
		{veyronChain.PublicID(), []string{"google/alice", "veyron/alice", "veyron/service/user-24", "google/service/user-42"}},
		{cVeyronService.PublicID(), []string{"veyron/alice", "veyron/service/user-24"}},
		{googleChain.PublicID(), []string{"veyron/alice", "google/alice", "google/service/user-42"}},
		{cGoogleService.PublicID(), []string{"google/alice", "google/service/user-42"}},
		{cGoogleServiceApp.PublicID(), []string{"google/service/user-42"}},
		{cBob.PublicID(), nil},
	}
	for _, d := range testDataForPeer {
		if got, err := s.ForPeer(d.peer); !verifyNamesAndPublicKey(got, err, d.names, pkey) {
			t.Errorf("%s.ForPeer(%s): got: %q, want PublicID with the exact set of names %q", s, d.peer, got, d.names)
		}
	}

	// Test initial DefaultPublicID -- we expect a PublicID with the union of the sets of names of all
	// PublicIDs in the store.
	defaultNames := []string{"google/alice", "veyron/alice", "veyron/service/user-24", "google/service/user-42"}
	if got, err := s.DefaultPublicID(); !verifyNamesAndPublicKey(got, err, defaultNames, pkey) {
		t.Errorf("%s.DefaultPublicID(): got: %s, want PublicID with the exact set of names: %s", s, got, defaultNames)
	}

	// Test SetDefaultBlessingPattern.
	testDataByBlessingPattern := []struct {
		defaultPattern security.BlessingPattern
		defaultNames   []string
	}{
		{"veyron", nil},
		{"veyron/...", []string{"veyron/alice", "veyron/service/user-24"}},
		{"veyron/alice", []string{"veyron/alice"}},
		{"veyron/service/...", []string{"veyron/service/user-24"}},
		{"google", nil},
		{"google/...", []string{"google/alice", "google/service/user-42"}},
		{"google/alice", []string{"google/alice"}},
		{"google/service/...", []string{"google/service/user-42"}},
		{"bob", nil},
	}
	for _, d := range testDataByBlessingPattern {
		s.SetDefaultBlessingPattern(d.defaultPattern)
		if got, err := s.DefaultPublicID(); !verifyNamesAndPublicKey(got, err, d.defaultNames, pkey) {
			t.Errorf("%s.DefaultPublicID(): got: %s, want PublicID with the exact set of names: %s", s, got, d.defaultNames)
		}
	}
}

func TestPublicIDStorePersistence(t *testing.T) {
	newTempDir := func(name string) string {
		dir, err := ioutil.TempDir("", name)
		if err != nil {
			t.Fatal(err)
		}
		return dir
	}

	var (
		signer = newChain("signer")

		cAlice       = newChain("alice")
		cBob         = newChain("bob")
		cVeyronAlice = bless(cAlice.PublicID(), veyronChain, "alice", nil)
		cGoogleAlice = bless(cAlice.PublicID(), googleChain, "alice", nil)
		sAllAlice    = newSetPublicID(cGoogleAlice, cVeyronAlice)

		pkey = cAlice.PublicID().PublicKey()
	)

	// Create a new PublicIDStore that saves all mutations to the provided directory.
	dir := newTempDir("publicid_store")
	defer os.RemoveAll(dir)

	s, err := NewPublicIDStore(&PublicIDStoreParams{dir, signer})
	if err != nil {
		t.Fatalf("NewPublicIDStore failed: %s", err)
	}
	if err := s.Add(sAllAlice, "google/..."); err != nil {
		t.Fatalf("%s.Add(%q, ...) failed unexpectedly: %s", s, sAllAlice, err)
	}
	if err := s.SetDefaultBlessingPattern("veyron/..."); err != nil {
		t.Fatalf("%s.SetDefaultBlessingPattern failed: %s", s, err)
	}

	// Test that all mutations are appropriately reflected in a PublicIDStore read from
	// the directory.
	s, err = NewPublicIDStore(&PublicIDStoreParams{dir, signer})
	if err != nil {
		t.Fatalf("NewPublicIDStore failed: %s", err)
	}

	testDataForPeer := []struct {
		peer  security.PublicID
		names []string
	}{
		{googleChain.PublicID(), []string{"veyron/alice", "google/alice"}},
		{veyronChain.PublicID(), nil},
		{cBob.PublicID(), nil},
	}
	for _, d := range testDataForPeer {
		if got, err := s.ForPeer(d.peer); !verifyNamesAndPublicKey(got, err, d.names, pkey) {
			t.Errorf("%s.ForPeer(%s): got: %q, want PublicID with the exact set of names %q", s, d.peer, got, d.names)
		}
	}

	wantDefaultNames := []string{"veyron/alice"}
	if got, err := s.DefaultPublicID(); !verifyNamesAndPublicKey(got, err, wantDefaultNames, pkey) {
		t.Errorf("%s.DefaultPublicID(): got: %s, want PublicID with the exact set of names: %s", s, got, wantDefaultNames)
	}

	diffPubKeyID := newChain("immaterial").PublicID()
	if got, want := s.Add(diffPubKeyID, security.AllPrincipals), errStoreAddMismatch; got != want {
		t.Fatalf("%s.Add(%q, ...): got: %v, want: %v", s, diffPubKeyID, got, want)
	}

}
