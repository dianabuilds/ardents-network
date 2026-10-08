//go:build !linux

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

type executionRefusalInput struct{ reads int }

func (input *executionRefusalInput) Read([]byte) (int, error) {
	input.reads++
	return 0, io.EOF
}
func (*executionRefusalInput) Close() error { return nil }

func TestUnsupportedExecutionConsumerRefusesBeforeParticipantEffects(t *testing.T) {
	for _, role := range []admission.AllocationRole{admission.AllocationUser, admission.AllocationPublisher} {
		for _, live := range []bool{false, true} {
			local := t.TempDir()
			roots := []string{filepath.Join(local, "network"), filepath.Join(local, "holder"), filepath.Join(local, "hosting"), filepath.Join(local, "entry"), filepath.Join(local, "interior")}
			holder := holderPlan{Role: role, Root: roots[1], Network: &networkAuthorityPlan{Root: roots[0],
				NetworkID: [32]byte{1}, Authorities: [][32]byte{{2}}, Threshold: 1, ProfileAuthority: [32]byte{2}, ClockObservationFile: filepath.Join(local, "clock")},
				Route: &routePrefixPlan{HostingRoot: roots[2], EntryRoot: roots[3], InteriorRoot: roots[4]}}
			path := filepath.Join(local, "execution.json")
			body, err := json.Marshal(map[string]any{"generation": [32]byte{3}, "principal": [32]byte{4}, "holder": holder})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			input := &executionRefusalInput{}
			var output, diagnostics bytes.Buffer
			if code := runExecutionHolder(t.Context(), []string{"--config", path}, input, &output, &diagnostics, live); code != 1 {
				t.Fatalf("unsupported qualified launch returned %d", code)
			}
			if input.reads != 0 || output.Len() != 0 {
				t.Fatal("unsupported worker reached the participant console")
			}
			for _, root := range roots {
				if _, err := os.Lstat(root); !os.IsNotExist(err) {
					t.Fatal("unsupported worker touched participant root", root, err)
				}
			}
		}
	}
}
