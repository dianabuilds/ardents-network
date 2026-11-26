package command

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func TestOfflineStateRefusesPreCanceledCallerBeforeRootBirth(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	input := signedIntakeDeclaration(t, root)
	caller, cancel := context.WithCancel(t.Context())
	cancel()

	if result := runOffline(caller, "accept-epoch", input); result.Outcome != "canceled" {
		t.Fatalf("pre-canceled signed intake returned %q", result.Outcome)
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("already-canceled signed intake created a State root: %v", err)
	}
	if result := runOffline(t.Context(), "accept-epoch", input); result.Outcome != "accepted-epoch" {
		t.Fatalf("live signed intake returned %q", result.Outcome)
	}
}

func TestOfflineStateCancellationAfterOpenRetainsRootAndReleasesLease(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	input := signedIntakeDeclaration(t, root)
	caller, cancel := context.WithCancel(t.Context())
	defer cancel()
	input.Config.PermitWork = func() error {
		_, err := os.Lstat(root)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		cancel()
		return nil
	}
	if result := runOffline(caller, "accept-epoch", input); result.Outcome != "canceled" {
		t.Fatalf("intake canceled after opening returned %q", result.Outcome)
	}
	if info, err := os.Lstat(root); err != nil || !info.IsDir() {
		t.Fatalf("original State root did not survive cancellation: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "current")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled intake committed a generation: %v", err)
	}
	input.Config.PermitWork = nil
	if result := runOffline(t.Context(), "accept-epoch", input); result.Outcome != "accepted-epoch" {
		t.Fatalf("signed intake through reopened root returned %q", result.Outcome)
	}
}

func signedIntakeDeclaration(t *testing.T, root string) offlineDeclaration {
	t.Helper()
	now := time.Unix(1_800_000_100, 0).UTC()
	networkID := sha256.Sum256([]byte("offline intake Network"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{1}, ed25519.SeedSize))
	nodeKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{2}, ed25519.SeedSize))
	public := authority.Public().(ed25519.PublicKey)
	nodeID := sha256.Sum256(nodeKey.Public().(ed25519.PublicKey))
	record, err := networkfixture.BuildRecord(networkfixture.RecordSpec{
		NetworkID: networkID, NodeID: nodeID, Generation: 1,
		ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour),
		Family: "offline-intake", Endpoint: "192.0.2.10:48000",
		Carrier: "ardents-carrier-tcp-tls-v2", Capability: 2, Capacity: 4, PrivateKey: nodeKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	epoch, err := networkfixture.BuildEpoch(networkfixture.EpochSpec{
		NetworkID: networkID, Number: 1, ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour),
		Inputs: [][]byte{record.Raw}, Accepted: []networkfixture.Record{record},
		AssignmentSeed: sha256.Sum256([]byte("offline intake assignment")),
		Profile:        "ardents-route-v3", Version: 3, Domains: []string{"offline"},
		Authorities: []ed25519.PrivateKey{authority},
	})
	if err != nil {
		t.Fatal(err)
	}
	return offlineDeclaration{
		Config: state.Config{Root: root, NetworkID: networkID,
			Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public},
			Threshold:   1, ClosedProfileAuthority: public, AcceptedProfile: "ardents-route-v3",
			Now: now, ClockObservation: now},
		Epoch: epoch.Raw, Inputs: epoch.Inputs, Materials: epoch.Materials,
	}
}
