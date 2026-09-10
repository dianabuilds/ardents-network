//go:build linux

package endpoint

import (
	"encoding/hex"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// Explicit State/qualified-launch fixtures isolate set ownership. The tests
// use the actual Entry and local-duty roots, not enrollment/host qualification.
type textSourceStateFixture struct {
	issuePermission    func(*testing.T, *textContext, [3]uint32)
	issueRawPermission func(*testing.T, []byte, [32]byte) []byte
	mu                 sync.Mutex
	view               state.ClosedRouteView
	snapshot           state.Snapshot
}

func (source *textSourceStateFixture) CurrentClosedProfile() (state.ClosedProfileView, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	return source.view.Profile, nil
}
func (source *textSourceStateFixture) CurrentClosedRoute() (state.ClosedRouteView, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	return source.view, nil
}
func (source *textSourceStateFixture) Current() (state.Snapshot, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	return source.snapshot, nil
}

func textSourceContextFixture(t *testing.T) (*endpoint, *textContext, *textSourceStateFixture) {
	t.Helper()
	now := time.Now().UTC()
	window := now.Truncate(time.Hour)
	source := &textSourceStateFixture{}
	// The selected State outlives bounded registrations even near the hour
	// boundary; holder permissions still keep their exact one-hour window.
	profile := state.ClosedProfileView{NetworkID: fixtureID(222), StateGeneration: fixtureID(223), StateDigest: fixtureID(224), Digest: fixtureID(225),
		IssuerNodeID: fixtureID(14), IssuerDutyGeneration: 5, IssuanceAuthorityKey: fixtureID(226), Epoch: 1, NotBefore: window, NotAfter: window.Add(2 * time.Hour)}
	source.view = state.ClosedRouteView{Profile: profile, NodeCount: 5}
	source.snapshot = state.Snapshot{Generation: hex.EncodeToString(profile.StateGeneration[:]), NetworkID: profile.NetworkID, Epoch: profile.Epoch, Digest: profile.StateDigest,
		EpochValidFrom: profile.NotBefore, ValidUntil: profile.NotAfter, Profile: route.ClosedRouteProfile, Freshness: "fresh", CandidateCount: 5}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	unused := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 5; index++ {
		domain, subrole := uint8(1), uint8(1)
		if index >= 2 {
			subrole = 2
		}
		if index == 4 {
			domain, subrole = 2, 6
		}
		id, record := fixtureID(byte(10+index)), fixtureID(byte(30+index))
		source.view.Nodes[index] = state.ClosedRouteNodeView{NodeID: id, RecordDigest: record, RoleDomain: domain, Subrole: subrole, DutyGeneration: uint64(index + 1)}
		candidate := &source.snapshot.Candidates[index]
		candidate.NodeID, candidate.PublicKey, candidate.FamilyID, candidate.RecordDigest = id, fixtureID(byte(50+index)), fixtureID(byte(70+index)), record
		candidate.Endpoint, candidate.CarrierProfile, candidate.Capacity = unused, string(route.ClosedCarrierTCP), 16
		candidate.ValidFrom, candidate.ValidUntil, candidate.AssignmentNotAfter = window, profile.NotAfter, profile.NotAfter
	}
	endpoint, principal := textContextEndpoint(t)
	endpoint.clock, endpoint.network, endpoint.closedState = time.Now, profile.NetworkID, source
	endpoint.closedEntryRoot, endpoint.closedRoleRoot = t.TempDir(), t.TempDir()
	for _, root := range []string{endpoint.closedEntryRoot, endpoint.closedRoleRoot} {
		if err := os.Chmod(root, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	roles, err := duty.Open(duty.Config{Root: endpoint.closedRoleRoot, Clock: time.Now, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := roles.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := endpoint.Close(); err != nil {
			t.Error(err)
		}
	})
	owner := textPermissionContextFixture(t, endpoint, principal, broker.Connection)
	return endpoint, owner, source
}

func selectTextSource(t *testing.T, owner *textContext) route.ClosedBootstrapSelection {
	t.Helper()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	selected, err := owner.selectTextBootstrapLocked()
	if err != nil {
		t.Fatal(err)
	}
	return selected
}

func TestTextSourceSetsSurviveWorkerLossAndKeepContextOwnership(t *testing.T) {
	endpoint, owner, _ := textSourceContextFixture(t)
	selected := selectTextSource(t, owner)
	original := *owner.sourceSet
	job, err := beginTextTestJob(t, owner, endpoint, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	owner.retireJob(job)
	if err := owner.finishJobCleanup(job, nil); err != nil {
		t.Fatal(err)
	}
	if repeated := selectTextSource(t, owner); repeated != selected || *owner.sourceSet != original {
		t.Fatal("worker loss rotated source sets")
	}
	publisher := textPermissionContextFixture(t, endpoint, fixtureID(211), broker.Administration)
	publisherSelection := selectTextSource(t, publisher)
	if publisher.sourceSet == owner.sourceSet || publisherSelection.EntryNodeID != selected.EntryNodeID {
		t.Fatal("context scope or installation Entry retention lost")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if owner.sourceSet != nil {
		t.Fatal("closed context retained Interior selection")
	}
	if repeated := selectTextSource(t, publisher); repeated != publisherSelection {
		t.Fatal("reader close changed Publisher source set")
	}
}

func TestTextSourceWithdrawalCannotResampleInterior(t *testing.T) {
	_, owner, source := textSourceContextFixture(t)
	selected := selectTextSource(t, owner)
	retained := *owner.sourceSet
	source.mu.Lock()
	for index := range source.snapshot.Candidates[:source.snapshot.CandidateCount] {
		if source.snapshot.Candidates[index].NodeID == selected.InteriorNodeID {
			source.snapshot.Candidates[index].Capacity = 0
		}
	}
	source.mu.Unlock()
	owner.mu.Lock()
	_, err := owner.selectTextBootstrapLocked()
	owner.mu.Unlock()
	if err == nil || *owner.sourceSet != retained {
		t.Fatal("withdrawn Interior replaced before set expiry")
	}
}

func TestTextSourceRejectsCrossProjectionStateBeforeSelection(t *testing.T) {
	endpoint, owner, source := textSourceContextFixture(t)
	source.snapshot.Digest[0] ^= 1
	owner.mu.Lock()
	_, err := owner.selectTextBootstrapLocked()
	owner.mu.Unlock()
	if err == nil || owner.sourceSet != nil || endpoint.closedEntries != nil {
		t.Fatal("mixed State projections created selection owner")
	}
}
