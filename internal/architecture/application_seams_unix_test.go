//go:build !windows

package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceIdentifierUsesPhysicalContainment(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "repository")
	writeApplicationClosureFixture(t, root, "command.go", "package main\nfunc RunParticipant() {}\n")
	physical, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(physical, alias); err != nil {
		t.Fatal(err)
	}
	for _, paths := range [][2]string{{alias, physical}, {physical, alias}} {
		name, found, err := sourceContainsIdentifier(paths[0], paths[1], "command.go", "RunParticipant")
		if err != nil || !found || name != "command.go" {
			t.Fatal("same physical source rejected", name, found, err)
		}
	}
	outside := filepath.Join(parent, "outside")
	writeApplicationClosureFixture(t, outside, "command.go", "package main\nfunc RunParticipant() {}\n")
	escape := filepath.Join(root, "escape")
	if err := os.Symlink(outside, escape); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{outside, escape} {
		if _, _, err := sourceContainsIdentifier(alias, directory, "command.go", "RunParticipant"); err == nil {
			t.Fatal("external source accepted", directory)
		}
	}
}
