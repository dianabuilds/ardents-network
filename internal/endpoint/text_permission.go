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
	window := now.Truncate(time.Hour)
	if window.Before(profile.NotBefore) || window.Add(time.Hour).After(profile.NotAfter) {
		return nil, [32]byte{}, errors.New("text permission hour is outside current authority")
	}
	role, limit := credential.AllocationUser, uint64(4096)
	if owner.surface == broker.Administration {
		role, limit = credential.AllocationPublisher, 16384
	}
	total := uint64(maxima[0]) + uint64(maxima[1]) + uint64(maxima[2])
	if total == 0 || total > limit {
		return nil, [32]byte{}, errors.New("text permission allocation is invalid")
	}
	if previous := owner.permission; previous != nil {
		if window.Before(previous.request.Permission.NotAfter) {
			if previous.profile != profile || previous.request.Permission.NotBefore != window || previous.request.Permission.Maxima != maxima {
				return nil, [32]byte{}, errors.New("text permission request cannot be replaced in its hour")
			}
			return bytes.Clone(previous.public), previous.digest, nil
		}
		owner.clearTextPermissionLocked()
	}
	request, holder, err := credential.PreparePermissionRequest(profile.IssuanceAuthorityKey, profile.NetworkID, profile.IssuerNodeID,
		profile.IssuerDutyGeneration, role, window, maxima)
	if err != nil {
		return nil, [32]byte{}, err
	}
	public, err := credential.EncodePermissionRequest(request)
	if err != nil {
		clear(holder)
		return nil, [32]byte{}, err
	}
	owner.permission = &textPermission{profile: profile, request: request, public: public, digest: sha256.Sum256(public), holder: holder}
	return bytes.Clone(public), owner.permission.digest, nil
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
	pending := owner.permission
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
	if owner.permission != nil {
		if owner.issuance != nil {
			owner.issuance.cancel()
		}
		if owner.permission.pending != nil {
			owner.permission.pending.pending.Discard()
		}
		for _, stock := range owner.permission.stock {
			for _, token := range stock.tokens {
				clear(token)
			}
		}
		clear(owner.permission.holder)
		clear(owner.permission.public)
		*owner.permission = textPermission{}
		owner.permission = nil
	}
}
