//go:build installation_native

package generation

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// This measures only fresh range reads/comparison through an actual descriptor;
// it supplies no sealed custody, Release or installed-startup authority.
func BenchmarkSnapshotFreshBytes(b *testing.B) {
	expected := make([]byte, 29<<20)
	for index := range expected {
		expected[index] = byte(index*31 + index/251)
	}
	path := filepath.Join(b.TempDir(), "program")
	if err := os.WriteFile(path, expected, 0600); err != nil {
		b.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	defer file.Close()
	snapshot := &Snapshot{ctx: context.Background()}
	scratch := make([]byte, 2*(256<<10))
	b.SetBytes(int64(len(expected)))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := snapshot.compareBytes(file, expected, scratch); err != nil {
			b.Fatal(err)
		}
	}
}
