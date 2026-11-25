//go:build linux && (endpoint_launch_native || (endpoint_pair_native && document_worker_installed))

package endpoint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
)

// PrepareLaunchRoots initializes the Endpoint history and Hosting root as the installed account.
func PrepareLaunchRoots(t testing.TB, base, program, request, provider string, uid, gid uint32, epoch, profile []byte, inputs, materials [][]byte) {
	t.Helper()
	credential := &syscall.Credential{Uid: uid, Gid: gid, Groups: []uint32{}}

	// Genuine first histories through authenticated offline State acceptance.
	bootstrap := filepath.Join(base, "bootstrap")
	if err := os.Mkdir(bootstrap, 0755); err != nil {
		t.Fatal(err)
	}
	var declaration struct{ Headless json.RawMessage }
	if err := json.Unmarshal([]byte(request), &declaration); err != nil {
		t.Fatal(err)
	}
	historyPlan := filepath.Join(bootstrap, "headless.json")
	WriteLaunchFile(t, historyPlan, declaration.Headless, 0644)
	if len(materials) == 0 {
		t.Fatal("initial Endpoint history requires signed State material")
	}
	bundleRaw, err := source.EncodeBundle(source.Bundle{Epoch: epoch, Inputs: inputs, Materials: materials[:1]})
	if err != nil {
		t.Fatal(err)
	}
	bundlePath, profilePath := filepath.Join(bootstrap, "state.bundle"), filepath.Join(bootstrap, "profile.bin")
	WriteLaunchFile(t, bundlePath, bundleRaw, 0644)
	WriteLaunchFile(t, profilePath, profile, 0644)
	prepared := RunStartupCommand(t, program, credential, "prepared", "endpoint", "prepare-history", historyPlan, bundlePath, profilePath)
	var preparation struct{ Phase string }
	if err := json.Unmarshal(prepared, &preparation); err != nil || preparation.Phase != "history-preparation" {
		t.Fatalf("explicit history preparation phase: %v: %s", err, prepared)
	}

	// Hosting provider budget is an explicit input, initialized under the account.
	period := time.Now().UTC().Truncate(time.Second)
	budgetInput, err := json.Marshal(map[string]any{
		"root": filepath.Join(base, "entry", "hosting"),
		"policy": map[string]any{"provider": provider, "start": period.Add(-time.Minute),
			"end": period.Add(time.Hour), "unit": "GiB", "quantity": 16,
			"directions": "tx+rx", "interfaces": []string{"lo"},
			"initial_used_bytes": 0, "low_watermark_bytes": 1 << 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	budgetPath := filepath.Join(bootstrap, "hosting.json")
	WriteLaunchFile(t, budgetPath, budgetInput, 0644)
	RunStartupCommand(t, program, credential, "completed", "hosting", "initialize", "--config", budgetPath)
}
