package routeplan_test

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/routeplan"
)

func TestBridgeNextRequiresExactInitiatorManifest(t *testing.T) {
	manifest := [32]byte{1}
	path := filepath.Join(t.TempDir(), "initiator.json")
	raw := []byte(`{"Role":"initiator","ManifestDigest":"` + hex.EncodeToString(manifest[:]) +
		`","Listen":"203.0.113.9:7443"}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	address, err := routeplan.BridgeNext(path, manifest)
	if err != nil || address != "203.0.113.9:7443" {
		t.Fatalf("BridgeNext() = %q, %v", address, err)
	}
	changed := manifest
	changed[0]++
	if _, err := routeplan.BridgeNext(path, changed); err == nil {
		t.Fatal("changed manifest selected a next leg")
	}
	if bytes.Contains(raw, []byte("certificate")) {
		t.Fatal("fixture unexpectedly contains cross-role credentials")
	}
}
