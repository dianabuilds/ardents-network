//go:build linux

package endpoint

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/attempts"
	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func historyPreparationPlan(t *testing.T, role string) (HeadlessPlan, source.Bundle, []byte) {
	t.Helper()
	root := t.TempDir()
	encoded := func(n byte) string { key := [32]byte{n}; return hex.EncodeToString(key[:]) }
	authority := ed25519.NewKeyFromSeed(make([]byte, 32))
	nodeSeed := [32]byte{1}
	nodeKey := ed25519.NewKeyFromSeed(nodeSeed[:])
	facts, _ := participantStockFactsAt(t, time.Now().UTC(), 2)
	spec := networkfixture.ClosedSpec{NetworkID: facts.NetworkID, Seed: [32]byte{7}, IssuanceAuthority: facts.IssuanceAuthorityKey,
		Number: 1, NotBefore: facts.NotBefore, NotAfter: facts.NotAfter, Authority: authority,
		Nodes: []networkfixture.ClosedNode{{RecordSpec: networkfixture.RecordSpec{NodeID: facts.IssuerNodeID, Generation: 1,
			ValidFrom: facts.NotBefore, ValidUntil: facts.NotAfter, Endpoint: "127.0.0.1:4401", Carrier: "ardents-carrier-tcp-tls-v2", Capability: 2, Capacity: 3, PrivateKey: nodeKey}, RoleDomain: 2, Subrole: 6}}}
	for _, key := range facts.TokenKeys[:facts.TokenKeyCount] {
		spec.Keys = append(spec.Keys, networkfixture.ClosedTokenKey{WindowStart: key.WindowStart, Class: key.Class, SPKI: key.SPKI[:]})
	}
	signed, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	public := hex.EncodeToString(authority.Public().(ed25519.PublicKey))
	plan := HeadlessPlan{Schema: "ardents-headless-runtime-v2", Role: role,
		NetworkStateRoot: filepath.Join(root, "state"), LocalRoleStateRoot: filepath.Join(root, "participation"),
		TextTokenRoot: filepath.Join(root, "stock"), TimeConfidenceFile: filepath.Join(root, "clock"),
		NetworkID: encoded(1), NetworkAuthorities: []string{public}, ClosedProfileAuthority: public, NetworkThreshold: 1}
	for _, path := range []string{plan.NetworkStateRoot, plan.LocalRoleStateRoot, plan.TextTokenRoot} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(plan.TimeConfidenceFile, nil, 0644); err != nil {
		t.Fatal(err)
	}
	return plan, source.Bundle{Epoch: signed.Epoch.Raw, Inputs: signed.Epoch.Inputs, Materials: signed.Epoch.Materials}, signed.Profile
}

type historyEntry struct {
	Mode fs.FileMode
	Data string
}

func historyInventory(t *testing.T, root string) map[string]historyEntry {
	t.Helper()
	inventory := make(map[string]historyEntry)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		var raw []byte
		if info.Mode().IsRegular() {
			raw, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		}
		name, err := filepath.Rel(root, path)
		inventory[name] = historyEntry{info.Mode(), string(raw)}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return inventory
}

func TestParticipantInitialHistoriesAreExplicitAndRetained(t *testing.T) {
	for _, role := range []string{"reader", ""} {
		t.Run("role="+role, func(t *testing.T) {
			plan, bundle, profile := historyPreparationPlan(t, role)
			if err := PrepareHistories(t.Context(), plan, bundle, profile); err != nil {
				t.Fatal(err)
			}
			// Genuine signed State reopens with its accepted generation. No
			// Startup, Source transport or runtime success is supplied by this test.
			config, err := plan.Network()
			if err != nil {
				t.Fatal(err)
			}
			network, err := state.OpenRetained(config)
			if err != nil {
				t.Fatal(err)
			}
			if current, err := network.CurrentRuntime(); err != nil || current.Profile().Epoch != 1 {
				t.Fatal("signed preparation lost original State", err)
			}
			if err := network.Close(); err != nil {
				t.Fatal(err)
			}
			participation, err := duty.Open(duty.Config{Root: plan.LocalRoleStateRoot, Clock: time.Now})
			if err != nil {
				t.Fatal(err)
			}
			if err := participation.Close(); err != nil {
				t.Fatal(err)
			}
			names := []string{"reader"}
			if role == "" {
				names = append(names, "publisher")
			}
			for _, name := range names {
				journal, err := attempts.OpenRetained(filepath.Join(plan.TextTokenRoot, name), config.NetworkID, time.Now)
				if err != nil {
					t.Fatal(err)
				}
				if err := journal.Close(); err != nil {
					t.Fatal(err)
				}
			}
			entries, err := os.ReadDir(plan.TextTokenRoot)
			if err != nil || len(entries) != len(names) {
				t.Fatal("unexpected role histories", entries, err)
			}
			root := filepath.Dir(plan.NetworkStateRoot)
			before := historyInventory(t, root)
			if err := PrepareHistories(t.Context(), plan, bundle, profile); err == nil {
				t.Fatal("completed preparation replayed")
			}
			if !reflect.DeepEqual(before, historyInventory(t, root)) {
				t.Fatal("repeat changed history")
			}
		})
	}
}

