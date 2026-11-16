package main

import (
	"os"
	"path/filepath"
	"strings"
)

// Check path declarations before Release can create or advance history.
// Authentication and Installation still retain their own independent native
// ancestors, immutable inputs and original verifier custody.
func independentReleasePaths(distribution, history string) bool {
	for _, path := range []string{distribution, history} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" || strings.ContainsRune(path, 0) {
			return false
		}
		for p := path; ; p = filepath.Dir(p) {
			info, err := os.Lstat(p)
			if err != nil && !os.IsNotExist(err) {
				return false
			}
			if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
				return false
			}
			if p == "/" {
				break
			}
		}
	}
	inside := func(parent, child string) bool {
		relative, err := filepath.Rel(parent, child)
		return err == nil && (relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
	}
	return !inside(distribution, history) && !inside(history, distribution)
}
