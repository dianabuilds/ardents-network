package state_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"

	state2 "github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

func TestOfflineAcceptUsesClockAtAcceptance(t *testing.T) {
	genesis := newFixture(t)
	successor := futureFixture(t, genesis, fixtureNow+10)
	now := time.Unix(fixtureNow, 0).UTC()
	clockCalls := 0
	store, err := state2.Open(state2.Config{
		Root: t.TempDir(), NetworkID: genesis.networkID,
		Authorities:            map[[32]byte]ed25519.PublicKey{genesis.authorityID: genesis.authorityPublic},
		ClosedProfileAuthority: genesis.authorityPublic,
		Threshold:              1, Clock: func() time.Time { clockCalls++; return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Accept(context.Background(), genesis.epoch, genesis.inputs, genesis.materializations); err != nil {
		t.Fatalf("accept genesis: %v", err)
	}

	now = now.Add(20 * time.Second)
	beforeCalls := clockCalls
	accepted, err := store.Accept(context.Background(), successor.epoch, successor.inputs, successor.materializations)
	if err != nil || accepted.Epoch != 2 || accepted.Digest != successor.epochDigest || !accepted.TrustedTime.Equal(now) || accepted.Freshness != "fresh" {
		t.Fatalf("accept successor current at acceptance time: epoch=%d err=%v", accepted.Epoch, err)
	}
	if clockCalls != beforeCalls+1 {
		t.Fatalf("Accept sampled clock %d times, want one", clockCalls-beforeCalls)
	}
}

func TestOfflineAcceptRejectsExpiredAtAcceptanceWithoutWriting(t *testing.T) {
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	root := t.TempDir()
	now := time.Unix(fixtureNow, 0).UTC()
	store, err := state2.Open(state2.Config{
		Root: root, NetworkID: genesis.networkID,
		Authorities:            map[[32]byte]ed25519.PublicKey{genesis.authorityID: genesis.authorityPublic},
		ClosedProfileAuthority: genesis.authorityPublic,
		Threshold:              1, Clock: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Accept(context.Background(), genesis.epoch, genesis.inputs, genesis.materializations); err != nil {
		t.Fatalf("accept genesis: %v", err)
	}
	beforeCurrent, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
		t.Fatal(err)
	}
	beforeDistribution, err := os.ReadFile(filepath.Join(root, "distribution", "current"))
	if err != nil {
		t.Fatal(err)
	}

	now = now.Add(2 * time.Hour)
	if _, err := store.Accept(context.Background(), successor.epoch, successor.inputs, successor.materializations); err == nil {
		t.Fatal("accepted successor expired at acceptance time")
	}
	afterCurrent, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
		t.Fatal(err)
	}
	afterDistribution, err := os.ReadFile(filepath.Join(root, "distribution", "current"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterCurrent, beforeCurrent) || !bytes.Equal(afterDistribution, beforeDistribution) {
		t.Fatal("expired successor changed current pointer or distribution floor")
	}
}
