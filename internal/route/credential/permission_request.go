package credential

import (
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"time"
)

const permissionRequestSize = 269

var permissionRequestDomain = []byte("ardents-admission-allocation-v1\x00")

// AllocationRole is the closed local role to which an hourly permission is
// allocated. It is not a Persona, transport identity, or network role.
type AllocationRole byte

const (
	AllocationUser      AllocationRole = 1
	AllocationPublisher AllocationRole = 2
)

// PermissionRequest is the Endpoint holder's exact public request for one
// offline allocation. The holder signature proves possession without exposing
// its private key to Custody or the issuer.
type PermissionRequest struct {
	AuthorityKey [32]byte
	Role         AllocationRole
	Permission   Permission
	HolderProof  [ed25519.SignatureSize]byte
}

// DecodePermissionRequest parses and verifies one canonical allocation request.
func DecodePermissionRequest(raw []byte) (PermissionRequest, error) {
	if len(raw) != permissionRequestSize {
		return PermissionRequest{}, errors.New("permission request framing is invalid")
	}
	request := PermissionRequest{}
	offset := 0
	if string(raw[:8]) != "ARDPAR01" {
		return PermissionRequest{}, errors.New("permission request magic is invalid")
	}
	offset += 8
	for _, field := range []*[32]byte{&request.AuthorityKey, &request.Permission.NetworkID, &request.Permission.IssuerNodeID} {
		copy(field[:], raw[offset:offset+32])
		offset += 32
	}
	request.Permission.DutyGeneration = binary.BigEndian.Uint64(raw[offset : offset+8])
	offset += 8
	for _, field := range []*[32]byte{&request.Permission.PermissionID, &request.Permission.HolderKey} {
		copy(field[:], raw[offset:offset+32])
		offset += 32
	}
	request.Permission.NotBefore = time.Unix(int64(binary.BigEndian.Uint64(raw[offset:offset+8])), 0).UTC()
	offset += 8
	request.Permission.NotAfter = time.Unix(int64(binary.BigEndian.Uint64(raw[offset:offset+8])), 0).UTC()
	offset += 8
	for index := range request.Permission.Maxima {
		request.Permission.Maxima[index] = binary.BigEndian.Uint32(raw[offset : offset+4])
		offset += 4
	}
	request.Role = AllocationRole(raw[offset])
	offset++
	copy(request.HolderProof[:], raw[offset:])
	if err := verifyPermissionRequest(request); err != nil {
		return PermissionRequest{}, err
	}
	return request, nil
}

func permissionRequestUnsigned(request PermissionRequest) []byte {
	raw := make([]byte, 0, permissionRequestSize-ed25519.SignatureSize)
	raw = append(raw, "ARDPAR01"...)
	for _, field := range [][32]byte{request.AuthorityKey, request.Permission.NetworkID, request.Permission.IssuerNodeID} {
		raw = append(raw, field[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, request.Permission.DutyGeneration)
	for _, field := range [][32]byte{request.Permission.PermissionID, request.Permission.HolderKey} {
		raw = append(raw, field[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, uint64(request.Permission.NotBefore.Unix()))
	raw = binary.BigEndian.AppendUint64(raw, uint64(request.Permission.NotAfter.Unix()))
	for _, maximum := range request.Permission.Maxima {
		raw = binary.BigEndian.AppendUint32(raw, maximum)
	}
	return append(raw, byte(request.Role))
}

func permissionRequestTranscript(request PermissionRequest) []byte {
	transcript := make([]byte, 0, len(permissionRequestDomain)+permissionRequestSize-ed25519.SignatureSize)
	transcript = append(transcript, permissionRequestDomain...)
	return append(transcript, permissionRequestUnsigned(request)...)
}

func verifyPermissionRequest(request PermissionRequest) error {
	if err := validatePermissionRequestFacts(request); err != nil ||
		subtle.ConstantTimeCompare(request.HolderProof[:], make([]byte, ed25519.SignatureSize)) == 1 ||
		!ed25519.Verify(ed25519.PublicKey(request.Permission.HolderKey[:]), permissionRequestTranscript(request), request.HolderProof[:]) {
		return errors.New("permission request is invalid")
	}
	return nil
}

func validatePermissionRequestFacts(request PermissionRequest) error {
	permission := request.Permission
	if request.AuthorityKey == [32]byte{} || request.Role != AllocationUser && request.Role != AllocationPublisher ||
		permission.NetworkID == [32]byte{} || permission.IssuerNodeID == [32]byte{} || permission.PermissionID == [32]byte{} ||
		permission.HolderKey == [32]byte{} || permission.DutyGeneration == 0 || permission.NotBefore.IsZero() ||
		permission.NotBefore != permission.NotBefore.UTC() || permission.NotBefore.Truncate(time.Hour) != permission.NotBefore ||
		!permission.NotAfter.Equal(permission.NotBefore.Add(time.Hour)) || permission.Signature != [ed25519.SignatureSize]byte{} {
		return errors.New("permission request facts are invalid")
	}
	any := false
	for _, maximum := range permission.Maxima {
		if maximum > 65536 {
			return errors.New("permission request maximum is invalid")
		}
		any = any || maximum > 0
	}
	if !any {
		return errors.New("permission request maximum is invalid")
	}
	return nil
}
