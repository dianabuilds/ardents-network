//go:build linux

package publisher

import (
	"path/filepath"
	"strings"
)

func independentRoots(roots ...string) bool {
	for i, root := range roots {
		if !filepath.IsAbs(root) || filepath.Clean(root) != root {
			return false
		}
		for _, other := range roots[:i] {
			if root == other || strings.HasPrefix(root, other+string(filepath.Separator)) || strings.HasPrefix(other, root+string(filepath.Separator)) {
				return false
			}
		}
	}
	return true
}
