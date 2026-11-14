package issuer

import (
	"errors"
	"os"
	"path/filepath"
)

const (
	closedIssuerLedgerBindingName = ".closed-token-ledger-binding"
	closedIssuerLedgerStageName   = ".closed-token-ledger-next"
)

// issuerLedgerPersistence keeps each open's physical transition operations
// together. The normal owner uses native files; failure tests retain real files
// while failing one write/replace/flush boundary, without shared mutable hooks.
type issuerLedgerPersistence struct {
	writeExclusive func(string, []byte) error
	rename         func(string, string) error
	syncDirectory  func(string) error
}

func nativeIssuerLedgerPersistence() issuerLedgerPersistence {
	return issuerLedgerPersistence{writeIssuerExclusive, os.Rename, syncIssuerDirectory}
}

func (ledger *closedTokenIssuerLedger) retainBinding(storage issuerLedgerPersistence) error {
	if err := storage.writeExclusive(filepath.Join(ledger.root, closedIssuerLedgerBindingName), ledger.header()); err != nil {
		return err
	}
	return storage.syncDirectory(ledger.root)
}

// Promote only the complete, verified old ledger under the existing issuer
// process lease. A crash before rename leaves the old authoritative records;
// a crash after rename leaves the complete new records, never a merged prefix.
func (ledger *closedTokenIssuerLedger) promote(storage issuerLedgerPersistence) error {
	if err := removeClosedIssuerLedgerStage(ledger.root); err != nil {
		return err
	}
	raw := ledger.header()
	for _, record := range ledger.reservations {
		body, err := encodeClosedTokenIssuerReservation(record)
		if err != nil {
			return err
		}
		body[len(body)-1] = 1
		raw = append(raw, body...)
	}
	stage := filepath.Join(ledger.root, closedIssuerLedgerStageName)
	if err := storage.writeExclusive(stage, raw); err != nil {
		return err
	}
	if err := storage.rename(stage, filepath.Join(ledger.root, closedTokenIssuerLedgerName)); err != nil {
		return err
	}
	return storage.syncDirectory(ledger.root)
}

func removeClosedIssuerLedgerStage(root string) error {
	path := filepath.Join(root, closedIssuerLedgerStageName)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("closed token issuer ledger stage is invalid")
	}
	return os.Remove(path)
}
