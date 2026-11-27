package state_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	networkdomain "github.com/dianabuilds/ardents-network/internal/domain/network"
	state2 "github.com/dianabuilds/ardents-network/internal/domain/network/state"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// Readers use the same public observation as Node and Endpoint while the real
// owner commits a successor Epoch and its separate signed role statement.
func TestMembershipObservationCannotMixPublishedGenerations(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	start := now.Truncate(time.Hour)
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{81}, 32))
	public := authority.Public().(ed25519.PublicKey)
	spec := networkfixture.ClosedSpec{NetworkID: [32]byte{1}, Seed: [32]byte{2}, IssuanceAuthority: [32]byte{3}, Number: 1,
		NotBefore: start, NotAfter: start.Add(2 * time.Hour), Authority: authority,
		Nodes: []networkfixture.ClosedNode{{NodeID: [32]byte{4}, Generation: 1,
			PrivateKey: ed25519.NewKeyFromSeed(bytes.Repeat([]byte{82}, 32)), ValidFrom: start, ValidUntil: start.Add(2 * time.Hour),
			Endpoint: "127.0.0.1:4601", Carrier: "ardents-carrier-tcp-tls-v2", Capability: 2, Capacity: 8, RoleDomain: 2, Subrole: 6}}}
	first, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	spec.Number, spec.Previous = 2, first.Epoch.Digest
	spec.Nodes[0].Generation = 2
	spec.Nodes[0].PrivateKey = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{83}, 32))
	spec.Nodes[0].Endpoint = "127.0.0.1:4602"
	second, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	clock := advancingVerificationClock(now)
	var observationOffset atomic.Int64
	config := state2.Config{Root: t.TempDir(), NetworkID: spec.NetworkID,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public}, Threshold: 1,
		Clock: clock, ObserveClock: func() time.Time { return clock().Add(time.Duration(observationOffset.Load())) },
		AcceptedProfile: "ardents-route-v3", ClosedProfileAuthority: public}
	owner, err := state2.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	accept := func(signed networkfixture.Closed) {
		t.Helper()
		if _, err := owner.AcceptEpoch(t.Context(), signed.Epoch.Raw, signed.Epoch.Inputs, signed.Epoch.Materials[:1]); err != nil {
			t.Fatal(err)
		}
		if _, err := owner.AcceptClosedProfile(signed.Profile); err != nil {
			t.Fatal(err)
		}
	}
	accept(first)
	checkMembership := func(view networkdomain.RuntimeView) error {
		profile := view.Profile()
		expected := first
		if profile.Epoch == 2 {
			expected = second
		}
		member, err := view.Member(spec.Nodes[0].NodeID, view.ObservedAt())
		if err != nil || profile.Epoch < 1 || profile.Epoch > 2 || profile.Generation != expected.Epoch.Digest ||
			profile.EpochDigest != expected.Epoch.Digest || profile.Digest != sha256.Sum256(expected.Profile) ||
			member.RecordDigest != sha256.Sum256(expected.Epoch.Inputs[0]) || member.DutyGeneration != expected.Nodes[0].Generation ||
			member.PublicKey != [32]byte(expected.Nodes[0].PrivateKey.Public().(ed25519.PublicKey)) || member.Endpoint != expected.Nodes[0].Endpoint {
			return fmt.Errorf("mixed membership observation for Epoch %d: %v", profile.Epoch, err)
		}
		return nil
	}
	stop, done := make(chan struct{}), make(chan struct{})
	observed := make(chan uint64, 2)
	failures := make(chan error, 1)
	go func() {
		defer close(done)
		seen := map[uint64]bool{}
		for {
			select {
			case <-stop:
				return
			default:
			}
			view, err := owner.CurrentRuntime()
			if err != nil {
				continue
			} // Epoch/profile publication gap may refuse.
			profile := view.Profile()
			if err := checkMembership(view); err != nil {
				failures <- err
				return
			}
			if !seen[profile.Epoch] {
				seen[profile.Epoch] = true
				observed <- profile.Epoch
			}
		}
	}()
	join := sync.OnceFunc(func() { close(stop); <-done })
	defer join()
	wait := func(epoch uint64) {
		t.Helper()
		select {
		case got := <-observed:
			if got != epoch {
				t.Fatalf("observed Epoch %d, wanted %d", got, epoch)
			}
		case err := <-failures:
			t.Fatal(err)
		case <-time.After(5 * time.Second):
			t.Fatalf("Epoch %d never became observable", epoch)
		}
	}
	wait(1)
	accept(second)
	wait(2)
	join()
	select {
	case err := <-failures:
		t.Fatal(err)
	default:
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}

	currentPath := filepath.Join(config.Root, "current")
	if err := os.WriteFile(currentPath, []byte(fmt.Sprintf("%x\n", first.Epoch.Digest)), 0600); err != nil {
		t.Fatal(err)
	}
	reopened, err := state2.OpenRetained(config)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	observeSuccessor := func() {
		t.Helper()
		view, err := reopened.CurrentRuntime()
		if err != nil {
			t.Fatal(err)
		}
		if view.Profile().Epoch != 2 {
			t.Fatalf("retained observation Epoch = %d, want 2", view.Profile().Epoch)
		}
		if err := checkMembership(view); err != nil {
			t.Fatal(err)
		}
	}
	observeSuccessor()
	observationOffset.Store(int64(-10 * time.Second))
	if _, err := reopened.CurrentRuntime(); err == nil {
		t.Fatal("retained runtime exposed the signed Profile without original clock confidence")
	}
	observationOffset.Store(0)
	observeSuccessor()
	profilePath := filepath.Join(config.Root, fmt.Sprintf("closed-profile-%x.bin", second.Epoch.Digest))
	if raw, err := os.ReadFile(profilePath); err != nil || !bytes.Equal(raw, second.Profile) {
		t.Fatalf("retained signed Profile bytes changed: %v", err)
	}
}
