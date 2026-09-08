package credential

import (
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"time"
)

const permissionSize = 228

var permissionDomain = []byte("ardents-issuance-permission-v1\x00")

// Permission is one fixed, offline closed-admission allocation. It has no
// Target, Name, Persona, transport identity, or issuer-selected endpoint.
type Permission struct {
	NetworkID, IssuerNodeID, PermissionID, HolderKey [32]byte
	DutyGeneration                                   uint64
	NotBefore, NotAfter                              time.Time
	Maxima                                           [3]uint32
	Signature                                        [ed25519.SignatureSize]byte
}

// EncodePermission returns the canonical 228-byte signed permission.
func EncodePermission(permission Permission) ([]byte, error) {
	if err := validatePermission(permission); err != nil {
		return nil, err
	}
	raw := permissionUnsigned(permission)
	return append(raw, permission.Signature[:]...), nil
}

// DecodePermission parses one canonical fixed-size permission without
// accepting a caller-provided validity result.
func DecodePermission(raw []byte) (Permission, error) {
	if len(raw) != permissionSize {
		return Permission{}, errors.New("permission framing is invalid")
	}
	permission := Permission{}
	offset := 0
	for _, field := range []*[32]byte{&permission.NetworkID, &permission.IssuerNodeID} {
		copy(field[:], raw[offset:offset+32])
		offset += 32
	}
	permission.DutyGeneration = binary.BigEndian.Uint64(raw[offset : offset+8])
	offset += 8
	for _, field := range []*[32]byte{&permission.PermissionID, &permission.HolderKey} {
		copy(field[:], raw[offset:offset+32])
		offset += 32
	}
	permission.NotBefore = time.Unix(int64(binary.BigEndian.Uint64(raw[offset:offset+8])), 0).UTC()
	offset += 8
	permission.NotAfter = time.Unix(int64(binary.BigEndian.Uint64(raw[offset:offset+8])), 0).UTC()
	offset += 8
	for index := range permission.Maxima {
		permission.Maxima[index] = binary.BigEndian.Uint32(raw[offset : offset+4])
		offset += 4
	}
	copy(permission.Signature[:], raw[offset:])
	if err := validatePermission(permission); err != nil {
		return Permission{}, err
	}
	return permission, nil
}

// VerifyPermission checks the exact authority signature and current bounded
// permission facts. Authority selection remains State's narrow projection.
func VerifyPermission(permission Permission, authority ed25519.PublicKey, network, issuer [32]byte, duty uint64, now time.Time) error {
	if len(authority) != ed25519.PublicKeySize || validatePermission(permission) != nil || permission.NetworkID != network ||
		permission.IssuerNodeID != issuer || permission.DutyGeneration != duty || now.Before(permission.NotBefore) || !now.Before(permission.NotAfter) ||
		!ed25519.Verify(authority, permissionTranscript(permission), permission.Signature[:]) {
		return errors.New("permission does not match closed admission authority")
	}
	return nil
}

func permissionTranscript(permission Permission) []byte {
	transcript := make([]byte, 0, len(permissionDomain)+permissionSize-ed25519.SignatureSize)
	transcript = append(transcript, permissionDomain...)
	return append(transcript, permissionUnsigned(permission)...)
}

func permissionUnsigned(permission Permission) []byte {
	raw := make([]byte, 0, permissionSize-ed25519.SignatureSize)
	for _, field := range [][32]byte{permission.NetworkID, permission.IssuerNodeID} {
		raw = append(raw, field[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, permission.DutyGeneration)
	for _, field := range [][32]byte{permission.PermissionID, permission.HolderKey} {
		raw = append(raw, field[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, uint64(permission.NotBefore.Unix()))
	raw = binary.BigEndian.AppendUint64(raw, uint64(permission.NotAfter.Unix()))
	for _, maximum := range permission.Maxima {
		raw = binary.BigEndian.AppendUint32(raw, maximum)
	}
	return raw
}

func validatePermission(permission Permission) error {
	if permission.NetworkID == [32]byte{} || permission.IssuerNodeID == [32]byte{} || permission.PermissionID == [32]byte{} ||
		permission.HolderKey == [32]byte{} || permission.DutyGeneration == 0 || permission.NotBefore.IsZero() || permission.NotBefore != permission.NotBefore.UTC() ||
		permission.NotBefore.Truncate(time.Hour) != permission.NotBefore || !permission.NotAfter.Equal(permission.NotBefore.Add(time.Hour)) {
		return errors.New("permission facts are invalid")
	}
	any := false
	for _, maximum := range permission.Maxima {
		if maximum > 65536 {
			return errors.New("permission maximum is invalid")
		}
		any = any || maximum > 0
	}
	if !any || subtle.ConstantTimeCompare(permission.Signature[:], make([]byte, ed25519.SignatureSize)) == 1 {
		return errors.New("permission signature is invalid")
	}
	return nil
}
