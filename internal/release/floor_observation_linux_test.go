//go:build linux

package release

import (
	"path/filepath"
	"testing"
)

func TestCurrentFloorsDoesNotAliasTrustOrReopenClosedLease(t *testing.T) {
	verifier, err := Open(filepath.Join(t.TempDir(), "floors"))
	if err != nil {
		t.Fatal(err)
	}
	defer verifier.Close()
	initial, err := verifier.CurrentFloors()
	if err != nil || initial.RootVersion != 0 {
		t.Fatal("empty floor observation invented trust")
	}
	repo := newSyntheticRepository(t, syntheticOptions{})
	decision := evaluateWithRepo(t, repo, verifier.store, defaultLocalEnvironment(testRefTime))
	if _, ok := decision.Authorization(); !ok {
		t.Fatal("fixture did not establish trusted floors")
	}
	observed, err := verifier.CurrentFloors()
	if err != nil || !floorSetEqual(observed, decision.Floors) {
		t.Fatal("floor observation disagrees with the held store")
	}
	observed.RootDigest[0] ^= 0xff
	observed.TargetsDigest[0] ^= 0xff
	again, err := verifier.CurrentFloors()
	if err != nil || !floorSetEqual(again, decision.Floors) {
		t.Fatal("caller mutation changed established floor observations")
	}
	if err := verifier.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.CurrentFloors(); err == nil {
		t.Fatal("closed verifier kept exposing an unheld trust state")
	}
}
