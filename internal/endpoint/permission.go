//go:build linux

package endpoint

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// permission keeps exactly one holder/hour in its authorized context.
// Worker retirement does not erase it; context or Endpoint loss does. No key,
// local scope identifier or permission bytes are put on the worker attachment.
type permission struct {
	batches  uint8
	reserved [3]uint32
	pending  *tokenBatch
	stock    []tokenStock
	profile  state.ClosedProfileView
	request  admission.PermissionRequest
	public   []byte
	digest   [32]byte
	holder   ed25519.PrivateKey
	accepted admission.Permission
}

// These predicates keep Context coordinators from interpreting retained
// permission state. Callers still hold the Context admission lock.
func (permission *permission) hasAccepted() bool {
	return permission != nil && permission.accepted != (admission.Permission{})
}

func (permission *permission) hasPending() bool {
	return permission != nil && permission.pending != nil
}

func (permission *permission) pendingFor(prefix *textSourceHandle) bool {
	return permission.hasPending() && permission.pending.prefix == prefix
}

func (permission *permission) matchesRequest(digest [32]byte) bool {
	return permission != nil && digest != [32]byte{} && permission.digest == digest
}

// currentFor is the permission owner's live authority check. Callers still
// recheck their Context, job, and State authority under the Context lock.
func (permission *permission) currentFor(profile state.ClosedProfileView, now time.Time) bool {
	return permission != nil && permission.profile == profile &&
		permission.accepted.Signature != [64]byte{} &&
		!now.Before(permission.accepted.NotBefore) && now.Before(permission.accepted.NotAfter)
}

// permissionRequestScope is the exact current authority and allocation
// selected by the Context before the permission owner creates or reuses a key.
type permissionRequestScope struct {
	profile state.ClosedProfileView
	window  time.Time
	role    admission.AllocationRole
	maxima  [3]uint32
}

func newPermissionRequestScope(profile state.ClosedProfileView, now time.Time,
	role admission.AllocationRole, maxima [3]uint32) (permissionRequestScope, error) {
	window := now.Truncate(time.Hour)
	if window.Before(profile.NotBefore) || window.Add(time.Hour).After(profile.NotAfter) {
		return permissionRequestScope{}, errors.New("text permission hour is outside current authority")
	}
	limit := uint64(4096)
	if role == admission.AllocationPublisher {
		limit = 16384
	}
	total := uint64(maxima[0]) + uint64(maxima[1]) + uint64(maxima[2])
	if total == 0 || total > limit {
		return permissionRequestScope{}, errors.New("text permission allocation is invalid")
	}
	return permissionRequestScope{profile: profile, window: window, role: role, maxima: maxima}, nil
}

// retainedRequest returns an exact same-hour request without exposing its
// retained public backing bytes. An expired owner leaves the caller free to
// retire it before creating a fresh holder.
func (permission *permission) retainedRequest(scope permissionRequestScope) ([]byte, [32]byte, bool, error) {
	if !scope.window.Before(permission.request.Permission.NotAfter) {
		return nil, [32]byte{}, false, nil
	}
	if permission.profile != scope.profile || permission.request.Permission.NotBefore != scope.window ||
		permission.request.Permission.Maxima != scope.maxima {
		return nil, [32]byte{}, true, errors.New("text permission request cannot be replaced in its hour")
	}
	return bytes.Clone(permission.public), permission.digest, true, nil
}

func preparePermission(scope permissionRequestScope) (*permission, []byte, [32]byte, error) {
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

// requestPermission returns only the public holder-signed request and its
// exact approval digest. A repeat in the same hour returns the existing request;
// changing the allocation/profile cannot silently create a replacement holder.
func (owner *textContext) requestPermission(maxima [3]uint32) ([]byte, [32]byte, error) {
	if owner == nil {
		return nil, [32]byte{}, errors.New("text permission context is unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.permissionProfileLocked()
	if err != nil {
		return nil, [32]byte{}, err
	}
	role := admission.AllocationUser
	if owner.surface == broker.Administration {
		role = admission.AllocationPublisher
	}
	scope, err := newPermissionRequestScope(profile, now, role, maxima)
	if err != nil {
		return nil, [32]byte{}, err
	}
	if previous := owner.tokens.permission; previous != nil {
		public, digest, active, err := previous.retainedRequest(scope)
		if active {
			return public, digest, err
		}
		owner.clearPermissionLocked()
	}
	prepared, public, digest, err := preparePermission(scope)
	if err != nil {
		return nil, [32]byte{}, err
	}
	owner.tokens.permission = prepared
	return public, digest, nil
}

// importPermission verifies signed bytes against this same live public
// request. The digest selects an outstanding request; it supplies no authority.
// A foreign holder, changed allocation, old profile/hour or revoked context
// cannot acquire a usable permission even with a valid authority signature.
func (owner *textContext) importPermission(digest [32]byte, raw []byte) error {
	if owner == nil {
		return errors.New("text permission context is unavailable")
	}
	permission, err := admission.DecodePermission(raw)
	if err != nil {
		return err
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.permissionProfileLocked()
	if err != nil {
		return err
	}
	return owner.tokens.permission.acceptResponse(profile, now, digest, permission)
}

func (owner *textContext) permissionProfileLocked() (state.ClosedProfileView, time.Time, error) {
	if err := owner.retireTextPrefixLocked(); err != nil {
		return state.ClosedProfileView{}, time.Time{}, err
	}
	endpoint := owner.endpoint
	if !owner.liveLocked(endpoint, owner.surface) || owner.verifiedJob == nil || owner.verifiedJob.owner != owner ||
		owner.verifiedJob.workerGrant == nil || endpoint.closedState == nil || endpoint.clock == nil {
		return state.ClosedProfileView{}, time.Time{}, errors.New("text permission requires a verified live Endpoint context")
	}
	now := endpoint.clock().UTC()
	profile, err := endpoint.closedState.CurrentClosedProfile()
	if err != nil || profile.NetworkID != endpoint.network || profile.NetworkID == [32]byte{} || profile.StateGeneration == [32]byte{} ||
		profile.StateDigest == [32]byte{} || profile.Digest == [32]byte{} || profile.IssuanceAuthorityKey == [32]byte{} ||
		profile.IssuerNodeID == [32]byte{} || profile.IssuerDutyGeneration == 0 || now.Before(profile.NotBefore) || !now.Before(profile.NotAfter) {
		return state.ClosedProfileView{}, time.Time{}, errors.New("text permission State authority is unavailable")
	}
	return profile, now, nil
}

func (owner *textContext) clearPermissionLocked() {
	permission := owner.tokens.permission
	if permission == nil {
		return
	}
	owner.tokens.permission = nil
	if owner.tokens.issuance != nil && owner.tokens.issuance.retirePermissionLocked(permission) {
		return
	}
	clearPermission(permission)
}

func clearPermission(target *permission) {
	if target.pending != nil {
		target.pending.pending.Discard()
	}
	for _, stock := range target.stock {
		for _, token := range stock.tokens {
			clear(token)
		}
	}
	clear(target.holder)
	clear(target.public)
	*target = permission{}
}
