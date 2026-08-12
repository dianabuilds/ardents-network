package store

import "testing"

func TestRootDurabilityProbeLeavesNoState(t *testing.T) {
	root := t.TempDir()
	if err := verifyRootWritable(root); err != nil {
		t.Fatal(err)
	}
	entries, err := readBoundedDirectory(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("durability probe left %d entries", len(entries))
	}
}
