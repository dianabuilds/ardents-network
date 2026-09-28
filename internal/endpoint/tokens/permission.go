//go:build linux

package tokens

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// Permission keeps exactly one holder/hour in its authorized context.
// Worker retirement does not erase it; context or Endpoint loss does. No key,
// local scope identifier or permission bytes are put on the worker attachment.
type Permission struct {
	Batches  uint8
	Reserved [3]uint32
	Pending  *Batch
	Stock    []Stock
	Profile  state.ClosedProfileView
	Request  admission.PermissionRequest
	Public   []byte
	Digest   [32]byte
	Holder   ed25519.PrivateKey
	Accepted admission.Permission
}

// These predicates keep Context coordinators from interpreting retained
// permission state. Callers still hold the Context admission lock.
func (permission *Permission) HasAccepted() bool {
	return permission != nil && permission.Accepted != (admission.Permission{})
}

func (permission *Permission) HasPending() bool {
	return permission != nil && permission.Pending != nil
}

func (permission *Permission) PendingFor(prefix Prefix) bool {
	return permission.HasPending() && permission.Pending.Prefix == prefix
}

func (permission *Permission) matchesRequest(digest [32]byte) bool {
	return permission != nil && digest != [32]byte{} && permission.Digest == digest
}

// CurrentFor is the permission owner's live authority check. Callers still
// recheck their Context, job, and State authority under the Context lock.
func (permission *Permission) CurrentFor(profile state.ClosedProfileView, now time.Time) bool {
	return permission != nil && permission.Profile == profile &&
		permission.Accepted.Signature != [64]byte{} &&
		!now.Before(permission.Accepted.NotBefore) && now.Before(permission.Accepted.NotAfter)
}

// RequestScope is the exact current authority and allocation
// selected by the Context before the permission owner creates or reuses a key.
type RequestScope struct {
	profile state.ClosedProfileView
	window  time.Time
	role    admission.AllocationRole
	maxima  [3]uint32
}

func newRequestScope(profile state.ClosedProfileView, now time.Time,
	role admission.AllocationRole, maxima [3]uint32) (RequestScope, error) {
	window := now.Truncate(time.Hour)
	if window.Before(profile.NotBefore) || window.Add(time.Hour).After(profile.NotAfter) {
		return RequestScope{}, errors.New("text permission hour is outside current authority")
	}
	limit := uint64(4096)
	if role == admission.AllocationPublisher {
		limit = 16384
	}
	total := uint64(maxima[0]) + uint64(maxima[1]) + uint64(maxima[2])
	if total == 0 || total > limit {
		return RequestScope{}, errors.New("text permission allocation is invalid")
	}
	return RequestScope{profile: profile, window: window, role: role, maxima: maxima}, nil
}

// retainedRequest returns an exact same-hour request without exposing its
// retained public backing bytes. An expired owner leaves the caller free to
// retire it before creating a fresh holder.
func (permission *Permission) retainedRequest(scope RequestScope) ([]byte, [32]byte, bool, error) {
	if !scope.window.Before(permission.Request.Permission.NotAfter) {
		return nil, [32]byte{}, false, nil
	}
	if permission.Profile != scope.profile || permission.Request.Permission.NotBefore != scope.window ||
		permission.Request.Permission.Maxima != scope.maxima {
		return nil, [32]byte{}, true, errors.New("text permission request cannot be replaced in its hour")
	}
	return bytes.Clone(permission.Public), permission.Digest, true, nil
}

func preparePermission(scope RequestScope) (*Permission, []byte, [32]byte, error) {
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
	permission := &Permission{Profile: scope.profile, Request: request, Public: public,
		Digest: sha256.Sum256(public), Holder: holder}
	return permission, bytes.Clone(public), permission.Digest, nil
}

// acceptResponse verifies the signed response against this owner's exact
// retained request and only then changes its accepted permission.
func (pending *Permission) acceptResponse(profile state.ClosedProfileView, now time.Time,
	digest [32]byte, permission admission.Permission) error {
	if pending == nil || digest == [32]byte{} || pending.Digest != digest || pending.Profile != profile {
		return errors.New("text permission response has no matching live request")
	}
	expected := pending.Request.Permission
	expected.Signature = permission.Signature
	if permission != expected || admission.VerifyPermission(permission, ed25519.PublicKey(profile.IssuanceAuthorityKey[:]),
		profile.NetworkID, profile.IssuerNodeID, profile.IssuerDutyGeneration, now) != nil {
		return errors.New("text permission response does not match its approved request")
	}
	if pending.Accepted != (admission.Permission{}) && pending.Accepted != permission {
		return errors.New("text permission response conflicts")
	}
	pending.Accepted = permission
	return nil
}

func clearPermission(target *Permission) {
	if target.Pending != nil {
		target.Pending.Pending.Discard()
	}
	for _, stock := range target.Stock {
		for _, token := range stock.Tokens {
			clear(token)
		}
	}
	clear(target.Holder)
	clear(target.Public)
	*target = Permission{}
}