func TestParticipantPartialPreparationCannotReplayOrInitializeLostHistory(t *testing.T) {
	plan, bundle, profile := historyPreparationPlan(t, "reader")
	ctx, cancel := context.WithCancel(t.Context())
	err := prepareHistories(ctx, plan, bundle, profile, cancel)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("interruption after State claim not retained", err)
	}
	root := filepath.Dir(plan.NetworkStateRoot)
	before := historyInventory(t, root)
	if len(before) <= 4 {
		t.Fatal("partial State claim disappeared")
	}
	if err := PrepareHistories(t.Context(), plan, bundle, profile); err == nil {
		t.Fatal("partial initialization adopted")
	}
	if !reflect.DeepEqual(before, historyInventory(t, root)) {
		t.Fatal("refusal changed partial history")
	}
}

func TestParticipantRetainedStateRefusesErasedAcceptedHistory(t *testing.T) {
	plan, bundle, profile := historyPreparationPlan(t, "reader")
	if err := PrepareHistories(t.Context(), plan, bundle, profile); err != nil {
		t.Fatal(err)
	}
	// Erase only signed State contents while retaining its original root inode. The
	// same constructor selected by Run must refuse rather than prepare again.
	original, err := os.Stat(plan.NetworkStateRoot)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(plan.NetworkStateRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(plan.NetworkStateRoot, entry.Name())); err != nil {
			t.Fatal(err)
		}
	}
	config, err := plan.Network()
	if err != nil {
		t.Fatal(err)
	}
	if owner, err := state.OpenRetained(config); err == nil {
		_ = owner.Close()
		t.Fatal("lost State history initialized")
	}
	current, err := os.Stat(plan.NetworkStateRoot)
	if err != nil || !os.SameFile(original, current) {
		t.Fatal("root inode changed", err)
	}
	entries, err = os.ReadDir(plan.NetworkStateRoot)
	if err != nil || len(entries) != 0 {
		t.Fatal("refusal recreated history", entries, err)
	}
}

func TestParticipantHistoryPreparationRefusesBeforeMutation(t *testing.T) {
	for _, test := range []string{"canceled", "role", "schema", "overlap", "clock-overlap", "foreign", "symlink", "access", "missing"} {
		t.Run(test, func(t *testing.T) {
			plan, bundle, profile := historyPreparationPlan(t, "reader")
			ctx := t.Context()
			switch test {
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "role":
				plan.Role = "publisher"
			case "schema":
				plan.Schema = "ardents-headless-runtime-v1"
			case "overlap":
				plan.LocalRoleStateRoot = filepath.Join(plan.NetworkStateRoot, "child")
			case "clock-overlap":
				plan.TimeConfidenceFile = filepath.Join(plan.TextTokenRoot, "clock")
			case "foreign":
				if err := os.WriteFile(filepath.Join(plan.LocalRoleStateRoot, "foreign"), []byte("retain"), 0600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				original := plan.LocalRoleStateRoot
				plan.LocalRoleStateRoot += "-link"
				if err := os.Symlink(original, plan.LocalRoleStateRoot); err != nil {
					t.Fatal(err)
				}
			case "access":
				if err := os.Chmod(plan.LocalRoleStateRoot, 0755); err != nil {
					t.Fatal(err)
				}
			case "missing":
				if err := os.Remove(plan.LocalRoleStateRoot); err != nil {
					t.Fatal(err)
				}
			}
			root := filepath.Dir(plan.NetworkStateRoot)
			before := historyInventory(t, root)
			if err := PrepareHistories(ctx, plan, bundle, profile); err == nil {
				t.Fatal("invalid initial preparation accepted")
			}
			if !reflect.DeepEqual(before, historyInventory(t, root)) {
				t.Fatal("refusal modified roots")
			}
		})
	}
}

func TestParticipantHistoryPreparationRetainsFailureAndSignedFloor(t *testing.T) {
	for _, changed := range []string{"epoch", "profile"} {
		t.Run(changed, func(t *testing.T) {
			plan, bundle, profile := historyPreparationPlan(t, "reader")
			originalBundle := bundle
			originalProfile := append([]byte(nil), profile...)
			if changed == "epoch" {
				bundle.Epoch = append([]byte(nil), bundle.Epoch...)
				bundle.Epoch[len(bundle.Epoch)-1] ^= 1
			} else {
				profile[len(profile)-1] ^= 1
			}
			if err := PrepareHistories(t.Context(), plan, bundle, profile); err == nil {
				t.Fatal("altered signature accepted")
			}
			for _, path := range []string{plan.LocalRoleStateRoot, plan.TextTokenRoot} {
				entries, err := os.ReadDir(path)
				if err != nil || len(entries) != 0 {
					t.Fatal("unauthenticated preparation created later history", entries, err)
				}
			}
			if changed == "profile" {
				config, err := plan.Network()
				if err != nil {
					t.Fatal(err)
				}
				owner, err := state.OpenRetained(config)
				if err != nil {
					t.Fatal("failed profile erased accepted Epoch", err)
				}
				if current, err := owner.Current(); err != nil || current.Epoch != 1 {
					t.Fatal("Epoch floor lost", current, err)
				}
				if _, err := owner.CurrentRuntime(); err == nil {
					t.Fatal("invalid profile supplied runtime authority")
				}
				if err := owner.Close(); err != nil {
					t.Fatal(err)
				}
			}
			root := filepath.Dir(plan.NetworkStateRoot)
			before := historyInventory(t, root)
			if err := PrepareHistories(t.Context(), plan, originalBundle, originalProfile); err == nil {
				t.Fatal("failed preparation replayed")
			}
			if !reflect.DeepEqual(before, historyInventory(t, root)) {
				t.Fatal("retry changed retained failure prefix")
			}
		})
	}
}
