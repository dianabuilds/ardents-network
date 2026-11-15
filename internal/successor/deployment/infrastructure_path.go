package deployment

import (
	"path"
	"strings"
)

// ValidInfrastructurePath checks a declared path in the installed Linux
// namespace. It performs no filesystem observation and grants no custody.
func ValidInfrastructurePath(value string) bool {
	if !path.IsAbs(value) || path.Clean(value) != value || value == "/" || len(value) > 4096 || len(strings.Split(value, "/")) > 128 {
		return false
	}
	for _, c := range value {
		if c < 32 || c > 126 || strings.ContainsRune("%\\\"'*?[]{}$`", c) {
			return false
		}
	}
	return true
}

// InfrastructurePathContains checks component boundaries in that namespace.
func InfrastructurePathContains(parent, child string) bool {
	return parent == child || strings.HasPrefix(child, parent+"/")
}

// IndependentNodePaths validates declared runtime paths without accessing them.
// Each semantic root and input must remain distinct; shared parent directories
// are allowed. Filesystem identity and custody require independent observation.
func IndependentNodePaths(paths ...string) bool {
	for i, value := range paths {
		if !ValidInfrastructurePath(value) {
			return false
		}
		for _, previous := range paths[:i] {
			if InfrastructurePathContains(value, previous) || InfrastructurePathContains(previous, value) {
				return false
			}
		}
	}
	return true
}
