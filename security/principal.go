package security

import (
	"crypto/ecdsa"
	"fmt"
	"os"
	"path"

	"veyron.io/veyron/veyron2/security"
)

const (
	blessingStoreDataFile = "blessingstore.data"
	blessingStoreSigFile  = "blessingstore.sig"

	blessingRootsDataFile = "blessingroots.data"
	blessingRootsSigFile  = "blessingroots.sig"

	privateKeyFile = "privatekey.pem"
)

// NewPrincipal mints a new private key and generates a principal based on
// this key, storing its BlessingRoots and BlessingStore in memory.
func NewPrincipal() (security.Principal, error) {
	pub, priv, err := NewPrincipalKey()
	if err != nil {
		return nil, err
	}
	return security.CreatePrincipal(security.NewInMemoryECDSASigner(priv), newInMemoryBlessingStore(pub), newInMemoryBlessingRoots())
}

// NewPrincipalFromSigner creates a principal using the provided signer, storing its
// BlessingRoots and BlessingStore in memory.
func NewPrincipalFromSigner(signer security.Signer) (security.Principal, error) {
	return security.CreatePrincipal(signer, newInMemoryBlessingStore(signer.PublicKey()), newInMemoryBlessingRoots())
}

// NewPersistentPrincipalFromSigner creates a new principal using the provided Signer and a
// partial state (i.e., BlessingRoots, BlessingStore) that is read from the provided directory 'dir'.
// Changes to the partial state are persisted and commited to the same directory; the provided
// signer isn't persisted: the caller is expected to persist it separately or use the
// {Load,Create}PersistentPrincipal() methods instead.
//
// If the directory does not contain any partial state, new partial state instances are created
// and subsequently commited to 'dir'.  If the directory does not exist, it is created.
func NewPersistentPrincipalFromSigner(signer security.Signer, dir string) (security.Principal, error) {
	if err := mkDir(dir); err != nil {
		return nil, err
	}
	return newPersistentPrincipalFromSigner(signer, dir)
}

// LoadPersistentPrincipal reads state for a principal (private key, BlessingRoots, BlessingStore)
// from the provided directory 'dir' and commits all state changes to the same directory.
// If private key file does not exist then an error 'err' is returned such that os.IsNotExist(err) is true.
// If private key file exists then 'passphrase' must be correct, otherwise PassphraseErr will be returned.
func LoadPersistentPrincipal(dir string, passphrase []byte) (security.Principal, error) {
	key, err := loadKeyFromDir(dir, passphrase)
	if err != nil {
		return nil, err
	}
	return newPersistentPrincipalFromSigner(security.NewInMemoryECDSASigner(key), dir)
}

// CreatePersistentPrincipal creates a new principal (private key, BlessingRoots,
// BlessingStore) and commits all state changes to the provided directory.
//
// The generated private key is serialized and saved encrypted if the 'passphrase'
// is non-nil, and unencrypted otherwise.
//
// If the directory has any preexisting principal data, CreatePersistentPrincipal
// will return an error.
//
// The specified directory may not exist, in which case it gets created by this
// function.
func CreatePersistentPrincipal(dir string, passphrase []byte) (principal security.Principal, err error) {
	if err := mkDir(dir); err != nil {
		return nil, err
	}
	key, err := initKey(dir, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize private key: %v", err)
	}
	return newPersistentPrincipalFromSigner(security.NewInMemoryECDSASigner(key), dir)
}

// InitDefaultBlessings uses the provided principal to create a self blessing for name 'name',
// sets it as default on the principal's BlessingStore and adds it as root to the principal's BlessingRoots.
func InitDefaultBlessings(p security.Principal, name string) error {
	blessing, err := p.BlessSelf(name)
	if err != nil {
		return err
	}
	if err := p.BlessingStore().SetDefault(blessing); err != nil {
		return err
	}
	if _, err := p.BlessingStore().Set(blessing, security.AllPrincipals); err != nil {
		return err
	}
	if err := p.AddToRoots(blessing); err != nil {
		return err
	}
	return nil
}

func newPersistentPrincipalFromSigner(signer security.Signer, dir string) (security.Principal, error) {
	serializationSigner, err := security.CreatePrincipal(signer, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create serialization.Signer: %v", err)
	}
	dataFile := path.Join(dir, blessingRootsDataFile)
	signatureFile := path.Join(dir, blessingRootsSigFile)
	fs, err := NewFileSerializer(dataFile, signatureFile)
	if err != nil {
		return nil, err
	}
	roots, err := newPersistingBlessingRoots(fs, serializationSigner)
	if err != nil {
		return nil, fmt.Errorf("failed to load BlessingRoots from %q: %v", dir, err)
	}
	dataFile = path.Join(dir, blessingStoreDataFile)
	signatureFile = path.Join(dir, blessingStoreSigFile)
	if fs, err = NewFileSerializer(dataFile, signatureFile); err != nil {
		return nil, err
	}
	store, err := newPersistingBlessingStore(fs, serializationSigner)
	if err != nil {
		return nil, fmt.Errorf("failed to load BlessingStore from %q: %v", dir, err)
	}
	return security.CreatePrincipal(signer, store, roots)
}

func mkDir(dir string) error {
	if finfo, err := os.Stat(dir); err == nil {
		if !finfo.IsDir() {
			return fmt.Errorf("%q is not a directory", dir)
		}
	} else if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create %q: %v", dir, err)
	}
	return nil
}

func loadKeyFromDir(dir string, passphrase []byte) (*ecdsa.PrivateKey, error) {
	keyFile := path.Join(dir, privateKeyFile)
	f, err := os.Open(keyFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	key, err := LoadPEMKey(f, passphrase)
	if err != nil {
		return nil, err
	}
	return key.(*ecdsa.PrivateKey), nil
}

func initKey(dir string, passphrase []byte) (*ecdsa.PrivateKey, error) {
	keyFile := path.Join(dir, privateKeyFile)
	f, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open %q for writing: %v", keyFile, err)
	}
	defer f.Close()
	_, key, err := NewPrincipalKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %v", err)
	}
	if err := SavePEMKey(f, key, passphrase); err != nil {
		return nil, fmt.Errorf("failed to save private key to %q: %v", keyFile, err)
	}
	return key, nil
}
