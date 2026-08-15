package state_test

import (
	"context"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestAcceptOfflineGenesisAndRecoverCurrent(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	root := t.TempDir()
	store, err := state.Open(state.Config{
		Root:        root,
		NetworkID:   fixture.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{fixture.authorityID: fixture.authorityPublic},
		Threshold:   1,
		Now:         time.Unix(fixture.now, 0),
	})
	if err != nil {
		t.Fatalf("open empty state: %v", err)
	}

	snapshot, err := store.Accept(context.Background(), fixture.epoch, fixture.inputs, fixture.materializations)
	if err != nil {
		t.Fatalf("accept genesis: %v", err)
	}
	if snapshot.Epoch != 1 || snapshot.ViewLength != 2 || snapshot.RejectedLength != 6 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if snapshot.Digest != fixture.epochDigest || snapshot.ViewRoot != fixture.viewRoot {
		t.Fatal("snapshot did not retain the verified commitments")
	}
	if !snapshot.RecordPresent || snapshot.Profile != "h3-role-probe-v1" || snapshot.Assignment == "" || snapshot.ProbeCapacity == 0 {
		t.Fatalf("snapshot lacks its proven local record and assignment: %+v", snapshot)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close state: %v", err)
	}

	pointer, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
		t.Fatalf("read current pointer: %v", err)
	}
	if string(pointer) != snapshot.Generation+"\n" {
		t.Fatalf("current pointer = %q, want %q", pointer, snapshot.Generation+"\n")
	}
	if _, err := os.Stat(filepath.Join(root, "generations", snapshot.Generation, "epoch.bin")); err != nil {
		t.Fatalf("stat immutable epoch: %v", err)
	}

	reopened, err := state.Open(state.Config{
		Root:        root,
		NetworkID:   fixture.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{fixture.authorityID: fixture.authorityPublic},
		Threshold:   1,
		Now:         time.Unix(fixture.now, 0),
	})
	if err != nil {
		t.Fatalf("reopen state: %v", err)
	}
	defer reopened.Close()
	recovered, err := reopened.Current()
	if err != nil {
		t.Fatalf("read recovered current state: %v", err)
	}
	if recovered != snapshot {
		t.Fatalf("recovered snapshot = %+v, want %+v", recovered, snapshot)
	}
}

func TestAcceptRejectsWrongMaterializationWithoutWriting(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	fixture.materializations[0][len(fixture.materializations[0])-1] ^= 0xff
	root := t.TempDir()
	store, err := state.Open(state.Config{
		Root:        root,
		NetworkID:   fixture.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{fixture.authorityID: fixture.authorityPublic},
		Threshold:   1,
		Now:         time.Unix(fixture.now, 0),
	})
	if err != nil {
		t.Fatalf("open state: %v", err)
	}
	defer store.Close()
	if _, err := store.Accept(context.Background(), fixture.epoch, fixture.inputs, fixture.materializations); err == nil {
		t.Fatal("accept succeeded with a corrupt proof")
	}
	if _, err := os.Stat(filepath.Join(root, "current")); !os.IsNotExist(err) {
		t.Fatalf("invalid decision changed current pointer: %v", err)
	}
}

func TestOpenRemovesInterruptedStagingAndKeepsNoCurrent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	fixture := newFixture(t)
	claimed, err := state.Open(state.Config{
		Root: root, NetworkID: fixture.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{fixture.authorityID: fixture.authorityPublic},
		Threshold:   1, Now: time.Unix(fixtureNow, 0),
	})
	if err != nil {
		t.Fatalf("claim empty owned root: %v", err)
	}
	if err := claimed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "generations", ".stage-interrupted"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".current-interrupted"), []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := state.Open(state.Config{
		Root:        root,
		NetworkID:   fixture.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{fixture.authorityID: fixture.authorityPublic},
		Threshold:   1,
		Now:         time.Unix(fixture.now, 0),
	})
	if err != nil {
		t.Fatalf("recover interrupted staging: %v", err)
	}
	defer store.Close()
	if _, err := store.Current(); err == nil {
		t.Fatal("empty recovered root reported current state")
	}
	for _, path := range []string{
		filepath.Join(root, "generations", ".stage-interrupted"),
		filepath.Join(root, ".current-interrupted"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("interrupted path remains %s: %v", path, err)
		}
	}
}

