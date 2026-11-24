package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAdmissionAuthorityDoesNotInitializeMissingHistory(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent root", true: "empty retained root"}[existing], func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "state")
			if existing {
				if err := os.Mkdir(root, 0700); err != nil {
					t.Fatal(err)
				}
			}
			plan := networkAuthorityPlan{Root: root, NetworkID: [32]byte{1}, Authorities: [][32]byte{{2}}, Threshold: 1, ProfileAuthority: [32]byte{2}, ClockObservationFile: filepath.Join(base, "clock")}
			authority, err := openAdmissionAuthority("", &plan)
			if err == nil {
				_ = authority.close()
				t.Fatal("missing history opened as Admission authority")
			}
			entries, err := os.ReadDir(root)
			if existing && (err != nil || len(entries) != 0) {
				t.Fatalf("authority opening modified empty root: %v %v", entries, err)
			}
			if !existing && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("authority opening created missing root: %v", err)
			}
		})
	}
}
