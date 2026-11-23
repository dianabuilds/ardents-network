package node

import (
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
)

// IndependentRoots checks the declared State root together with the role roots.
func (network NodeNetwork) IndependentRoots(roots ...string) bool {
	return separateRoots(append([]string{network.Root}, roots...)...)
}

func separateRoots(roots ...string) bool {
	for i, root := range roots {
		if !pathgrammar.Strict(root) {
			return false
		}
		for j, other := range roots {
			if i != j && (root == other || len(root) > len(other) && root[:len(other)] == other && root[len(other)] == filepath.Separator) {
				return false
			}
		}
	}
	return true
}
