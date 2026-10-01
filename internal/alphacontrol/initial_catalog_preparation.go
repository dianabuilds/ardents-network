package alphacontrol

import (
	"encoding/binary"
	"errors"
)

// PrepareInitialCatalogSigningInput prepares only the unsigned initial ACA1
// message. Component references must describe separately signed exact bytes;
// preparation authenticates neither those bytes nor their domain decisions.
func PrepareInitialCatalogSigningInput(input Catalog) ([]byte, error) {
	if input.Generation != 1 || input.PreviousDigest != [32]byte{} || input.Signature != [64]byte{} ||
		!validCatalog(input) || input.NotBefore.Unix() < 0 || input.NotAfter.Unix() < 0 {
		return nil, errors.New("initial alpha catalog is invalid")
	}
	for _, component := range input.Components {
		if component.Generation != 1 || component.NotAfter.Unix() < 0 || !input.NotBefore.Before(component.NotAfter) {
			return nil, errors.New("initial alpha catalog component is invalid")
		}
	}
	payload := append([]byte("ACA1"), 1, byte(len(input.Cohort)))
	payload = append(payload, input.Cohort...)
	payload = binary.BigEndian.AppendUint64(payload, input.Generation)
	payload = binary.BigEndian.AppendUint64(payload, uint64(input.NotBefore.Unix()))
	payload = binary.BigEndian.AppendUint64(payload, uint64(input.NotAfter.Unix()))
	payload = append(payload, input.PreviousDigest[:]...)
	payload = append(payload, 3)
	for _, component := range input.Components {
		payload = append(payload, byte(component.Class))
		payload = append(payload, component.RootID[:]...)
		payload = binary.BigEndian.AppendUint64(payload, component.Generation)
		payload = binary.BigEndian.AppendUint64(payload, uint64(component.NotAfter.Unix()))
		payload = binary.BigEndian.AppendUint32(payload, component.Size)
		payload = append(payload, component.Digest[:]...)
	}
	return append([]byte(catalogDomain), payload...), nil
}
