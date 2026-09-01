package alphacontrol_test

import (
	"crypto/ed25519"
	"encoding/binary"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
)

func signCatalog(input alphacontrol.Catalog, signer ed25519.PrivateKey) ([]byte, error) {
	if len(signer) != ed25519.PrivateKeySize {
		return nil, errors.New("catalog fixture signer is invalid")
	}
	payload := catalogFixturePayload("ACA1", input.Cohort, input.Generation, input.NotBefore.Unix(), input.NotAfter.Unix(),
		input.PreviousDigest, input.Components[:])
	signature := ed25519.Sign(signer, append([]byte("ardents-alpha-control-catalog-v1\x00"), payload...))
	return append(payload, signature...), nil
}

func signCatalogV2(input alphacontrol.CatalogV2, signer ed25519.PrivateKey) ([]byte, error) {
	if len(signer) != ed25519.PrivateKeySize {
		return nil, errors.New("ACA2 fixture signer is invalid")
	}
	payload := catalogFixturePayload("ACA2", input.Cohort, input.Generation, input.NotBefore.Unix(), input.NotAfter.Unix(),
		input.PreviousDigest, input.Components[:])
	signature := ed25519.Sign(signer, append([]byte("ardents-alpha-control-catalog-v2\x00"), payload...))
	return append(payload, signature...), nil
}

func catalogFixturePayload(magic, cohort string, generation uint64, notBefore, notAfter int64, previous [32]byte,
	components []alphacontrol.Component,
) []byte {
	payload := append([]byte(magic), byte(magic[3]-'0'), byte(len(cohort)))
	payload = append(payload, cohort...)
	payload = binary.BigEndian.AppendUint64(payload, generation)
	payload = binary.BigEndian.AppendUint64(payload, uint64(notBefore))
	payload = binary.BigEndian.AppendUint64(payload, uint64(notAfter))
	payload = append(payload, previous[:]...)
	payload = append(payload, byte(len(components)))
	for _, component := range components {
		payload = append(payload, byte(component.Class))
		payload = append(payload, component.RootID[:]...)
		payload = binary.BigEndian.AppendUint64(payload, component.Generation)
		payload = binary.BigEndian.AppendUint64(payload, uint64(component.NotAfter.Unix()))
		payload = binary.BigEndian.AppendUint32(payload, component.Size)
		payload = append(payload, component.Digest[:]...)
	}
	return payload
}

func signComponent(input alphacontrol.ComponentStatement, signer ed25519.PrivateKey) ([]byte, error) {
	if len(signer) != ed25519.PrivateKeySize {
		return nil, errors.New("component fixture signer is invalid")
	}
	payload := append([]byte("ACS1"), 1, byte(input.Class))
	payload = binary.BigEndian.AppendUint64(payload, input.Generation)
	payload = binary.BigEndian.AppendUint64(payload, uint64(input.NotBefore.Unix()))
	payload = binary.BigEndian.AppendUint64(payload, uint64(input.NotAfter.Unix()))
	payload = binary.BigEndian.AppendUint32(payload, uint32(len(input.Body)))
	payload = append(payload, input.Body...)
	signature := ed25519.Sign(signer, append([]byte("ardents-alpha-control-component-v1\x00"), payload...))
	return append(payload, signature...), nil
}
