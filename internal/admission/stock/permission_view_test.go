//go:build linux

package stock_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/attempts"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

type permissionHost struct {
	profile state.ClosedProfileView
	now     time.Time
}

func (host permissionHost) ProfileLocked() (state.ClosedProfileView, time.Time, error) {
	return host.profile, host.now, nil
}
func (permissionHost) SurfaceRole() admission.AllocationRole { return admission.AllocationUser }
func (permissionHost) Fail(error)                            {}
func (permissionHost) Journal() (*attempts.Journal, error) {
	return nil, errors.New("permission observation test has no spend authority")
}
func (permissionHost) LeaseContext() context.Context { return context.Background() }
func (permissionHost) SelectBootstrapLocked() (client.ClosedBootstrapSelection, error) {
	return client.ClosedBootstrapSelection{}, errors.New("permission observation test has no Source")
}
func (permissionHost) PrefixCurrent(stock.Prefix) bool { return false }

func TestPermissionObservationCannotOverwriteOrRestoreOwner(t *testing.T) {
	public, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(authority)
	now := time.Now().UTC()
	profile := state.ClosedProfileView{NetworkID: [32]byte{1}, IssuerNodeID: [32]byte{2}, IssuerDutyGeneration: 3,
		NotBefore: now.Truncate(time.Hour), NotAfter: now.Truncate(time.Hour).Add(time.Hour)}
	copy(profile.IssuanceAuthorityKey[:], public)
	var owner stock.Owner
	owner.Init(&sync.Mutex{}, permissionHost{profile: profile, now: now})
	raw, digest, err := owner.Request([3]uint32{1, 2, 0})
	if err != nil {
		t.Fatal(err)
	}
	request, err := admission.DecodePermissionRequest(raw)
	if err != nil {
		t.Fatal(err)
	}
	grant := request.Permission
	copy(grant.Signature[:], ed25519.Sign(authority, admission.PermissionTranscript(grant)))
	signed, err := admission.EncodePermission(grant)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Import(digest, signed); err != nil {
		t.Fatal(err)
	}
	// The explicit value type is intentional: a returned aggregate pointer
	// would let an external caller replace the owner's entire private struct.
	var observed stock.Permission = owner.PermissionLocked()
	saved := observed
	observed = stock.Permission{}
	if observed.Present() || owner.PermissionLocked() != saved || !saved.HasAccepted() {
		t.Fatal("overwriting an observation changed the retained permission")
	}
	copyOfGrant := saved.Grant()
	copyOfGrant.Maxima[1] = 999
	if saved.Remaining(2) != 2 || owner.PermissionLocked().Grant() != grant {
		t.Fatal("public grant copy changed allocation")
	}
	owner.StopLocked().Join()
	observed = saved
	if owner.PermissionLocked().Present() || observed.HasAccepted() || observed.CurrentFor(profile, now) {
		t.Fatal("retained observation restored retired authority")
	}
	fresh, _, err := owner.Request([3]uint32{1, 2, 0})
	if err != nil || bytes.Equal(raw, fresh) || owner.PermissionLocked() == observed {
		t.Fatal("fresh request revived retired permission")
	}
	owner.StopLocked().Join()
}
