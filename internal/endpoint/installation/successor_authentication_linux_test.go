//go:build linux

package installation

import (
	"context"
	"encoding/hex"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
)

func TestSuccessorCannotBootstrapAnotherRootOrEmptyFloors(t *testing.T) {
	enrolled := protectedReleaseFixture(t, nil, false)
	candidate := enrollment.Candidate{Inputs: enrolled.Inputs, ProtectedDescriptor: enrolled.ProtectedDescriptor, ProtectedFiles: enrolled.ProtectedFiles}
	verifier, err := release.Open(filepath.Join(t.TempDir(), "floors"))
	if err != nil {
		t.Fatal(err)
	}
	defer verifier.Close()
	previous := localBinding{Generation: targetBinding{TargetsVersion: 1}}
	if _, err := authenticateSuccessor(context.Background(), verifier, candidate, previous); err == nil {
		t.Fatal("untrusted candidate bootstrapped an empty floor store")
	}
	floors, err := verifier.CurrentFloors()
	if err != nil || floors.RootVersion != 0 {
		t.Fatal("bootstrap refusal changed floors")
	}
	authorization, err := Authenticate(context.Background(), verifier, enrolled)
	if err != nil {
		t.Fatal(err)
	}
	_, proof := authorization.Targets()
	decision, ok := proof.AcceptedDecision()
	if !ok {
		t.Fatal("initial fixture lacks proof")
	}
	previous.Generation = targetBinding{TargetsVersion: decision.Floors.TargetsVersion,
		TargetsDigest: hex.EncodeToString(decision.Floors.TargetsDigest), Platform: decision.Platform,
		Architecture: decision.Architecture, Environment: decision.Environment, Network: decision.Network}
	if _, err := authenticateSuccessor(context.Background(), verifier, candidate, previous); err != nil {
		t.Fatal(err)
	}
	foreign := protectedReleaseFixture(t, nil, false)
	candidate.Inputs = foreign.Inputs
	candidate.ProtectedDescriptor, candidate.ProtectedFiles = foreign.ProtectedDescriptor, foreign.ProtectedFiles
	if _, err := authenticateSuccessor(context.Background(), verifier, candidate, previous); err == nil {
		t.Fatal("another locally supplied trust root replaced the established root")
	}
}

func TestSuccessorRefusesBindingAboveFloorAndForeignEnvironment(t *testing.T) {
	enrolled := protectedReleaseFixture(t, nil, false)
	verifier, err := release.Open(filepath.Join(t.TempDir(), "floors"))
	if err != nil {
		t.Fatal(err)
	}
	defer verifier.Close()
	auth, err := Authenticate(context.Background(), verifier, enrolled)
	if err != nil {
		t.Fatal(err)
	}
	_, proof := auth.Targets()
	decision, _ := proof.AcceptedDecision()
	base := targetBinding{TargetsVersion: decision.Floors.TargetsVersion, TargetsDigest: hex.EncodeToString(decision.Floors.TargetsDigest),
		Platform: decision.Platform, Architecture: decision.Architecture, Environment: decision.Environment, Network: decision.Network}
	candidate := enrollment.Candidate{Inputs: enrolled.Inputs, ProtectedDescriptor: enrolled.ProtectedDescriptor, ProtectedFiles: enrolled.ProtectedFiles}
	for _, change := range []func(*targetBinding){
		func(b *targetBinding) { b.TargetsVersion++ },
		func(b *targetBinding) { b.TargetsDigest = "foreign" },
		func(b *targetBinding) { b.Network = "foreign" },
		func(b *targetBinding) { b.Environment = "foreign" },
	} {
		changed := base
		change(&changed)
		if _, err := authenticateSuccessor(context.Background(), verifier, candidate, localBinding{Generation: changed}); err == nil {
			t.Fatal("successor accepted a conflicting binding")
		}
	}
}
