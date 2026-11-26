//go:build linux || darwin

package receiving_test

import (
	"context"
	"crypto/ed25519"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	epochfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkfixture "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

// An actual signed Network State supplies the facts at each Receiving
// observation. Invalid attempts must revoke the original fresh-root fact.
func TestRegistrationInitializationFactUsesGenuineNetworkReceivingOwner(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	spec, _, _ := networkfixture.PrepareNetworkGenesis(t, root)
	spec.Nodes[1].RoleDomain, spec.Nodes[1].Subrole = 4, 3
	bundle, err := epochfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal("signed Network", err)
	}
	clock, stopClock := networkfixture.StartObservedClock(t)
	defer stopClock()
	var authorityKey [32]byte
	copy(authorityKey[:], spec.Authority.Public().(ed25519.PublicKey))
	plan := networkfixture.SeedNetworkState(t, spec.NetworkID, authorityKey, bundle, clock)
	config, err := plan.Config()
	if err != nil {
		t.Fatal(err)
	}
	stateOwner, err := state.OpenRetained(config)
	if err != nil {
		t.Fatal(err)
	}
	defer stateOwner.Close()
	view, err := stateOwner.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	profile := view.Profile()
	member, err := view.Member(spec.Nodes[1].NodeID, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	receiver := receiving.Receiver{NetworkID: profile.Network, StateGeneration: profile.Generation, StateDigest: profile.EpochDigest,
		ProfileDigest: profile.Digest, NodeID: member.NodeID, DutyGeneration: member.DutyGeneration}
	binding := spending.Binding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration}
	observe := func() (receiving.Observation, error) {
		current, err := stateOwner.CurrentRuntime()
		if err != nil {
			return receiving.Observation{}, err
		}
		p := current.Profile()
		facts := admission.AuthorityFacts{NetworkID: p.Network, StateGeneration: p.Generation, StateDigest: p.EpochDigest, Digest: p.Digest,
			IssuanceAuthorityKey: p.IssuanceAuthorityKey, IssuerNodeID: p.IssuerNodeID, IssuerDutyGeneration: p.IssuerDutyGeneration,
			Epoch: p.Epoch, NotBefore: p.NotBefore, NotAfter: p.NotAfter, TokenKeyCount: uint8(len(p.TokenKeys))}
		for i, key := range p.TokenKeys {
			facts.TokenKeys[i] = admission.TokenKey{WindowStart: key.WindowStart, Class: key.Class, SPKI: key.SPKI}
		}
		return receiving.ObserveCurrentAuthority(current, facts, current.ObservedAt(), receiver, member.NotAfter())
	}
	for _, attempt := range []string{"canceled", "absent reservation", "invalid token", "foreign refill", "closed owner"} {
		t.Run(attempt, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			owner, err := receiving.Open(root, receiver, observe)
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			fact, err := owner.TakeFreshRoot()
			if err != nil {
				t.Fatal(err)
			}
			initialization, err := fact.Begin(binding)
			if err != nil {
				t.Fatal(err)
			}
			if err := initialization.Check(); err != nil {
				t.Fatal(err)
			}
			ctx := t.Context()
			if attempt == "canceled" {
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = canceled
			}
			reserve := func() (func() error, error) { t.Fatal("invalid attempt reached capacity"); return nil, nil }
			if attempt == "absent reservation" {
				reserve = nil
			}
			switch attempt {
			case "closed owner":
				err = owner.Close()
			case "foreign refill":
				_, err = owner.Refill(ctx, receiving.Grant{}, 0, nil, reserve)
			default:
				_, err = owner.Accept(ctx, admission.RegistrationClass, nil, time.Now().Add(time.Minute), reserve)
			}
			if attempt != "closed owner" && err == nil {
				t.Fatal("invalid admission succeeded")
			}
			if err := initialization.Check(); err == nil {
				t.Fatal("admission attempt retained effect authority")
			}
			if err := initialization.Complete(); err == nil {
				t.Fatal("late completion published initialization")
			}
			if _, err := owner.TakeFreshRoot(); err == nil {
				t.Fatal("reissued initialization after admission")
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := receiving.Open(root, receiver, observe)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			if _, err := reopened.TakeFreshRoot(); err == nil {
				t.Fatal("retained no-spend reopen issued initialization")
			}
		})
	}
}
