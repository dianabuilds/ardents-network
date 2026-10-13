package node

import "path/filepath"

func separateRoots(roots ...string) bool {
	for i, root := range roots {
		if !filepath.IsAbs(root) || filepath.Clean(root) != root {
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
