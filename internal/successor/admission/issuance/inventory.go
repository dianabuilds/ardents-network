package issuance

import (
	"encoding/binary"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"
)

// Inventory is local unsigned preparation evidence, never a signed profile.
type Inventory struct {
	Binding Binding
	Keys    []issuerprofile.Key
	Digest  [32]byte
}

func (v Inventory) canonical() []byte {
	raw := append([]byte("ARDIINV1"), bindingBytes(v.Binding)...)
	raw = binary.BigEndian.AppendUint16(raw, uint16(len(v.Keys)))
	for _, k := range v.Keys {
		raw = binary.BigEndian.AppendUint64(raw, k.Window)
		raw = append(raw, k.Class)
		raw = append(raw, k.SPKI...)
	}
	return raw
}

func (v Inventory) clone() Inventory {
	keys := make([]issuerprofile.Key, len(v.Keys))
	for i, k := range v.Keys {
		keys[i] = k
		keys[i].SPKI = append([]byte(nil), k.SPKI...)
	}
	v.Keys = keys
	return v
}
