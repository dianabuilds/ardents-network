//go:build linux

package stock

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/attempts"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/client"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// This fixture isolates planning over already accepted in-memory permission.
// Real Custody, State selection and transport remain covered by Endpoint tests.
type stockHostFixture struct {
	profile state.ClosedProfileView
	now     time.Time
}

func (host *stockHostFixture) ProfileLocked() (state.ClosedProfileView, time.Time, error) {
	return host.profile, host.now, nil
}
func (*stockHostFixture) SurfaceRole() admission.AllocationRole { return admission.AllocationUser }
func (*stockHostFixture) Fail(error)                            {}
func (*stockHostFixture) Journal() (*attempts.Journal, error) {
	return nil, errors.New("planning fixture has no spend authority")
}
func (*stockHostFixture) LeaseContext() context.Context { return context.Background() }
func (*stockHostFixture) SelectBootstrapLocked() (client.ClosedBootstrapSelection, error) {
	return client.ClosedBootstrapSelection{}, errors.New("planning fixture has no Source selection")
}
func (*stockHostFixture) PrefixCurrent(Prefix) bool { return false }

func refillOwnerFixture() (*Owner, *stockHostFixture) {
	window := time.Date(2026, time.October, 3, 10, 0, 0, 0, time.UTC)
	host := &stockHostFixture{now: window.Add(time.Minute), profile: state.ClosedProfileView{
		Digest: fixtureID(1), IssuerNodeID: fixtureID(2), IssuerDutyGeneration: 7,
	}}
	owner := &Owner{permission: &permission{profile: host.profile, accepted: admission.Permission{
		NotBefore: window, NotAfter: window.Add(time.Hour), Signature: [64]byte{1},
		Maxima: [3]uint32{40, 4, 0},
	}}}
	owner.Init(&sync.Mutex{}, host)
	return owner, host
}

func TestRefillPlanPreservesControlAdmissionCostAndAllocation(t *testing.T) {
	for _, test := range []struct {
		name      string
		ready     int
		remaining uint32
		prefix    bool
		want      int
	}{
		{"cold bootstrap is capped at 32", 0, 40, false, 32},
		{"cold bootstrap honors remaining allocation", 0, 3, false, 3},
		{"no allocation cannot refill", 0, 0, false, 0},
		{"live prefix cannot refill without admission token", 0, 40, true, 0},
		{"one remaining grant cannot increase live stock", 1, 1, true, 0},
		{"one token funds bounded live refill", 1, 2, true, 2},
		{"two available tokens defer refill", 2, 40, true, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			owner, host := refillOwnerFixture()
			permission := owner.permission
			permission.reserved[0] = 40 - test.remaining
			permission.stock = []stockEntry{{Challenge: credential.ClosedTokenContext{
				ProfileDigest: host.profile.Digest, ReceiverNodeID: host.profile.IssuerNodeID,
				ReceiverDutyGeneration: 7, Class: 1, WindowStart: permission.accepted.NotBefore,
			}, Tokens: make([][]byte, test.ready)}}
			reserved := permission.reserved
			receivers, err := owner.RefillPlanLocked([][32]byte{fixtureID(3)}, 2, test.prefix)
			if err != nil || len(receivers) != test.want || permission.reserved != reserved || permission.HasPending() {
				t.Fatalf("plan=%d want=%d reservation=%v err=%v", len(receivers), test.want, permission.reserved, err)
			}
			for _, receiver := range receivers {
				if receiver != host.profile.IssuerNodeID {
					t.Fatal("refill planned a foreign receiving duty")
				}
			}
		})
	}
}

func TestRefillPlanResumesRetainedStageBeforeSelfRequest(t *testing.T) {
	owner, host := refillOwnerFixture()
	batch := &batch{Refill: true, Challenges: []credential.ClosedTokenContext{
		{ReceiverNodeID: host.profile.IssuerNodeID, Class: 1},
		{ReceiverNodeID: host.profile.IssuerNodeID, Class: 1},
	}}
	owner.permission.pending = batch
	receivers, err := owner.RefillPlanLocked([][32]byte{host.profile.IssuerNodeID}, 1, true)
	if err != nil || len(receivers) != 2 {
		t.Fatalf("retained refill skipped: %v %v", receivers, err)
	}
	receivers[0][0] ^= 1
	if batch.Challenges[0].ReceiverNodeID != host.profile.IssuerNodeID {
		t.Fatal("caller changed retained refill through returned intent")
	}
	batch.Challenges[0].Class = 2
	if _, err := owner.RefillPlanLocked(nil, 2, true); err == nil {
		t.Fatal("malformed retained refill was replaced or accepted")
	}
	batch.Refill = false
	if receivers, err := owner.RefillPlanLocked(nil, 2, true); err != nil || len(receivers) != 0 || owner.permission.pending != batch {
		t.Fatal("ordinary pending batch was replaced by refill")
	}
}
