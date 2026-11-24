package issuance

import (
	"encoding/json"

	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
)

// LedgerDeclaration fixes the root and issuer binding of an offline ledger.
type LedgerDeclaration struct {
	Root    string
	Binding quota.LedgerBinding
}

// DecodeLedgerDeclaration validates the complete local declaration before effects.
func DecodeLedgerDeclaration(raw []byte) (LedgerDeclaration, error) {
	var declaration LedgerDeclaration
	fields, err := resultFields(raw, "root", "binding")
	if err != nil {
		return declaration, err
	}
	if json.Unmarshal(fields["root"], &declaration.Root) != nil || !pathgrammar.Strict(declaration.Root) {
		return declaration, ErrInvalid
	}
	declaration.Binding, err = DecodeSetupLedgerBinding(fields["binding"])
	return declaration, err
}
