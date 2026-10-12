package node

import (
	"path/filepath"
	"testing"
)

func TestOriginalRoleRootsCannotShareOrRebindAPurpose(t *testing.T) {
	parent := t.TempDir()
	participation, spend, budget := filepath.Join(parent, "participation"), filepath.Join(parent, "spend"), filepath.Join(parent, "budget")
	if !separateRoots(participation, spend, budget) {
		t.Fatal("independent declared roots refused")
	}
	for _, roots := range [][]string{{participation, participation, budget}, {parent, spend, budget}, {spend, parent, budget}, {"relative", spend, budget}, {"", spend, budget}} {
		if separateRoots(roots...) {
			t.Fatal("shared or undeclared purpose root accepted", roots)
		}
	}
}
