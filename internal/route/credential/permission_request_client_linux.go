//go:build linux

package credential

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"time"
)

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

func zeroPrivatePermissionKey(private ed25519.PrivateKey) {
	for index := range private {
		private[index] = 0
	}
}
