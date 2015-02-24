package fs

import (
	"v.io/v23/naming"
)

// TP is a convenience function. It prepends the transactionNamePrefix
// to the given path.
func TP(path string) string {
	return naming.Join(transactionNamePrefix, path)
}

func (ms *Memstore) PersistedFile() string {
	return ms.persistedFile
}
