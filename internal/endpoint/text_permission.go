//go:build linux

package endpoint

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// textPermission keeps exactly one holder/hour in its authorized context.
// Worker retirement does not erase it; context or Endpoint loss does. No key,
// local scope identifier or permission bytes are put on the worker attachment.
type textPermission struct {
	batches  uint8
	reserved [3]uint32
	pending  *textTokenBatch
	stock    []textTokenStock
	profile  state.ClosedProfileView
	request  credential.PermissionRequest
	public   []byte
	digest   [32]byte
	holder   ed25519.PrivateKey
	accepted credential.Permission
}

// textPermissionRequestScope is the exact current authority and allocation
// selected by the Context before the permission owner creates or reuses a key.
type textPermissionRequestScope struct {
	profile state.ClosedProfileView
	window  time.Time
	role    credential.AllocationRole
	maxima  [3]uint32
}

func newTextPermissionRequestScope(profile state.ClosedProfileView, now time.Time,
	role credential.AllocationRole, maxima [3]uint32) (textPermissionRequestScope, error) {
	window := now.Truncate(time.Hour)
	if window.Before(profile.NotBefore) || window.Add(time.Hour).After(profile.NotAfter) {
		return textPermissionRequestScope{}, errors.New("text permission hour is outside current authority")
	}
	limit := uint64(4096)
	if role == credential.AllocationPublisher {
		limit = 16384
	}
	total := uint64(maxima[0]) + uint64(maxima[1]) + uint64(maxima[2])
	if total == 0 || total > limit {
		return textPermissionRequestScope{}, errors.New("text permission allocation is invalid")
	}
	return textPermissionRequestScope{profile: profile, window: window, role: role, maxima: maxima}, nil
}

// retainedRequest returns an exact same-hour request without exposing its
// retained public backing bytes. An expired owner leaves the caller free to
// retire it before creating a fresh holder.
func (permission *textPermission) retainedRequest(scope textPermissionRequestScope) ([]byte, [32]byte, bool, error) {
	if !scope.window.Before(permission.request.Permission.NotAfter) {
		return nil, [32]byte{}, false, nil
	}
	if permission.profile != scope.profile || permission.request.Permission.NotBefore != scope.window ||
		permission.request.Permission.Maxima != scope.maxima {
		return nil, [32]byte{}, true, errors.New("text permission request cannot be replaced in its hour")
	}
	return bytes.Clone(permission.public), permission.digest, true, nil
}

func prepareTextPermission(scope textPermissionRequestScope) (*textPermission, []byte, [32]byte, error) {
	request, holder, err := credential.PreparePermissionRequest(scope.profile.IssuanceAuthorityKey,
		scope.profile.NetworkID, scope.profile.IssuerNodeID, scope.profile.IssuerDutyGeneration,
		scope.role, scope.window, scope.maxima)
	if err != nil {
		return nil, nil, [32]byte{}, err
	}
	public, err := credential.EncodePermissionRequest(request)
	if err != nil {
		clear(holder)
		return nil, nil, [32]byte{}, err
	}
	permission := &textPermission{profile: scope.profile, request: request, public: public,
		digest: sha256.Sum256(public), holder: holder}
	return permission, bytes.Clone(public), permission.digest, nil
}

// acceptResponse verifies the signed response against this owner's exact
// retained request and only then changes its accepted permission.
func (pending *textPermission) acceptResponse(profile state.ClosedProfileView, now time.Time,
	digest [32]byte, permission credential.Permission) error {
	if pending == nil || digest == [32]byte{} || pending.digest != digest || pending.profile != profile {
		return errors.New("text permission response has no matching live request")
	}
	expected := pending.request.Permission
	expected.Signature = permission.Signature
	if permission != expected || credential.VerifyPermission(permission, ed25519.PublicKey(profile.IssuanceAuthorityKey[:]),
		profile.NetworkID, profile.IssuerNodeID, profile.IssuerDutyGeneration, now) != nil {
		return errors.New("text permission response does not match its approved request")
	}
	if pending.accepted != (credential.Permission{}) && pending.accepted != permission {
		return errors.New("text permission response conflicts")
	}
	pending.accepted = permission
	return nil
}

// requestTextPermission returns only the public holder-signed request and its
// exact approval digest. A repeat in the same hour returns the existing request;
// changing the allocation/profile cannot silently create a replacement holder.
func (owner *textContext) requestTextPermission(maxima [3]uint32) ([]byte, [32]byte, error) {
	if owner == nil {
		return nil, [32]byte{}, errors.New("text permission context is unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.textPermissionProfileLocked()
	if err != nil {
		return nil, [32]byte{}, err
	}
	role := credential.AllocationUser
	if owner.surface == broker.Administration {
		role = credential.AllocationPublisher
	}
	scope, err := newTextPermissionRequestScope(profile, now, role, maxima)
	if err != nil {
		return nil, [32]byte{}, err
	}
	if previous := owner.permission; previous != nil {
		public, digest, active, err := previous.retainedRequest(scope)
		if active {
			return public, digest, err
		}
		owner.clearTextPermissionLocked()
	}
	prepared, public, digest, err := prepareTextPermission(scope)
	if err != nil {
		return nil, [32]byte{}, err
	}
	owner.permission = prepared
	return public, digest, nil
}

// importTextPermission verifies signed bytes against this same live public
// request. The digest selects an outstanding request; it supplies no authority.
// A foreign holder, changed allocation, old profile/hour or revoked context
// cannot acquire a usable permission even with a valid authority signature.
func (owner *textContext) importTextPermission(digest [32]byte, raw []byte) error {
	if owner == nil {
		return errors.New("text permission context is unavailable")
	}
	permission, err := credential.DecodePermission(raw)
	if err != nil {
		return err
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.textPermissionProfileLocked()
	if err != nil {
		return err
	}
	return owner.permission.acceptResponse(profile, now, digest, permission)
}

func (owner *textContext) textPermissionProfileLocked() (state.ClosedProfileView, time.Time, error) {
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

func (owner *textContext) clearTextPermissionLocked() {
	permission := owner.permission
	if permission == nil {
		return
	}
	owner.permission = nil
	if owner.issuance != nil && owner.issuance.retirePermissionLocked(permission) {
		return
	}
	clearTextPermission(permission)
}

func clearTextPermission(permission *textPermission) {
	if permission.pending != nil {
		permission.pending.pending.Discard()
	}
	for _, stock := range permission.stock {
		for _, token := range stock.tokens {
			clear(token)
		}
	}
	clear(permission.holder)
	clear(permission.public)
	*permission = textPermission{}
}
