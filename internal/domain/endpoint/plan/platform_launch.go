package plan

import (
	"errors"
	"path"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/canonicaldigest"
	"github.com/dianabuilds/ardents-network/internal/pathcheck"
)

var errPlatformLaunch = errors.New("invalid Endpoint platform launch")

// PlatformLaunch binds an Endpoint generation to its local configuration and
// declared access. It contains no authenticated state or authorization.
type PlatformLaunch struct {
	Role          string            `json:"role"`
	Distribution  string            `json:"distribution"`
	ReleaseRoot   string            `json:"release_root"`
	Environment   string            `json:"environment"`
	Network       string            `json:"network"`
	Program       string            `json:"program"`
	Configuration string            `json:"configuration"`
	ConfigSHA256  string            `json:"config_sha256"`
	WritableRoots []string          `json:"writable_roots"`
	Inputs        map[string]string `json:"inputs"`
}

// Valid checks the detached Endpoint launch declaration.
func (p PlatformLaunch) Valid() bool {
	if p.Role != "endpoint" {
		return false
	}
	if !PlatformBelow("/etc/ardents", p.Configuration) || !pathcheck.LiteralPOSIXPath(p.Distribution) ||
		!pathcheck.LiteralPOSIXPath(p.ReleaseRoot) || !pathcheck.LiteralPOSIXPath(p.Program) ||
		pathsOverlap(p.Distribution, p.ReleaseRoot) || p.Environment == "" || p.Network == "" || !canonicaldigest.Valid(p.ConfigSHA256) {
		return false
	}
	if len(p.WritableRoots) < 2 || len(p.WritableRoots) > 16 {
		return false
	}
	if path.Base(p.Program) != "ardents-linux-amd64" || !strings.Contains(p.Program, "/generations/") {
		return false
	}
	if len(p.Inputs) != 2 {
		return false
	}
	for credential, digest := range p.Inputs {
		if !PlatformBelow("/etc/ardents", credential) || !canonicaldigest.Valid(digest) {
			return false
		}
	}
	for _, root := range p.WritableRoots {
		if !pathcheck.LiteralPOSIXPath(root) || pathsOverlap(root, p.Distribution) ||
			pathsOverlap(root, p.ReleaseRoot) || pathsOverlap(root, p.Program) {
			return false
		}
	}
	for i, dir := range p.WritableRoots {
		for _, other := range p.WritableRoots[:i] {
			if pathsOverlap(dir, other) {
				return false
			}
		}
	}
	return true
}

// PlatformWritableRoots projects the fixed unit's write permissions from the
// bounded Endpoint declaration. All emitted paths must be literal arguments.
func PlatformWritableRoots(decl Declaration) ([]string, error) {
	paths, err := WritableDirectories(decl)
	if err != nil {
		return nil, errPlatformLaunch
	}
	for _, writable := range paths {
		if !pathcheck.LiteralPOSIXPath(writable) {
			return nil, errPlatformLaunch
		}
	}
	return paths, nil
}

// PlatformSourceCredentials binds the Source credential pair exactly; both remain
// read-only enrolled inputs below /etc/ardents, never a writable semantic root.
func PlatformSourceCredentials(decl Declaration) ([]string, error) {
	paths := []string{decl.Source.ClientCertificate, decl.Source.ClientKey}
	if paths[0] == paths[1] || !PlatformBelow("/etc/ardents", paths[0]) || !PlatformBelow("/etc/ardents", paths[1]) {
		return nil, errPlatformLaunch
	}
	return paths, nil
}

// PlatformBelow tests a literal, strict descendant of an operator path.
func PlatformBelow(parent, value string) bool {
	return pathcheck.LiteralPOSIXPath(value) && strings.HasPrefix(value, parent+"/")
}
