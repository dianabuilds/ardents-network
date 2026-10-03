//go:build linux

package tokens

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/attempts"
	"github.com/dianabuilds/ardents-network/internal/endpoint/permissionfile"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

// Owner is the duty context's token authority: one retained permission plus
// the single admitted issuance operation slot. The endpoint package embeds it
// by value and binds it to the shared duty context lock and Host seam with
// Init before any authority use.
type Owner struct {
	mu         *sync.Mutex
	host       Host
	Permission *Permission
	Issuance   *Operation
}

func (owner *Owner) Init(mu *sync.Mutex, host Host) {
	owner.mu = mu
	owner.host = host
}

// Request returns only the public holder-signed request and its
// exact approval digest. A repeat in the same hour returns the existing request;
// changing the allocation/profile cannot silently create a replacement holder.
func (owner *Owner) Request(maxima [3]uint32) ([]byte, [32]byte, error) {
	if owner == nil || owner.host == nil {
		return nil, [32]byte{}, errors.New("text permission context is unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.host.ProfileLocked()
	if err != nil {
		return nil, [32]byte{}, err
	}
	scope, err := newRequestScope(profile, now, owner.host.SurfaceRole(), maxima)
	if err != nil {
		return nil, [32]byte{}, err
	}
	if previous := owner.Permission; previous != nil {
		public, digest, active, err := previous.retainedRequest(scope)
		if active {
			return public, digest, err
		}
		owner.ClearPermissionLocked()
	}
	prepared, public, digest, err := preparePermission(scope)
	if err != nil {
		return nil, [32]byte{}, err
	}
	owner.Permission = prepared
	return public, digest, nil
}

// Import verifies signed bytes against this same live public
// request. The digest selects an outstanding request; it supplies no authority.
// A foreign holder, changed allocation, old profile/hour or revoked context
// cannot acquire a usable permission even with a valid authority signature.
func (owner *Owner) Import(digest [32]byte, raw []byte) error {
	if owner == nil || owner.host == nil {
		return errors.New("text permission context is unavailable")
	}
	permission, err := admission.DecodePermission(raw)
	if err != nil {
		return err
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.host.ProfileLocked()
	if err != nil {
		return err
	}
	return owner.Permission.acceptResponse(profile, now, digest, permission)
}

func (owner *Owner) ClearPermissionLocked() {
	permission := owner.Permission
	if permission == nil {
		return
	}
	owner.Permission = nil
	if owner.Issuance != nil && owner.Issuance.retirePermissionLocked(permission) {
		return
	}
	clearPermission(permission)
}

// TakeTokenLocked is shared only after the exact opening or issuance
// flight has independently authorized its role. It durably marks consumed stock
// and rechecks the surviving context before releasing bytes to Route.
func (owner *Owner) TakeTokenLocked(profile state.ClosedProfileView, now time.Time, hello ardp.Hello, class uint8, attempt context.Context) ([]byte, error) {
	permission := owner.Permission
	token, err := permission.consumeToken(profile, now, hello, class)
	if err != nil {
		return nil, err
	}
	journal, err := owner.host.Journal()
	if err == nil {
		err = journal.Mark(token, attempts.Attempt{Profile: profile.Digest, Receiver: hello.RecipientNodeID, Duty: hello.RecipientDutyGeneration,
			Window: permission.Accepted.NotBefore, Class: class, Nonce: hello.ChannelNonce})
	}
	if err != nil {
		clear(token)
		owner.host.Fail(err)
		return nil, TransferFailureAt("journal", err)
	}
	currentProfile, currentTime, currentErr := owner.host.ProfileLocked()
	if currentErr != nil || currentProfile != profile || !currentTime.Before(permission.Accepted.NotAfter) || attempt.Err() != nil {
		clear(token)
		return nil, TransferFailureAt("owner", errors.Join(currentErr, attempt.Err(), errors.New("text token owner changed after durable mark")))
	}
	return token, nil
}

// The trusted Endpoint composition supplies these owner-only paths. Neither
// Application transport accepts a path, permission, holder key or context ID.
// ExportFile returns the public commitment separately for explicit Custody approval.
func (owner *Owner) ExportFile(ctx context.Context, path string, maxima [3]uint32) (commitment [32]byte, outcome error) {
	if ctx == nil || ctx.Err() != nil {
		return [32]byte{}, errors.New("text permission export canceled")
	}
	public, digest, err := owner.Request(maxima)
	if err != nil {
		return [32]byte{}, err
	}
	defer clear(public)
	file, err := permissionfile.Open(path)
	if err != nil {
		return [32]byte{}, err
	}
	defer func() { outcome = errors.Join(outcome, file.Close()) }()
	if err := file.PublishRequest(public); err != nil {
		return [32]byte{}, err
	}
	// Exporting public bytes grants nothing. Recheck without generating another
	// holder if the hour changed while the file operation was in progress.
	if err := ctx.Err(); err != nil {
		return [32]byte{}, err
	}
	owner.mu.Lock()
	profile, now, err := owner.host.ProfileLocked()
	pending := owner.Permission
	current := pending != nil && pending.Digest == digest && pending.Profile == profile &&
		!now.Before(pending.Request.Permission.NotBefore) && now.Before(pending.Request.Permission.NotAfter)
	owner.mu.Unlock()
	if err != nil || !current {
		return [32]byte{}, errors.New("text permission export context changed")
	}
	return digest, nil
}

// ImportFile consumes the existing Custody permission format and the separately
// retained request digest. Files are transport, never authority or context
// restoration: the in-memory owner performs all currentness checks again.
func (owner *Owner) ImportFile(ctx context.Context, path string, digest [32]byte) (outcome error) {
	if owner == nil || owner.host == nil || ctx == nil || ctx.Err() != nil {
		return errors.New("text permission import canceled")
	}
	owner.mu.Lock()
	_, _, err := owner.host.ProfileLocked()
	matches := owner.Permission.matchesRequest(digest)
	owner.mu.Unlock()
	if err != nil || !matches {
		return errors.New("text permission import has no live request")
	}
	file, err := permissionfile.Open(path)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, file.Close()) }()
	raw, err := file.ReadResponse()
	if err != nil {
		return err
	}
	defer clear(raw)
	if err := ctx.Err(); err != nil {
		return err
	}
	return owner.Import(digest, raw)
}
