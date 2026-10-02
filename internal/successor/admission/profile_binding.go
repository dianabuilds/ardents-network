package admission

import "github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"

// PrepareLedgerBinding fills only keys; State profile, authority and duty persist.
func PrepareLedgerBinding(base LedgerBinding, v issuerprofile.Verified) (LedgerBinding, error) {
	b, keys, ok := v.Snapshot()
	if !ok || len(base.Keys) != 0 || base.Network != b.Network || base.Issuer != b.Issuer || !base.Start.Equal(b.Start) || !base.End.Equal(b.End) {
		return LedgerBinding{}, ErrInvalid
	}
	base.Keys = keys
	if !base.valid() {
		return LedgerBinding{}, ErrInvalid
	}
	return base, nil
}
