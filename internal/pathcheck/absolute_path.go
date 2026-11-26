package pathcheck

import (
	"path/filepath"
	"strings"
)

// IsCleanAbsolute reports whether path is an absolute, already-clean host filesystem
// path: the one strict path-validation grammar shared by the maintained
// owners. It refuses relative paths, empty paths and non-clean spellings
// (redundant separators, dot segments, trailing separators) without converting
// them. Callers that additionally forbid the filesystem root, NUL runes or
// over-length paths compose those bounds around this predicate.
func IsCleanAbsolute(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path
}

// DisjointRoots checks clean absolute paths for duplicate or nested
// declarations. It compares path spellings without consulting the filesystem.
func DisjointRoots(roots ...string) bool {
	separator := string(filepath.Separator)
	for i, root := range roots {
		if !IsCleanAbsolute(root) {
			return false
		}
		for _, other := range roots[:i] {
			if root == other || containsPath(root, other, separator) || containsPath(other, root, separator) {
				return false
			}
		}
	}
	return true
}

// containsPath reports whether parent is a lexical ancestor of child. A
// filesystem root already ends in a separator, unlike other clean paths.
func containsPath(parent, child, separator string) bool {
	if !strings.HasSuffix(parent, separator) {
		parent += separator
	}
	return strings.HasPrefix(child, parent)
}
