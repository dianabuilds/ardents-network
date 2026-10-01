package alphacontrol

import (
	"encoding/binary"
	"errors"
)

// PrepareInitialComponentSigningInput encodes the exact domain-separated
// signing input of one initial ACS1 statement. It signs nothing and grants no
// component authority. The operator must separately bind its signer and verify
// the completed catalog through the ordinary inspection owners.
func PrepareInitialComponentSigningInput(input ComponentStatement) ([]byte, error) {
	if input.Generation != 1 || input.Signature != [64]byte{} || !validStatement(input) || input.NotBefore.Unix() < 0 || input.NotAfter.Unix() < 0 {
		return nil, errors.New("initial alpha component statement is invalid")
	}
	payload := append([]byte("ACS1"), 1, byte(input.Class))
	payload = binary.BigEndian.AppendUint64(payload, input.Generation)
	payload = binary.BigEndian.AppendUint64(payload, uint64(input.NotBefore.Unix()))
	payload = binary.BigEndian.AppendUint64(payload, uint64(input.NotAfter.Unix()))
	payload = binary.BigEndian.AppendUint32(payload, uint32(len(input.Body)))
	payload = append(payload, input.Body...)
	return append([]byte(componentDomain), payload...), nil
}
