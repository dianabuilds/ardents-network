package stock

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/allocation"
)

// permission keeps exactly one holder/hour until its Owner revokes it.
// The caller never receives the holder key or mutable retained state.
type permission struct {
	batches  uint8
	reserved [3]uint32
	pending  *batch
	stock    []stockEntry
	profile  admission.AuthorityFacts
	request  admission.PermissionRequest
	public   []byte
	digest   [32]byte
	holder   ed25519.PrivateKey
	accepted admission.Permission
}

// BootstrapAllowance is the number of fresh bootstrap batches left for this
// permission. Exact retry does not consume another batch; retirement and failed
// transport do not refund the allocation.
func (permission *permission) BootstrapAllowance() uint8 {
	if permission == nil || permission.batches >= 2 {
		return 0
	}
	return 2 - permission.batches
}

// These predicates inspect retained state under the Owner lock.
func (permission *permission) HasAccepted() bool {
	return permission != nil && permission.accepted != (admission.Permission{})
}

func (permission *permission) HasPending() bool {
	return permission != nil && permission.pending != nil
}

// CurrentFor binds retained permission to freshly supplied authority facts.
func (permission *permission) CurrentFor(profile admission.AuthorityFacts, now time.Time) bool {
	return permission != nil && permission.profile == profile &&
		permission.accepted.Signature != [64]byte{} &&
		!now.Before(permission.accepted.NotBefore) && now.Before(permission.accepted.NotAfter)
}

// requestScope is the exact current authority and allocation
// selected by the application before the permission owner creates or reuses a key.
type requestScope struct {
	profile admission.AuthorityFacts
	window  time.Time
	role    admission.AllocationRole
	maxima  [3]uint32
}

func newRequestScope(profile admission.AuthorityFacts, now time.Time,
	role admission.AllocationRole, maxima [3]uint32) (requestScope, error) {
	window := now.Truncate(time.Hour)
	if window.Before(profile.NotBefore) || window.Add(time.Hour).After(profile.NotAfter) {
		return requestScope{}, errors.New("text permission hour is outside current authority")
	}
	validationErr := allocation.ValidateMaxima(role, maxima)

	if validationErr != nil {
		return requestScope{}, errors.New("text permission allocation is invalid")
	}
	return requestScope{profile: profile, window: window, role: role, maxima: maxima}, nil
}

// retainedRequest returns an exact same-hour request without exposing its
// retained public backing bytes. An expired owner leaves the caller free to
// retire it before creating a fresh holder.
func (permission *permission) retainedRequest(scope requestScope) ([]byte, [32]byte, bool, error) {
	if !scope.window.Before(permission.request.Permission.NotAfter) {
		return nil, [32]byte{}, false, nil
	}
	if permission.profile != scope.profile || permission.request.Permission.NotBefore != scope.window ||
		permission.request.Permission.Maxima != scope.maxima {
		return nil, [32]byte{}, true, errors.New("text permission request cannot be replaced in its hour")
	}
	return bytes.Clone(permission.public), permission.digest, true, nil
}

func preparePermission(scope requestScope) (*permission, []byte, [32]byte, error) {
	request, holder, err := admission.PreparePermissionRequest(scope.profile.IssuanceAuthorityKey,
		scope.profile.NetworkID, scope.profile.IssuerNodeID, scope.profile.IssuerDutyGeneration,
		scope.role, scope.window, scope.maxima)
	if err != nil {
		return nil, nil, [32]byte{}, err
	}
	public, err := admission.EncodePermissionRequest(request)
	if err != nil {
		clear(holder)
		return nil, nil, [32]byte{}, err
	}
	permission := &permission{profile: scope.profile, request: request, public: public,
		digest: sha256.Sum256(public), holder: holder}
	return permission, bytes.Clone(public), permission.digest, nil
}

// acceptResponse verifies the signed response against this owner's exact
// retained request and only then changes its accepted permission.
func (pending *permission) acceptResponse(profile admission.AuthorityFacts, now time.Time,
	digest [32]byte, permission admission.Permission) error {
	if pending == nil || digest == [32]byte{} || pending.digest != digest || pending.profile != profile {
		return errors.New("text permission response has no matching live request")
	}
	expected := pending.request.Permission
	expected.Signature = permission.Signature
	if permission != expected || admission.VerifyPermission(permission, ed25519.PublicKey(profile.IssuanceAuthorityKey[:]),
		profile.NetworkID, profile.IssuerNodeID, profile.IssuerDutyGeneration, now) != nil {
		return errors.New("text permission response does not match its approved request")
	}
	if pending.accepted != (admission.Permission{}) && pending.accepted != permission {
		return errors.New("text permission response conflicts")
	}
	pending.accepted = permission
	return nil
}

func clearPermission(target *permission) {
	if target.pending != nil {
		target.pending.Pending.Discard()
	}
	for _, stock := range target.stock {
		for _, token := range stock.Tokens {
			clear(token)
		}
	}
	clear(target.holder)
	clear(target.public)
	*target = permission{}
}
