//go:build linux

package introduction

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRetainedReceivingRegistryPreservesClaimAndRejectsErasedHistory(t *testing.T) {
	history, _, root, binding := newHistoryFixture(t)
	now := time.Unix(1_800_000_000, 0).UTC()
	slot := [32]byte{42}
	until := now.Add(time.Minute)
	if err := history.Claim(slot, until, now); err != nil {
		t.Fatal(err)
	}
	if err := history.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, slotHistoryName))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := OpenRetainedReceivingRegistry(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Check(binding); err != nil {
		t.Fatal(err)
	}
	if err := registry.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, slotHistoryName))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("retained opening changed slot/floor bytes", err)
	}
	reopened, err := OpenHistory(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Claim(slot, until, now.Add(time.Second)); err == nil {
		t.Fatal("retained claim reclaimed")
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, slotHistoryName)); err != nil {
		t.Fatal(err)
	}
	if registry, err := OpenRetainedReceivingRegistry(root, binding); err == nil {
		_ = registry.Close()
		t.Fatal("lost history recreated")
	}
	if _, err := os.Stat(filepath.Join(root, slotHistoryName)); !os.IsNotExist(err) {
		t.Fatal("refusal mutated missing history", err)
	}
	if registry, err := OpenRetainedReceivingRegistry(historyTestRoot(t), binding); err == nil {
		_ = registry.Close()
		t.Fatal("empty retained root initialized")
	}
}
