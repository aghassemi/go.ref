package serialization

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	mrand "math/rand"
	"reflect"
	"strings"
	"testing"

	"veyron/lib/testutil"

	"veyron2/security"
)

type bufferCloser struct {
	bytes.Buffer
}

func (*bufferCloser) Close() error {
	return nil
}

func signingWrite(d, s io.WriteCloser, signer Signer, writeList [][]byte, opts *Options) error {
	swc, err := NewSigningWriteCloser(d, s, signer, opts)
	if err != nil {
		return fmt.Errorf("NewSigningWriteCloser failed: %s", err)
	}
	for _, b := range writeList {
		if _, err := swc.Write(b); err != nil {
			return fmt.Errorf("signingWriteCloser.Write failed: %s", err)
		}
	}
	if err := swc.Close(); err != nil {
		return fmt.Errorf("signingWriteCloser.Close failed: %s", err)
	}
	return nil
}

func verifyingRead(d, s io.Reader, key security.PublicKey) ([]byte, error) {
	vr, err := NewVerifyingReader(d, s, key)
	if err != nil {
		return nil, fmt.Errorf("NewVerifyingReader failed: %s", err)
	}
	return ioutil.ReadAll(vr)
}

type signerAdapter struct {
	s security.Signer
}

func (s signerAdapter) Sign(message []byte) (security.Signature, error) { return s.s.Sign(nil, message) }
func (s signerAdapter) PublicKey() security.PublicKey                   { return s.s.PublicKey() }

func newSigner() Signer {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	return signerAdapter{security.NewInMemoryECDSASigner(key)}
}

func matchesErrorPattern(err error, pattern string) bool {
	if (len(pattern) == 0) != (err == nil) {
		return false
	}
	return err == nil || strings.Index(err.Error(), pattern) >= 0
}

func TestRoundTrip(t *testing.T) {
	signer := newSigner()
	d, s := &bufferCloser{}, &bufferCloser{}

	testdata := []struct {
		writeList [][]byte
		opts      *Options
	}{
		{[][]byte{testutil.RandomBytes(1)}, nil},
		{[][]byte{testutil.RandomBytes(100)}, nil},
		{[][]byte{testutil.RandomBytes(100)}, &Options{ChunkSizeBytes: 10}},
		{[][]byte{testutil.RandomBytes(25), testutil.RandomBytes(15), testutil.RandomBytes(60), testutil.RandomBytes(5)}, &Options{ChunkSizeBytes: 7}},
	}
	for _, test := range testdata {
		d.Reset()
		s.Reset()

		if err := signingWrite(d, s, signer, test.writeList, test.opts); err != nil {
			t.Errorf("signingWrite(_, _, %v, %v) failed: %s", test.writeList, test.opts, err)
			continue
		}
		dataRead, err := verifyingRead(d, s, signer.PublicKey())
		if err != nil {
			t.Errorf("verifyingRead failed: %s", err)
			continue
		}

		dataWritten := bytes.Join(test.writeList, nil)
		if !reflect.DeepEqual(dataRead, dataWritten) {
			t.Errorf("Read-Write mismatch: data read: %v, data written: %v", dataRead, dataWritten)
			continue
		}
	}
}

func TestIntegrityAndAuthenticity(t *testing.T) {
	tamper := func(b []byte) []byte {
		c := make([]byte, len(b))
		copy(c, b)
		c[mrand.Int()%len(b)] += 1
		return c
	}

	signer := newSigner()
	d, s := &bufferCloser{}, &bufferCloser{}
	if err := signingWrite(d, s, signer, [][]byte{testutil.RandomBytes(100)}, &Options{ChunkSizeBytes: 7}); err != nil {
		t.Fatalf("signingWrite failed: %s", err)
	}

	// copy the data and signature bytes written.
	dataBytes := d.Bytes()
	sigBytes := s.Bytes()

	// Test that any tampering of the data bytes, or any change
	// to the signer causes a verifyingRead to fail.
	testdata := []struct {
		dataBytes, sigBytes []byte
		key                 security.PublicKey
		wantErr             string
	}{
		{dataBytes, sigBytes, signer.PublicKey(), ""},
		{dataBytes, sigBytes, newSigner().PublicKey(), "signature verification failed"},
		{tamper(dataBytes), sigBytes, signer.PublicKey(), "data has been modified"},
	}
	for _, test := range testdata {
		if _, err := verifyingRead(&bufferCloser{*bytes.NewBuffer(test.dataBytes)}, &bufferCloser{*bytes.NewBuffer(test.sigBytes)}, test.key); !matchesErrorPattern(err, test.wantErr) {
			t.Errorf("verifyingRead: got error: %s, want to match: %v", err, test.wantErr)
		}
	}
}
