//go:build linux

package endpointcli_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/network/source"

	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

func TestEndpointHistoryCommandPreparesGenuineStateOnce(t *testing.T) {
	binary := buildArdentsNext(t)
	spec, _, _ := networkinputs.PrepareNetworkGenesis(t, t.TempDir())
	signed, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	public := hex.EncodeToString(spec.Authority.Public().(ed25519.PublicKey))
	stateRoot, roleRoot := filepath.Join(root, "state"), filepath.Join(root, "participation")
	stockRoot, entryRoot, clock := filepath.Join(root, "stock"), filepath.Join(root, "route"), filepath.Join(root, "clock")
	for _, path := range []string{stateRoot, roleRoot, stockRoot, entryRoot} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(clock, nil, 0644); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{
		"schema": "ardents-headless-runtime-v2", "role": "reader",
		"network_state_root": stateRoot, "local_role_state_root": roleRoot, "text_token_root": stockRoot,
		"time_confidence_file": clock, "network_id": hex.EncodeToString(spec.NetworkID[:]), "entry_state_root": entryRoot,
		"network_authorities": []string{public}, "closed_profile_authority": public, "network_threshold": 1,
	})
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
	runCommand := func() int {
		command := exec.CommandContext(t.Context(), binary, args...)
		command.Stdout, command.Stderr = &output, &diagnostic
		err := command.Run()
		if err == nil {
			return 0
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		t.Fatalf("run Endpoint history preparation: %v", err)
		return -1
	}
	if code := runCommand(); code != 0 || output.Len() != 0 || diagnostic.String() != "{\"operation\":\"endpoint.prepare-history\",\"phase\":\"history-preparation\",\"outcome\":\"prepared\"}\n" {
		t.Fatal("first preparation", code, output.String(), diagnostic.String())
	}
	diagnostic.Reset()
	if code := runCommand(); code != 1 || diagnostic.String() != "{\"operation\":\"endpoint.prepare-history\",\"phase\":\"history-preparation\",\"outcome\":\"unavailable\"}\n" {
		t.Fatal("repeat preparation", code, diagnostic.String())
	}
	// Duplicate spellings cannot select another root behind the operator's
	// reviewed declaration. This is parser refusal before another domain open.
	raw = append([]byte(`{"role":"reader",`), raw[1:]...)
	if err := os.WriteFile(paths[0], raw, 0600); err != nil {
		t.Fatal(err)
	}
	diagnostic.Reset()
	if code := runCommand(); code != 2 || diagnostic.Len() != 0 {
		t.Fatal("duplicate input accepted", code, diagnostic.String())
	}
}
