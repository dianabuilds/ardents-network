//go:build linux

package admission

import (
	"encoding/binary"
	"errors"
	"time"
)

// Legacy holder-byte regression oracle; no runtime issuer consumes it.
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
