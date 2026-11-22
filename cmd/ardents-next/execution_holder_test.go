package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/admission"
)

func TestExecutionRouteHolderRefusesRetiredTargetLinkBeforeOpeningRoots(t *testing.T) {
	root := t.TempDir()
	holder := holderPlan{
		Role: admission.AllocationUser,
		Root: filepath.Join(root, "stock"),
		Network: &networkAuthorityPlan{
			Root: filepath.Join(root, "network"), ClockObservationFile: filepath.Join(root, "clock"),
			NetworkID: [32]byte{1}, ProfileAuthority: [32]byte{2}, Authorities: [][32]byte{{3}}, Threshold: 1,
		},
		Route: &routePrefixPlan{
			Domain: 1, EntryRoot: filepath.Join(root, "entry"),
			InteriorRoot: filepath.Join(root, "interior"), HostingRoot: filepath.Join(root, "hosting"),
		},
	}
	raw, err := json.Marshal(map[string]any{
		"generation": [32]byte{4}, "principal": [32]byte{5}, "holder": holder,
		"target_link": "retired-reader-input",
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "execution.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}

	var out, diagnostic bytes.Buffer
	code := runExecutionRouteHolder(t.Context(), []string{"--config", path}, io.NopCloser(bytes.NewReader(nil)), &out, &diagnostic)
	if code != 2 || out.Len() != 0 || diagnostic.Len() != 0 {
		t.Fatalf("retired input: code %d, stdout %q, stderr %q", code, out.String(), diagnostic.String())
	}
	for _, path := range []string{holder.Root, holder.Network.Root, holder.Route.EntryRoot, holder.Route.InteriorRoot, holder.Route.HostingRoot} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("retired input touched %s: %v", path, err)
		}
	}
}
