package main

import (
	"crypto/ed25519"
	"encoding/binary"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
)

func signCatalogV2Fixture(input alphacontrol.CatalogV2, signer ed25519.PrivateKey) ([]byte, error) {
	payload := append([]byte("ACA2"), 2, byte(len(input.Cohort)))
	payload = append(payload, input.Cohort...)
	payload = binary.BigEndian.AppendUint64(payload, input.Generation)
	payload = binary.BigEndian.AppendUint64(payload, uint64(input.NotBefore.Unix()))
	payload = binary.BigEndian.AppendUint64(payload, uint64(input.NotAfter.Unix()))
	payload = append(payload, input.PreviousDigest[:]...)
	payload = append(payload, byte(len(input.Components)))
	for _, component := range input.Components {
		payload = append(payload, byte(component.Class))
		payload = append(payload, component.RootID[:]...)
		payload = binary.BigEndian.AppendUint64(payload, component.Generation)
		payload = binary.BigEndian.AppendUint64(payload, uint64(component.NotAfter.Unix()))
		payload = binary.BigEndian.AppendUint32(payload, component.Size)
		payload = append(payload, component.Digest[:]...)
	}
	signature := ed25519.Sign(signer, append([]byte("ardents-alpha-control-catalog-v2\x00"), payload...))
	return append(payload, signature...), nil
}
