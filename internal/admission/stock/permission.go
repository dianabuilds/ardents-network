//go:build linux

package stock

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/allocation"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// permission keeps exactly one holder/hour in its authorized context.
// Worker retirement does not erase it; context or Endpoint loss does. No key,
// local scope identifier or permission bytes are put on the worker attachment.
type permission struct {
	batches  uint8
	reserved [3]uint32
	pending  *batch
	stock    []stockEntry
	profile  state.ClosedProfileView
	request  admission.PermissionRequest
	public   []byte
	digest   [32]byte
	holder   ed25519.PrivateKey
	accepted admission.Permission
}

// Grant returns a copy of the public signed allocation. Changing that value
// cannot change this holder's accepted permission or reserved balance.
func (permission *permission) Grant() admission.Permission {
	if permission == nil {
		return admission.Permission{}
	}
	return permission.accepted
}

// RequestExpiry bounds the offline handover for this exact public request.
// A digest names a retained request; it cannot confer approval.
func (permission *permission) RequestExpiry(digest [32]byte) (time.Time, bool) {
	if !permission.matchesRequest(digest) {
		return time.Time{}, false
	}
	return permission.request.Permission.NotAfter, true
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

// These predicates keep Context coordinators from interpreting retained
// permission state. Callers still hold the Context admission lock.
func (permission *permission) HasAccepted() bool {
	return permission != nil && permission.accepted != (admission.Permission{})
}

func (permission *permission) HasPending() bool {
	return permission != nil && permission.pending != nil
}

func (permission *permission) PendingFor(prefix Prefix) bool {
	return permission.HasPending() && permission.pending.Prefix == prefix
}

func (permission *permission) matchesRequest(digest [32]byte) bool {
	return permission != nil && digest != [32]byte{} && permission.digest == digest
}

// CurrentFor is the permission owner's live authority check. Callers still
// recheck their Context, job, and State authority under the Context lock.
func (permission *permission) CurrentFor(profile state.ClosedProfileView, now time.Time) bool {
	return permission != nil && permission.profile == profile &&
		permission.accepted.Signature != [64]byte{} &&
		!now.Before(permission.accepted.NotBefore) && now.Before(permission.accepted.NotAfter)
}

// requestScope is the exact current authority and allocation
// selected by the Context before the permission owner creates or reuses a key.
type requestScope struct {
	profile state.ClosedProfileView
	window  time.Time
	role    admission.AllocationRole
	maxima  [3]uint32
}

func newRequestScope(profile state.ClosedProfileView, now time.Time,
	role admission.AllocationRole, maxima [3]uint32) (requestScope, error) {
	window := now.Truncate(time.Hour)
	if window.Before(profile.NotBefore) || window.Add(time.Hour).After(profile.NotAfter) {
		return requestScope{}, errors.New("text permission hour is outside current authority")
	}
	limit := allocation.RoleLimit(role)
	total := uint64(maxima[0]) + uint64(maxima[1]) + uint64(maxima[2])
	if total == 0 || total > limit {
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
func (pending *permission) acceptResponse(profile state.ClosedProfileView, now time.Time,
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