func TestOpenRefusesNonEmptyUnownedRootWithoutCleanup(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	unrelated := filepath.Join(root, ".current-unrelated")
	if err := os.WriteFile(unrelated, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	value := newFixture(t)
	_, err := state.Open(state.Config{
		Root: root, NetworkID: value.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{value.authorityID: value.authorityPublic},
		Threshold:   1, Now: time.Unix(value.now, 0),
	})
	if err == nil {
		t.Fatal("non-empty unowned root was claimed")
	}
	contents, readErr := os.ReadFile(unrelated)
	if readErr != nil || string(contents) != "keep" {
		t.Fatalf("unowned file changed: contents=%q err=%v", contents, readErr)
	}
}

func TestAcceptRecoversCompleteGenerationMissingCurrentPointer(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	root := t.TempDir()
	config := state.Config{
		Root: root, NetworkID: fixture.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{fixture.authorityID: fixture.authorityPublic},
		Threshold:   1, Now: time.Unix(fixture.now, 0),
	}
	store, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := store.Accept(context.Background(), fixture.epoch, fixture.inputs, fixture.materializations)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "current")); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	recovered, err := restarted.Current()
	if err != nil || recovered != accepted {
		t.Fatalf("recover completed immutable generation: snapshot=%+v err=%v", recovered, err)
	}
}

func TestOpenRejectsMismatchedAuthorityIdentifier(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	wrongID := fixture.authorityID
	wrongID[0] ^= 0xff
	_, err := state.Open(state.Config{
		Root: t.TempDir(), NetworkID: fixture.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{wrongID: fixture.authorityPublic},
		Threshold:   1, Now: time.Unix(fixture.now, 0),
	})
	if err == nil {
		t.Fatal("mismatched authority identifier was accepted")
	}
}

func TestSuccessorChainSurvivesRestart(t *testing.T) {
	t.Parallel()
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	config := state.Config{
		Root: t.TempDir(), NetworkID: genesis.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{genesis.authorityID: genesis.authorityPublic},
		Threshold:   1, Now: time.Unix(genesis.now, 0),
	}
	store, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Accept(context.Background(), genesis.epoch, genesis.inputs, genesis.materializations); err != nil {
		t.Fatalf("accept genesis: %v", err)
	}
	want, err := store.Accept(context.Background(), successor.epoch, successor.inputs, successor.materializations)
	if err != nil {
		t.Fatalf("accept successor: %v", err)
	}
	if want.Epoch != 2 {
		t.Fatalf("successor epoch=%d, want 2", want.Epoch)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := state.Open(config)
	if err != nil {
		t.Fatalf("reopen successor chain: %v", err)
	}
	got, err := restarted.Current()
	if err != nil || got != want {
		t.Fatalf("restarted snapshot=%+v err=%v, want %+v", got, err, want)
	}
	if err := restarted.Close(); err != nil {
		t.Fatal(err)
	}
	laterConfig := config
	laterConfig.Now = time.Unix(genesis.now+1900, 0)
	later, err := state.Open(laterConfig)
	if err != nil {
		t.Fatalf("reopen current successor after genesis expiry: %v", err)
	}
	defer later.Close()
	laterSnapshot, err := later.Current()
	if err != nil || laterSnapshot != want {
		t.Fatalf("later snapshot=%+v err=%v, want %+v", laterSnapshot, err, want)
	}
}

func TestStateRootLeaseExcludesSecondStore(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	config := state.Config{
		Root: t.TempDir(), NetworkID: fixture.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{fixture.authorityID: fixture.authorityPublic},
		Threshold:   1, Now: time.Unix(fixture.now, 0),
	}
	first, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.Open(config); err == nil || !strings.Contains(err.Error(), "exclusive state-root lease") {
		t.Fatalf("second Store acquired the owned root: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	third, err := state.Open(config)
	if err != nil {
		t.Fatalf("released root lease stayed locked: %v", err)
	}
	defer third.Close()
}

func TestDistributionStateRepairsStaleCurrentMirror(t *testing.T) {
	t.Parallel()
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	config := state.Config{
		Root: t.TempDir(), NetworkID: genesis.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{genesis.authorityID: genesis.authorityPublic},
		Threshold:   1, Now: time.Unix(genesis.now, 0),
	}
	store, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Accept(context.Background(), genesis.epoch, genesis.inputs, genesis.materializations)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Accept(context.Background(), successor.epoch, successor.inputs, successor.materializations)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config.Root, "current"), []byte(first.Generation+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	current, err := restarted.Current()
	if err != nil || current.Generation != second.Generation {
		t.Fatalf("recovered active generation=%s err=%v", current.Generation, err)
	}
}
