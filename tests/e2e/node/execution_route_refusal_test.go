package node_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestExecutionRouteHolderRefusesRetiredTargetLinkBeforeOpeningRoots(t *testing.T) {
	root := t.TempDir()
	stock, network := filepath.Join(root, "stock"), filepath.Join(root, "network")
	entry, interior, hosting := filepath.Join(root, "entry"), filepath.Join(root, "interior"), filepath.Join(root, "hosting")
	authority := state.AuthorityDeclaration{Root: network, ClockObservationFile: filepath.Join(root, "clock"),
		NetworkID: [32]byte{1}, ProfileAuthority: [32]byte{2}, Authorities: [][32]byte{{3}}, Threshold: 1}
	holder := map[string]any{"role": admission.AllocationUser, "root": stock, "network": authority,
		"route": map[string]any{"domain": 1, "entry_root": entry, "interior_root": interior, "hosting_root": hosting}}
	config := map[string]any{"generation": [32]byte{4}, "principal": [32]byte{5}, "holder": holder}
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "execution.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	binary := buildNodeCommand(t)
	run := func() (error, string, string) {
		command := exec.CommandContext(t.Context(), binary, "execution", "route-holder", "--config", path)
		command.Stdin = strings.NewReader("")
		var out, diagnostic bytes.Buffer
		command.Stdout, command.Stderr = &out, &diagnostic
		err := command.Run()
		return err, out.String(), diagnostic.String()
	}
	var exit *exec.ExitError
	if err, out, diagnostic := run(); !errors.As(err, &exit) || exit.ExitCode() != 1 || out != "" || diagnostic != "" {
		t.Fatalf("valid declaration did not reach host refusal: error=%v, stdout=%q, stderr=%q", err, out, diagnostic)
	}
	config["target_link"] = "retired-reader-input"
	raw, err = json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err, out, diagnostic := run(); !errors.As(err, &exit) || exit.ExitCode() != 2 || out != "" || diagnostic != "" {
		t.Fatalf("retired input: error=%v, stdout=%q, stderr=%q", err, out, diagnostic)
	}
	for _, path := range []string{stock, network, entry, interior, hosting} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("retired input touched %s: %v", path, err)
		}
	}
}
