//go:build linux && ext4_identity_native

package duty

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestParticipationInspectionRejectsOtherExtFilesystems(t *testing.T) {
	for _, kind := range []string{"ext2", "ext3"} {
		t.Run(kind, func(t *testing.T) {
			root := os.Getenv("ARDENTS_TEST_" + strings.ToUpper(kind))
			if root == "" {
				t.Fatal("invalid environment: filesystem root required")
			}
			reported, err := exec.Command("findmnt", "--kernel", "--noheadings", "--output", "FSTYPE", "--target", root).Output()
			if err != nil || strings.TrimSpace(string(reported)) != kind {
				t.Fatalf("invalid environment: want %s mount, got %q: %v", kind, reported, err)
			}
			original, err := os.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			defer original.Close()
			inspection := &participationInspection{directory: original}
			if err := inspection.nativeRoot(); err == nil || !strings.Contains(err.Error(), "retained participation filesystem differs") {
				t.Fatalf("original non-ext4 mount must refuse at filesystem boundary: %v", err)
			}
		})
	}
}
