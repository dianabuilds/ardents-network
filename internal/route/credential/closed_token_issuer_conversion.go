package credential

import (
	"errors"
	"os"
	"path/filepath"
)

const (
	closedIssuerLedgerBindingName = ".closed-token-ledger-binding"
	closedIssuerLedgerStageName   = ".closed-token-ledger-next"
)

func (ledger *closedTokenIssuerLedger) retainBinding() error {
	if err := writeIssuerExclusive(filepath.Join(ledger.root, closedIssuerLedgerBindingName), ledger.header()); err != nil {
		return err
	}
	return syncIssuerDirectory(ledger.root)
}

// Promote only the complete, verified old ledger under the existing issuer
// process lease. A crash before rename leaves the old authoritative records;
// a crash after rename leaves the complete new records, never a merged prefix.
func (ledger *closedTokenIssuerLedger) promote() error {
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
	if err := writeIssuerExclusive(stage, raw); err != nil {
		return err
	}
	if err := os.Rename(stage, filepath.Join(ledger.root, closedTokenIssuerLedgerName)); err != nil {
		return err
	}
	return syncIssuerDirectory(ledger.root)
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
