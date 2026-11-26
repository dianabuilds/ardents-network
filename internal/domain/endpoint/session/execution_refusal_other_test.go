//go:build !linux

package session

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/network/state"
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
			holder := LocalHolderPlan{Role: role, Root: roots[1], Network: &state.AuthorityDeclaration{Root: roots[0],
				NetworkID: [32]byte{1}, Authorities: [][32]byte{{2}}, Threshold: 1, ProfileAuthority: [32]byte{2}, ClockObservationFile: filepath.Join(local, "clock")},
				Options: Config{Route: &funding.Plan{HostingRoot: roots[2], EntryRoot: roots[3], InteriorRoot: roots[4]}}}

			input := &executionRefusalInput{}
			var output, diagnostics bytes.Buffer
			if code := RunExecutionHolder(t.Context(), ExecutionHolderPlan{Holder: holder, Generation: [32]byte{3}, Principal: [32]byte{4}}, input, &output, &diagnostics, live); code != 1 {
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
