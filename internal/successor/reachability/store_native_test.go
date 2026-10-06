//go:build linux || windows

package reachability_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
)

// These tests require the actual Linux/Windows lease and directory-sync adapter.
// Public proof/rule code itself has no native dependency or installed claim.
func TestStorePublishCapabilityAndOriginalEffectChecks(t *testing.T) {
	raw, target, network, profile, at := signedDescriptor(2)
	root := t.TempDir()
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: root, Network: network})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := reachability.OpenStore(reachability.StoreConfig{Root: root, Network: network}); err == nil {
		t.Fatal("duplicate Store lease acquired")
	}
	if outcome, err := store.Publish(raw, profile, at, func() error { return nil }); err == nil || outcome != reachability.Invalid {
		t.Fatal("Connect-only input mutated Store", outcome, err)
	}
	if entries, err := os.ReadDir(filepath.Join(root, "records")); err != nil || len(entries) != 0 {
		t.Fatal("invalid capability touched records", entries, err)
	}
	raw, _, _, _, _ = signedDescriptor(3)
	lost := errors.New("original receiving lifetime lost")
	checks := 0
	if outcome, err := store.Publish(raw, profile, at, func() error {
		checks++
		if checks == 2 {
			return lost
		}
		return nil
	}); !errors.Is(err, lost) || outcome != reachability.Invalid {
		t.Fatal("loss after proof verification allowed mutation", outcome, err)
	}
	if entries, err := os.ReadDir(filepath.Join(root, "records")); err != nil || len(entries) != 0 {
		t.Fatal("original refusal touched records", entries, err)
	}
	checks = 0
	if outcome, err := store.Publish(raw, profile, at, func() error {
		checks++
		if checks == 3 {
			return lost
		}
		return nil
	}); !errors.Is(err, lost) || outcome != reachability.Invalid {
		t.Fatal("post-commit loss acknowledged", outcome, err)
	}
	if got, outcome, err := store.Lookup(target, profile, at); err != nil || outcome != reachability.Accepted || !bytes.Equal(got, raw) {
		t.Fatal("post-commit refusal erased actual durable floor", outcome, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = reachability.OpenStore(reachability.StoreConfig{Root: root, Network: network})
	if err != nil {
		t.Fatal(err)
	}
	if got, outcome, err := store.Lookup(target, profile, at); err != nil || outcome != reachability.Accepted || !bytes.Equal(got, raw) {
		t.Fatal("reopen lost committed proof", outcome, err)
	}
}

func TestStoreRevisionConflictReopenAndHigherRepair(t *testing.T) {
	raw, target, network, profile, at := signedDescriptor(3)
	root := t.TempDir()
	config := reachability.StoreConfig{Root: root, Network: network}
	store, err := reachability.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	check := func() error { return nil }
	if outcome, err := store.Publish(raw, profile, at, check); err != nil || outcome != reachability.Accepted {
		t.Fatal(outcome, err)
	}
	if outcome, err := store.Publish(raw, profile, at, check); err != nil || outcome != reachability.AlreadyCurrent {
		t.Fatal("exact retry differs", outcome, err)
	}
	conflict := resignDescriptor(raw, func(b []byte) { b[202] ^= 1 })
	if outcome, err := store.Publish(conflict, profile, at, check); err == nil || outcome != reachability.Conflicting {
		t.Fatal("same revision conflict accepted", outcome, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = reachability.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	if proof, outcome, err := store.Lookup(target, profile, at); err != nil || outcome != reachability.Conflicting || len(proof) != 0 {
		t.Fatal("conflict revived predecessor", outcome, err)
	}
	if outcome, err := store.Publish(raw, profile, at, check); err == nil || outcome != reachability.Conflicting {
		t.Fatal("exact old proof repaired conflict", outcome, err)
	}
	repaired := resignDescriptor(raw, func(b []byte) {
		binary.BigEndian.PutUint64(b[162:170], 10)
		binary.BigEndian.PutUint64(b[274:282], uint64(at.Add(30*time.Second).Unix()))
	})
	if outcome, err := store.Publish(repaired, profile, at, check); err != nil || outcome != reachability.Accepted {
		t.Fatal("higher revision with smaller expiry refused", outcome, err)
	}
	if outcome, err := store.Publish(raw, profile, at, check); err == nil || outcome != reachability.Stale {
		t.Fatal("lower revision revived", outcome, err)
	}
	if proof, outcome, _ := store.Lookup(target, profile, at.Add(time.Minute)); outcome != reachability.Stale || len(proof) != 0 {
		t.Fatal("expired higher revision revealed predecessor", outcome)
	}
}

func TestStoreInterruptedStageRecoveryAndForeignRefusal(t *testing.T) {
	raw, target, network, profile, at := signedDescriptor(3)
	for _, stage := range [][]byte{nil, []byte{3, 0, 0, 3}, append([]byte{3, 0}, raw...)} {
		root := t.TempDir()
		config := reachability.StoreConfig{Root: root, Network: network}
		store, err := reachability.OpenStore(config)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.Publish(raw, profile, at, func() error { return nil }); err != nil {
			t.Fatal(err)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		committed := filepath.Join(root, "records", fmt.Sprintf("%x", target))
		before, err := os.ReadFile(committed)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "records", ".record-stage"), stage, 0600); err != nil {
			t.Fatal(err)
		}
		store, err = reachability.OpenStore(config)
		if err != nil {
			t.Fatal("own interrupted staging refused", err)
		}
		got, outcome, err := store.Lookup(target, profile, at)
		if err != nil || outcome != reachability.Accepted || !bytes.Equal(got, raw) {
			t.Fatal("staging changed retained proof", outcome, err)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(committed)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("recovery changed committed floor", err)
		}
		if _, err := os.Lstat(filepath.Join(root, "records", ".record-stage")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("staging removal not completed", err)
		}
		foreign := filepath.Join(root, "records", ".record-foreign")
		if err := os.WriteFile(foreign, []byte{1}, 0600); err != nil {
			t.Fatal(err)
		}
		if owner, err := reachability.OpenStore(config); err == nil {
			_ = owner.Close()
			t.Fatal("foreign entry silently adopted or removed")
		}
		if got, err := os.ReadFile(foreign); err != nil || !bytes.Equal(got, []byte{1}) {
			t.Fatal("foreign evidence deleted", err)
		}
	}
}

func TestStorePublicationConflictRetainsLatestExpiryAcrossReopen(t *testing.T) {
	raw, target, network, profile, at := signedDescriptor(3)
	config := reachability.StoreConfig{Root: t.TempDir(), Network: network}
	store, err := reachability.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	check := func() error { return nil }
	if _, err := store.Publish(raw, profile, at, check); err != nil {
		t.Fatal(err)
	}
	credential := 284 + len("ardents-service-publication-v3\x00")
	for _, hours := range []int{2, 3, 1} {
		conflict := resignDescriptor(raw, func(b []byte) {
			binary.BigEndian.PutUint64(b[credential+114:credential+122], uint64(at.Add(time.Duration(hours)*time.Hour).Unix()))
			b[credential+222] = byte(hours)
		})
		outcome, err := store.Publish(conflict, profile, at, check)
		if err == nil || (outcome != reachability.Conflicting && outcome != reachability.Stale) {
			t.Fatal("Publication conflict accepted", outcome, err)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		store, err = reachability.OpenStore(config)
		if err != nil {
			t.Fatal(err)
		}
		if proof, outcome, err := store.Lookup(target, profile, at); err != nil || outcome != reachability.Conflicting || len(proof) != 0 {
			t.Fatal("reopen revived conflicting Publication", outcome, err)
		}
	}
	for _, hours := range []int{2, 3} {
		start := at.Add(time.Duration(hours) * time.Hour)
		successor := resignDescriptor(raw, func(b []byte) {
			binary.BigEndian.PutUint64(b[credential+98:credential+106], 8)
			binary.BigEndian.PutUint64(b[credential+106:credential+114], uint64(start.Unix()))
			binary.BigEndian.PutUint64(b[credential+114:credential+122], uint64(start.Add(time.Hour).Unix()))
			binary.BigEndian.PutUint64(b[266:274], uint64(start.Unix()))
			binary.BigEndian.PutUint64(b[274:282], uint64(start.Add(600*time.Second).Unix()))
		})
		outcome, err := store.Publish(successor, profile, start, check)
		if hours == 2 {
			if outcome != reachability.Invalid || err == nil {
				t.Fatal("successor overlaps latest conflicting Credential", outcome, err)
			}
		} else {
			if outcome != reachability.Accepted || err != nil {
				t.Fatal("nonoverlapping successor refused", outcome, err)
			}
			if got, outcome, err := store.Lookup(target, profile, start); err != nil || outcome != reachability.Accepted || !bytes.Equal(got, successor) {
				t.Fatal("successor unavailable", outcome, err)
			}
		}
	}
}

func TestStoreCapacityIncludesRecoveryWithoutEviction(t *testing.T) {
	raw, target, network, profile, at := signedDescriptor(3)
	config := reachability.StoreConfig{Root: t.TempDir(), Network: network}
	store, err := reachability.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	check := func() error { return nil }
	for i := 0; i < 128; i++ {
		proof, _, _, _, _ := signedDescriptorForAuthority(3, byte(0x11+i))
		if outcome, err := store.Publish(proof, profile, at, check); err != nil || outcome != reachability.Accepted {
			t.Fatal("Target below capacity refused", i, outcome, err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(config.Root, "records", ".record-stage")
	if err := os.WriteFile(stage, append([]byte{3, 0}, raw...), 0600); err != nil {
		t.Fatal(err)
	}
	store, err = reachability.OpenStore(config)
	if err != nil {
		t.Fatal("full committed population cannot recover own stage", err)
	}
	extra, extraTarget, _, _, _ := signedDescriptorForAuthority(3, 0xfe)
	if outcome, err := store.Publish(extra, profile, at, check); err == nil || outcome != reachability.Invalid {
		t.Fatal("129th Target accepted", outcome, err)
	}
	if _, err := os.Lstat(stage); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("capacity refusal wrote staging", err)
	}
	if got, outcome, err := store.Lookup(extraTarget, profile, at); err != nil || outcome != reachability.Stale || len(got) != 0 {
		t.Fatal("refused Target appeared", outcome, err)
	}
	revised := resignDescriptor(raw, func(b []byte) { binary.BigEndian.PutUint64(b[162:170], 10) })
	if outcome, err := store.Publish(revised, profile, at, check); err != nil || outcome != reachability.Accepted {
		t.Fatal("existing Target cannot advance at capacity", outcome, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = reachability.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 128; i++ {
		proof, key, _, _, _ := signedDescriptorForAuthority(3, byte(0x11+i))
		if key == target {
			proof = revised
		}
		if got, outcome, err := store.Lookup(key, profile, at); err != nil || outcome != reachability.Accepted || !bytes.Equal(got, proof) {
			t.Fatal("capacity/reopen evicted or changed floor", i, outcome, err)
		}
	}
}

func TestStoreUncertainWriteSealsOwnerAndMalformedRecoveryPreservesEvidence(t *testing.T) {
	raw, target, network, profile, at := signedDescriptor(3)
	config := reachability.StoreConfig{Root: t.TempDir(), Network: network}
	store, err := reachability.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	}()
	stage := filepath.Join(config.Root, "records", ".record-stage")
	if err := os.Mkdir(stage, 0700); err != nil {
		t.Fatal(err)
	}
	if outcome, err := store.Publish(raw, profile, at, func() error { return nil }); err == nil || outcome != reachability.Invalid {
		t.Fatal("failed physical write accepted", outcome, err)
	}
	if err := os.Remove(stage); err != nil {
		t.Fatal(err)
	}
	if outcome, err := store.Publish(raw, profile, at, func() error { return nil }); err == nil || outcome != reachability.Invalid {
		t.Fatal("uncertain owner resumed without reopen", outcome, err)
	}
	if proof, outcome, err := store.Lookup(target, profile, at); err == nil || outcome != reachability.Invalid || len(proof) != 0 {
		t.Fatal("uncertain owner returned proof", outcome, err)
	}
	if owner, err := reachability.OpenStore(config); err == nil {
		_ = owner.Close()
		t.Fatal("uncertain owner released lease before Close")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = reachability.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Publish(raw, profile, at, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	committed := filepath.Join(config.Root, "records", fmt.Sprintf("%x", target))
	if err := os.WriteFile(committed, []byte{2, 0, 1}, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stage, []byte{3, 0}, 0600); err != nil {
		t.Fatal(err)
	}
	if owner, err := reachability.OpenStore(config); err == nil {
		_ = owner.Close()
		t.Fatal("damaged committed floor adopted")
	}
	if got, err := os.ReadFile(stage); err != nil || !bytes.Equal(got, []byte{3, 0}) {
		t.Fatal("recovery deleted stage before committed validation", err)
	}
	if got, err := os.ReadFile(committed); err != nil || !bytes.Equal(got, []byte{2, 0, 1}) {
		t.Fatal("damaged evidence changed", err)
	}
}
