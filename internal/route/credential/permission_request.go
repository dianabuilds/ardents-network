package credential

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"io"
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

// PreparePermissionRequest generates an independent holder key and fresh
// permission identifier for exactly one closed allocation. The caller must
// retain the returned private key locally; it is not part of the request.
func PreparePermissionRequest(authorityKey, network, issuer [32]byte, duty uint64, role AllocationRole, notBefore time.Time, maxima [3]uint32) (PermissionRequest, ed25519.PrivateKey, error) {
	if authorityKey == [32]byte{} || network == [32]byte{} || issuer == [32]byte{} || duty == 0 {
		return PermissionRequest{}, nil, errors.New("permission request identity is invalid")
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return PermissionRequest{}, nil, err
	}
	request := PermissionRequest{AuthorityKey: authorityKey, Role: role,
		Permission: Permission{NetworkID: network, IssuerNodeID: issuer, DutyGeneration: duty,
			NotBefore: notBefore.UTC(), NotAfter: notBefore.UTC().Add(time.Hour), Maxima: maxima}}
	if _, err := io.ReadFull(rand.Reader, request.Permission.PermissionID[:]); err != nil {
		zeroPrivatePermissionKey(private)
		return PermissionRequest{}, nil, err
	}
	copy(request.Permission.HolderKey[:], public)
	sealed, err := SealPermissionRequest(request, private)
	if err != nil {
		zeroPrivatePermissionKey(private)
		return PermissionRequest{}, nil, err
	}
	return sealed, private, nil
}

// SealPermissionRequest proves that holder owns the exact allocation request.
// It is the only holder-key signing operation in the public request builder.
func SealPermissionRequest(request PermissionRequest, holder ed25519.PrivateKey) (PermissionRequest, error) {
	if err := validatePermissionRequestFacts(request); err != nil || len(holder) != ed25519.PrivateKeySize ||
		[ed25519.PublicKeySize]byte(holder.Public().(ed25519.PublicKey)) != request.Permission.HolderKey {
		return PermissionRequest{}, errors.New("permission request holder is invalid")
	}
	copy(request.HolderProof[:], ed25519.Sign(holder, permissionRequestTranscript(request)))
	return request, nil
}

// EncodePermissionRequest returns the canonical holder-signed allocation
// request. Custody accepts no other permission signing input.
func EncodePermissionRequest(request PermissionRequest) ([]byte, error) {
	if err := verifyPermissionRequest(request); err != nil {
		return nil, err
	}
	return append(permissionRequestUnsigned(request), request.HolderProof[:]...), nil
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

func zeroPrivatePermissionKey(private ed25519.PrivateKey) {
	for index := range private {
		private[index] = 0
	}
}
