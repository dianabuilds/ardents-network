//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/release"
	"github.com/dianabuilds/ardents-network/internal/release/infrastructure"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// Signed in-process byte fixtures isolate the Release handoff. No process,
// system manager, network, initial independent pin or startup is simulated.
func TestPlatformRuntimeReleaseUsesRetainedFloorsAndOriginalProof(t *testing.T) {
	for _, role := range []string{"source", "forwarding", "issuer", "descriptor", "introduction", "join"} {
		t.Run(role, func(t *testing.T) {
			program := []byte("signed program fixture")
			dir, _, _ := infrastructureDistributionInvocation(t, t.TempDir(), program, nil, 1, nil, role, []byte(platformUnitTemplate))
			candidate, err := enrollment.ReadInfrastructureCandidate(t.Context(), dir)
			if err != nil {
				t.Fatal(err)
			}
			in := infrastructureCandidateInput(t, candidate)
			history := filepath.Join(t.TempDir(), "release")
			v, err := release.Open(history)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := infrastructure.AuthenticateRuntimeCandidate(t.Context(), v, candidate, in); err == nil {
				t.Fatal("candidate supplied initial trust")
			}
			// Test setup explicitly seeds the genuine verifier, not a serialized
			// accepting fact. Production first trust requires Enrollment's pin.
			decision := v.Evaluate(t.Context(), in)
			if _, ok := decision.Authorization(); !ok {
				t.Fatal(decision.Outcome, decision.Err())
			}
			if err := v.Close(); err != nil {
				t.Fatal(err)
			}
			v, err = release.OpenRetained(history)
			if err != nil {
				t.Fatal(err)
			}
			a, err := infrastructure.AuthenticateRuntimeCandidate(t.Context(), v, candidate, in)
			if err != nil {
				t.Fatal(err)
			}
			if err := a.Check(t.Context(), v); err != nil {
				t.Fatal("original pair failed handoff", err)
			}
			cancelled, cancel := context.WithCancel(t.Context())
			cancel()
			if a.Check(cancelled, v) == nil {
				t.Fatal("canceled handoff accepted")
			}
			if err := v.Close(); err != nil {
				t.Fatal(err)
			}
			if a.Check(t.Context(), v) == nil {
				t.Fatal("closed proof reused")
			}
			if err := os.RemoveAll(history); err != nil {
				t.Fatal(err)
			}
			if v, err := release.OpenRetained(history); err == nil {
				_ = v.Close()
				t.Fatal("lost trust recreated")
			}
			if _, err := os.Stat(history); !os.IsNotExist(err) {
				t.Fatal("retained opening created missing history", err)
			}
		})
	}
}

