//go:build linux

package stock

import (
	"github.com/dianabuilds/ardents-network/internal/admission"
	publicadmission "github.com/dianabuilds/ardents-network/internal/successor/admission"
)

// decodeFixturePermissionRequest inspects exact public output in a predecessor
// regression. It passes no root, live owner or signing authority between trees.
func decodeFixturePermissionRequest(raw []byte) (admission.PermissionRequest, error) {
	parsed, err := publicadmission.DecodePermissionRequest(raw)
	return admission.PermissionRequest{AuthorityKey: parsed.AuthorityKey,
		Role: admission.AllocationRole(parsed.Role), Permission: admission.Permission(parsed.Permission), HolderProof: parsed.HolderProof}, err
}
