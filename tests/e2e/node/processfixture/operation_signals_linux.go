//go:build linux && node_process_native

package processfixture

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// WriteOperationMarker is shared by the original process and its independent native workload driver.
func WriteOperationMarker(t *testing.T, root, name string) {
	t.Helper()
	if err := writeOperationMarker(root, name); err != nil {
		t.Fatal(err)
	}
}

func writeOperationMarker(root, name string) error {
	return os.WriteFile(filepath.Join(root, name), []byte("1"), 0600)
}

// WaitOperationMarker is shared by the original process and its independent native workload driver.
func WaitOperationMarker(t *testing.T, root, name string, limit time.Duration) {
	t.Helper()
	deadline := time.NewTimer(limit)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatal("native operation marker unavailable", name)
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		}
	}
}

// IssuerStorageFault is shared by the original process and its independent native workload driver.
func IssuerStorageFault(mode string) bool {
	return mode == "quota-uncertain" || mode == "result-uncertain"
}

// DescriptorRecordCount is shared by the original process and its independent native workload driver.
func DescriptorRecordCount(t *testing.T, root string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "records"))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && len(entry.Name()) == 64 {
			count++
		}
	}
	return count
}

// IntroductionRecordCount is shared by the original process and its independent native workload driver.
func IntroductionRecordCount(t *testing.T, root string) int {
	t.Helper()
	snapshot, err := os.ReadFile(filepath.Join(root, "closed-introduction-slots"))
	if err != nil || len(snapshot) < 120 || string(snapshot[:8]) != "ARDISL01" || (len(snapshot)-120)%40 != 0 {
		t.Fatal("native History snapshot unavailable", err)
	}
	return (len(snapshot) - 120) / 40
}