// This is the offline software/State boundary. Real signatures, verifier floors
// and State files are used, but program bytes are never executed. It grants no
// systemd, serving, predecessor completion or platform qualification evidence.
func TestPlatformSourceForwardProgramKeepsSignedStateAndRefusesMixedRelease(t *testing.T) {
	now := infrastructureReference
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{81}, 32))
	nodeKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{82}, 32))
	public := key.Public().(ed25519.PublicKey)
	network := [32]byte{83}
	record, err := networkfixture.BuildRecord(networkfixture.RecordSpec{NetworkID: network, NodeID: [32]byte{84}, Generation: 1, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), Family: "source-lifecycle-fixture", Endpoint: "127.0.0.1:4101", Carrier: "ardents-carrier-tcp-tls-v2", Capability: 2, Capacity: 1, PrivateKey: nodeKey})
	if err != nil {
		t.Fatal(err)
	}
	spec := networkfixture.EpochSpec{NetworkID: network, Number: 1, ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour), Inputs: [][]byte{record.Raw}, Accepted: []networkfixture.Record{record}, AssignmentSeed: [32]byte{85}, Profile: "ardents-route-v3", Version: 3, Domains: []string{"initiator", "introduction", "rendezvous", "responder"}, Authorities: []ed25519.PrivateKey{key}}
	genesis, err := networkfixture.BuildEpoch(spec)
	if err != nil {
		t.Fatal(err)
	}
	spec.Number = 2
	spec.Previous = genesis.Digest
	current, err := networkfixture.BuildEpoch(spec)
	if err != nil {
		t.Fatal(err)
	}
	cfg := state.Config{Root: t.TempDir(), NetworkID: network, Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public}, Threshold: 1, ClosedProfileAuthority: public, Now: now, ClockObservation: now}
	owner, err := state.OpenFresh(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, epoch := range []networkfixture.Epoch{genesis, current} {
		if _, err := owner.Accept(t.Context(), epoch.Raw, epoch.Inputs, epoch.Materials); err != nil {
			_ = owner.Close()
			t.Fatal(err)
		}
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	pointer, err := os.ReadFile(filepath.Join(cfg.Root, "current"))
	if err != nil {
		t.Fatal(err)
	}
	participation := t.TempDir()
	if err := os.Chmod(participation, 0700); err != nil {
		t.Fatal(err)
	}
	rolesConfig := duty.Config{Root: participation, Clock: func() time.Time { return now }}
	if err := duty.Initialize(t.Context(), rolesConfig); err != nil {
		t.Fatal(err)
	}
	roles, err := duty.Open(rolesConfig)
	if err != nil {
		t.Fatal(err)
	}
	identity, family := [32]byte{90}, [32]byte{91}
	if err := roles.Replace([32]byte{92}, []duty.Duty{{Identity: identity, Family: family, Class: "direct-source", State: "exposed", NotAfter: now.Add(time.Hour)}}); err != nil {
		_ = roles.Close()
		t.Fatal(err)
	}
	if err := roles.Close(); err != nil {
		t.Fatal(err)
	}
	rolesPointer, err := os.ReadFile(filepath.Join(participation, "current"))
	if err != nil {
		t.Fatal(err)
	}
	authority := newConsumerReleaseAuthority(t, now.Add(365*24*time.Hour))
	load := func(version int64, program []byte, mutate func(map[string]*metadata.TargetFiles)) enrollment.InfrastructureCandidate {
		dir, _, _ := infrastructureDistributionInvocation(t, t.TempDir(), program, authority, version, mutate, "source", []byte(platformUnitTemplate))
		candidate, err := enrollment.ReadInfrastructureCandidate(t.Context(), dir)
		if err != nil {
			t.Fatal(err)
		}
		return candidate
	}
	old := load(1, []byte("original signed Source program"), nil)
	next := load(2, []byte("distinct forward signed Source program"), nil)
	mixed := load(3, []byte("mixed Source program"), func(targets map[string]*metadata.TargetFiles) {
		generation := targets["ardents/linux-amd64/infrastructure/source"]
		var fields map[string]any
		if err := json.Unmarshal(*generation.Custom, &fields); err != nil {
			t.Fatal(err)
		}
		fields["release_version"] = 2
		raw, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		custom := json.RawMessage(raw)
		generation.Custom = &custom
	})
	history := filepath.Join(t.TempDir(), "release")
	verifier, err := release.Open(history)
	if err != nil {
		t.Fatal(err)
	}
	if d := verifier.Evaluate(t.Context(), infrastructureCandidateInput(t, old)); d.Err() != nil {
		_ = verifier.Close()
		t.Fatal(d.Outcome, d.Err())
	}
	if err := verifier.Close(); err != nil {
		t.Fatal(err)
	}
	// Initial admitted bytes, same-program relaunch, then distinct forward bytes.
	for _, candidate := range []enrollment.InfrastructureCandidate{old, old, next} {
		verifier, err = release.OpenRetained(history)
		if err != nil {
			t.Fatal(err)
		}
		proof, err := infrastructure.AuthenticateRuntimeCandidate(t.Context(), verifier, candidate, infrastructureCandidateInput(t, candidate))
		if err != nil {
			_ = verifier.Close()
			t.Fatal(err)
		}
		if err := proof.Check(t.Context(), verifier); err != nil {
			_ = verifier.Close()
			t.Fatal(err)
		}
		if err := verifier.Close(); err != nil {
			t.Fatal(err)
		}
		if err := proof.Check(t.Context(), verifier); err == nil {
			t.Fatal("old closed verifier proof reused")
		}
		owner, err = state.OpenRetained(cfg)
		if err != nil {
			t.Fatal(err)
		}
		view, err := owner.Current()
		if err != nil || view.Epoch != 2 || view.Digest != current.Digest {
			_ = owner.Close()
			t.Fatal("program change lost authenticated State", view, err)
		}
		if _, err := owner.Accept(t.Context(), genesis.Raw, genesis.Inputs, genesis.Materials); err == nil {
			_ = owner.Close()
			t.Fatal("program change reset Epoch floor")
		}
		if err := owner.Close(); err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(filepath.Join(cfg.Root, "current"))
		if err != nil || !bytes.Equal(pointer, after) {
			t.Fatal("program change or rejected Epoch rewrote current", err)
		}
		roles, err = duty.Open(rolesConfig)
		if err != nil {
			t.Fatal(err)
		}
		conflict, err := roles.Conflict(identity, family)
		if closeErr := roles.Close(); err != nil || !conflict || closeErr != nil {
			t.Fatal("program change lost retained participation", conflict, err, closeErr)
		}
		after, err = os.ReadFile(filepath.Join(participation, "current"))
		if err != nil || !bytes.Equal(rolesPointer, after) {
			t.Fatal("program change rewrote participation", err)
		}
	}
	verifier, err = release.OpenRetained(history)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := verifier.Close(); err != nil {
			t.Error(err)
		}
	}()
	for name, candidate := range map[string]enrollment.InfrastructureCandidate{"rollback": old, "mixed": mixed} {
		if _, err := infrastructure.AuthenticateRuntimeCandidate(t.Context(), verifier, candidate, infrastructureCandidateInput(t, candidate)); err == nil {
			t.Fatal("unsafe Source program accepted", name)
		}
	}
	after, err := os.ReadFile(filepath.Join(cfg.Root, "current"))
	if err != nil || !bytes.Equal(pointer, after) {
		t.Fatal("Release refusal changed State history", err)
	}
}

