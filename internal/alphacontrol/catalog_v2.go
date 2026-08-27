package alphacontrol

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"
)

const catalogV2Domain = "ardents-alpha-control-catalog-v2\x00"

// SignV2 returns the canonical signed ACA2 catalog encoding. ACA2 neither
// changes nor loosens ACA1's fixed three-component parser.
func SignV2(input CatalogV2, signer ed25519.PrivateKey) ([]byte, error) {
	if len(signer) != ed25519.PrivateKeySize {
		return nil, errors.New("alpha control ACA2 signer is invalid")
	}
	payload, err := catalogV2Payload(input)
	if err != nil {
		return nil, err
	}
	copy(input.Signature[:], ed25519.Sign(signer, append([]byte(catalogV2Domain), payload...)))
	return append(payload, input.Signature[:]...), nil
}

// VerifyV2 verifies one exact ACA2 catalog under the separately pinned
// disclosure key. It is reader evidence only, never Endpoint authorization.
func VerifyV2(raw []byte, public ed25519.PublicKey, at time.Time) (CatalogV2, [32]byte, error) {
	var digest [32]byte
	if len(raw) == 0 || len(raw) > MaximumCatalogSize || len(public) != ed25519.PublicKeySize || at.IsZero() {
		return CatalogV2{}, digest, errors.New("alpha control ACA2 verification input is invalid")
	}
	if len(raw) < 4+1+1+8+8+8+32+1+4*(1+32+8+8+4+32)+ed25519.SignatureSize {
		return CatalogV2{}, digest, errors.New("alpha control ACA2 encoding is truncated")
	}
	payload, signature := raw[:len(raw)-ed25519.SignatureSize], raw[len(raw)-ed25519.SignatureSize:]
	if !ed25519.Verify(public, append([]byte(catalogV2Domain), payload...), signature) {
		return CatalogV2{}, digest, errors.New("alpha control ACA2 signature is invalid")
	}
	catalog, err := decodeCatalogV2Payload(payload)
	if err != nil || !at.Before(catalog.NotAfter) || at.Before(catalog.NotBefore) {
		return CatalogV2{}, digest, errors.New("alpha control ACA2 is invalid or outside validity")
	}
	copy(catalog.Signature[:], signature)
	digest = sha256.Sum256(raw)
	return catalog, digest, nil
}

func catalogV2Payload(input CatalogV2) ([]byte, error) {
	if !validCatalogV2(input) {
		return nil, errors.New("alpha control ACA2 is invalid")
	}
	payload := make([]byte, 0, MaximumCatalogSize-ed25519.SignatureSize)
	payload = append(payload, 'A', 'C', 'A', '2', 2, byte(len(input.Cohort)))
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

func decodeCatalogV2Payload(payload []byte) (CatalogV2, error) {
	if len(payload) < 6 || string(payload[:4]) != "ACA2" || payload[4] != 2 {
		return CatalogV2{}, errors.New("alpha control ACA2 version is invalid")
	}
	offset, cohortLength := 6, int(payload[5])
	if cohortLength == 0 || cohortLength > 64 || offset+cohortLength+8+8+8+32+1 > len(payload) {
		return CatalogV2{}, errors.New("alpha control ACA2 cohort is invalid")
	}
	result := CatalogV2{Cohort: string(payload[offset : offset+cohortLength])}
	offset += cohortLength
	result.Generation = binary.BigEndian.Uint64(payload[offset : offset+8])
	offset += 8
	notBefore, notAfter := binary.BigEndian.Uint64(payload[offset:offset+8]), binary.BigEndian.Uint64(payload[offset+8:offset+16])
	offset += 16
	if notBefore > uint64(^uint64(0)>>1) || notAfter > uint64(^uint64(0)>>1) {
		return CatalogV2{}, errors.New("alpha control ACA2 times are invalid")
	}
	result.NotBefore, result.NotAfter = time.Unix(int64(notBefore), 0).UTC(), time.Unix(int64(notAfter), 0).UTC()
	copy(result.PreviousDigest[:], payload[offset:offset+32])
	offset += 32
	if payload[offset] != 4 {
		return CatalogV2{}, errors.New("alpha control ACA2 component count is invalid")
	}
	offset++
	for index := range result.Components {
		if offset+1+32+8+8+4+32 > len(payload) {
			return CatalogV2{}, errors.New("alpha control ACA2 component is truncated")
		}
		component := Component{Class: ComponentClass(payload[offset])}
		offset++
		copy(component.RootID[:], payload[offset:offset+32])
		offset += 32
		component.Generation = binary.BigEndian.Uint64(payload[offset : offset+8])
		offset += 8
		componentNotAfter := binary.BigEndian.Uint64(payload[offset : offset+8])
		offset += 8
		if componentNotAfter > uint64(^uint64(0)>>1) {
			return CatalogV2{}, errors.New("alpha control ACA2 component expiry is invalid")
		}
		component.NotAfter = time.Unix(int64(componentNotAfter), 0).UTC()
		component.Size = binary.BigEndian.Uint32(payload[offset : offset+4])
		offset += 4
		copy(component.Digest[:], payload[offset:offset+32])
		offset += 32
		result.Components[index] = component
	}
	if offset != len(payload) || !validCatalogV2(result) {
		return CatalogV2{}, errors.New("alpha control ACA2 content is invalid")
	}
	return result, nil
}

func validCatalogV2(input CatalogV2) bool {
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
