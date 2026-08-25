package alphacontrol

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"
)

const catalogDomain = "ardents-alpha-control-catalog-v1\x00"

// Sign returns the one canonical signed Catalog v1 encoding.
func Sign(input Catalog, signer ed25519.PrivateKey) ([]byte, error) {
	if len(signer) != ed25519.PrivateKeySize {
		return nil, errors.New("alpha control catalog signer is invalid")
	}
	payload, err := catalogPayload(input)
	if err != nil {
		return nil, err
	}
	copy(input.Signature[:], ed25519.Sign(signer, append([]byte(catalogDomain), payload...)))
	return append(payload, input.Signature[:]...), nil
}

// Verify decodes and verifies one catalog against its independently supplied
// disclosure public key and reader decision time.
func Verify(raw []byte, public ed25519.PublicKey, at time.Time) (Catalog, [32]byte, error) {
	var digest [32]byte
	if len(raw) == 0 || len(raw) > MaximumCatalogSize || len(public) != ed25519.PublicKeySize || at.IsZero() {
		return Catalog{}, digest, errors.New("alpha control catalog verification input is invalid")
	}
	if len(raw) < 4+1+1+8+8+8+32+1+3*(1+32+8+8+4+32)+ed25519.SignatureSize {
		return Catalog{}, digest, errors.New("alpha control catalog encoding is truncated")
	}
	payload, signature := raw[:len(raw)-ed25519.SignatureSize], raw[len(raw)-ed25519.SignatureSize:]
	if !ed25519.Verify(public, append([]byte(catalogDomain), payload...), signature) {
		return Catalog{}, digest, errors.New("alpha control catalog signature is invalid")
	}
	catalog, err := decodeCatalogPayload(payload)
	if err != nil || !at.Before(catalog.NotAfter) || at.Before(catalog.NotBefore) {
		return Catalog{}, digest, errors.New("alpha control catalog is invalid or outside validity")
	}
	copy(catalog.Signature[:], signature)
	digest = sha256.Sum256(raw)
	return catalog, digest, nil
}

func catalogPayload(input Catalog) ([]byte, error) {
	if !validCatalog(input) {
		return nil, errors.New("alpha control catalog is invalid")
	}
	payload := make([]byte, 0, MaximumCatalogSize-ed25519.SignatureSize)
	payload = append(payload, 'A', 'C', 'A', '1', 1, byte(len(input.Cohort)))
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
	return payload, nil
}

func decodeCatalogPayload(payload []byte) (Catalog, error) {
	if len(payload) < 6 || string(payload[:4]) != "ACA1" || payload[4] != 1 {
		return Catalog{}, errors.New("alpha control catalog version is invalid")
	}
	offset, cohortLength := 6, int(payload[5])
	if cohortLength == 0 || cohortLength > 64 || offset+cohortLength+8+8+8+32+1 > len(payload) {
		return Catalog{}, errors.New("alpha control catalog cohort is invalid")
	}
	result := Catalog{Cohort: string(payload[offset : offset+cohortLength])}
	offset += cohortLength
	result.Generation = binary.BigEndian.Uint64(payload[offset : offset+8])
	offset += 8
	notBefore, notAfter := binary.BigEndian.Uint64(payload[offset:offset+8]), binary.BigEndian.Uint64(payload[offset+8:offset+16])
	offset += 16
	if notBefore > uint64(^uint64(0)>>1) || notAfter > uint64(^uint64(0)>>1) {
		return Catalog{}, errors.New("alpha control catalog times are invalid")
	}
	result.NotBefore, result.NotAfter = time.Unix(int64(notBefore), 0).UTC(), time.Unix(int64(notAfter), 0).UTC()
	copy(result.PreviousDigest[:], payload[offset:offset+32])
	offset += 32
	if payload[offset] != 3 {
		return Catalog{}, errors.New("alpha control catalog component count is invalid")
	}
	offset++
	for index := range result.Components {
		if offset+1+32+8+8+4+32 > len(payload) {
			return Catalog{}, errors.New("alpha control catalog component is truncated")
		}
		component := Component{Class: ComponentClass(payload[offset])}
		offset++
		copy(component.RootID[:], payload[offset:offset+32])
		offset += 32
		component.Generation = binary.BigEndian.Uint64(payload[offset : offset+8])
		offset += 8
		notAfter := binary.BigEndian.Uint64(payload[offset : offset+8])
		offset += 8
		if notAfter > uint64(^uint64(0)>>1) {
			return Catalog{}, errors.New("alpha control catalog component expiry is invalid")
		}
		component.NotAfter = time.Unix(int64(notAfter), 0).UTC()
		component.Size = binary.BigEndian.Uint32(payload[offset : offset+4])
		offset += 4
		copy(component.Digest[:], payload[offset:offset+32])
		offset += 32
		result.Components[index] = component
	}
	if offset != len(payload) || !validCatalog(result) {
		return Catalog{}, errors.New("alpha control catalog content is invalid")
	}
	return result, nil
}

func validCatalog(input Catalog) bool {
	if input.Cohort == "" || len(input.Cohort) > 64 || input.Generation == 0 || input.NotBefore.IsZero() || input.NotAfter.IsZero() ||
		!input.NotBefore.Equal(input.NotBefore.UTC().Truncate(time.Second)) || !input.NotAfter.Equal(input.NotAfter.UTC().Truncate(time.Second)) ||
		!input.NotBefore.Before(input.NotAfter) {
		return false
	}
	for index, component := range input.Components {
		if component.Class != ComponentClass(index+1) || component.RootID == [32]byte{} || component.Generation == 0 || component.NotAfter.IsZero() ||
			!component.NotAfter.Equal(component.NotAfter.UTC().Truncate(time.Second)) || !component.NotAfter.Before(input.NotAfter.Add(time.Second)) ||
			component.Size == 0 || component.Size > 64<<20 || component.Digest == [32]byte{} {
			return false
		}
	}
	return true
}
