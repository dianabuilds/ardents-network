package plan

import (
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/pathcheck"
)

func TestOriginalRoleRootsCannotShareOrRebindAPurpose(t *testing.T) {
	parent := t.TempDir()
	participation, spend, budget := filepath.Join(parent, "participation"), filepath.Join(parent, "spend"), filepath.Join(parent, "budget")
	if !pathcheck.DisjointRoots(participation, spend, budget) {
		t.Fatal("independent declared roots refused")
	}
	for _, roots := range [][]string{{participation, participation, budget}, {parent, spend, budget}, {spend, parent, budget}, {"relative", spend, budget}, {"", spend, budget}} {
		if pathcheck.DisjointRoots(roots...) {
			t.Fatal("shared or undeclared purpose root accepted", roots)
		}
	}
	filesystemRoot := filepath.VolumeName(parent) + string(filepath.Separator)
	if pathcheck.DisjointRoots(filesystemRoot, spend, budget) {
		t.Fatal("filesystem root accepted alongside nested purpose roots")
	}
}
