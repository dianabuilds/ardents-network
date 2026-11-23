package pathgrammar

import "path/filepath"

// Strict reports whether path is an absolute, already-clean host filesystem
// path: the one strict path-validation grammar shared by the maintained
// owners. It refuses relative paths, empty paths and non-clean spellings
// (redundant separators, dot segments, trailing separators) without converting
// them. Callers that additionally forbid the filesystem root, NUL runes or
// over-length paths compose those bounds around this predicate.
func Strict(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path
}
