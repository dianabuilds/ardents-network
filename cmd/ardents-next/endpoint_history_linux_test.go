//go:build linux

package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/network/source"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func TestEndpointHistoryCommandPreparesGenuineStateOnce(t *testing.T) {
	f := newNetworkAdmissionFixture(t)
	signed, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	public := hex.EncodeToString(f.spec.Authority.Public().(ed25519.PublicKey))
	plan := endpoint.HeadlessPlan{Schema: "ardents-headless-runtime-v2", Role: "reader",
		NetworkStateRoot: filepath.Join(root, "state"), LocalRoleStateRoot: filepath.Join(root, "participation"), ApplicationTokenRoot: filepath.Join(root, "stock"),
		TimeConfidenceFile: filepath.Join(root, "clock"), NetworkID: hex.EncodeToString(f.spec.NetworkID[:]),
		EntryStateRoot:     filepath.Join(root, "route"),
		NetworkAuthorities: []string{public}, ClosedProfileAuthority: public, NetworkThreshold: 1}
	for _, path := range []string{plan.NetworkStateRoot, plan.LocalRoleStateRoot, plan.ApplicationTokenRoot, plan.EntryStateRoot} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(plan.TimeConfidenceFile, nil, 0644); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := source.EncodeBundle(source.Bundle{Epoch: signed.Epoch.Raw, Inputs: signed.Epoch.Inputs, Materials: signed.Epoch.Materials[:1]})
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{filepath.Join(root, "headless.json"), filepath.Join(root, "state.bundle"), filepath.Join(root, "profile.bin")}
	for i, data := range [][]byte{raw, bundle, signed.Profile} {
		if err := os.WriteFile(paths[i], data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	args := append([]string{"endpoint", "prepare-history"}, paths...)
	var output, diagnostic bytes.Buffer
	if code := run(t.Context(), args, &output, &diagnostic); code != 0 || output.Len() != 0 || diagnostic.String() != "{\"outcome\":\"prepared\",\"phase\":\"history-preparation\"}\n" {
		t.Fatal("first preparation", code, output.String(), diagnostic.String())
	}
	diagnostic.Reset()
	if code := run(t.Context(), args, &output, &diagnostic); code != 1 || diagnostic.String() != "{\"outcome\":\"unavailable\",\"phase\":\"history-preparation\"}\n" {
		t.Fatal("repeat preparation", code, diagnostic.String())
	}
	// Duplicate spellings cannot select another root behind the operator's
	// reviewed declaration. This is parser refusal before another domain open.
	raw = append([]byte(`{"role":"reader",`), raw[1:]...)
	if err := os.WriteFile(paths[0], raw, 0600); err != nil {
		t.Fatal(err)
	}
	diagnostic.Reset()
	if code := run(t.Context(), args, &output, &diagnostic); code != 2 || diagnostic.Len() != 0 {
		t.Fatal("duplicate input accepted", code, diagnostic.String())
	}
}