func TestPlatformRuntimeReleaseRefusesRollbackAfterReopen(t *testing.T) {
	authority := newConsumerReleaseAuthority(t, infrastructureReference.Add(365*24*time.Hour))
	program := []byte("same signed runtime")
	load := func(version int64) enrollment.InfrastructureCandidate {
		dir, _, _ := infrastructureDistributionInvocation(t, t.TempDir(), program, authority, version, nil, "introduction", []byte(platformUnitTemplate))
		c, err := enrollment.ReadInfrastructureCandidate(t.Context(), dir)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	old, current := load(1), load(2)
	history := filepath.Join(t.TempDir(), "release")
	v, err := release.Open(history)
	if err != nil {
		t.Fatal(err)
	}
	in := infrastructureCandidateInput(t, current)
	if d := v.Evaluate(t.Context(), in); d.Err() != nil {
		t.Fatal(d.Outcome, d.Err())
	}
	if _, err := infrastructure.AuthenticateRuntimeCandidate(t.Context(), v, current, in); err != nil {
		t.Fatal(err)
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}
	v, err = release.OpenRetained(history)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := v.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := infrastructure.AuthenticateRuntimeCandidate(t.Context(), v, old, infrastructureCandidateInput(t, old)); err == nil {
		t.Fatal("older signed generation authorized after reopen")
	}
	floors, err := v.CurrentFloors(t.Context())
	if err != nil || floors.TargetsVersion != 2 {
		t.Fatal("rollback changed retained floor", floors, err)
	}
}
