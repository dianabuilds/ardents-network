// Package runtimeimage owns immutable runtime image reference identity.
// It does not own image pulling, container execution, rollout, or provenance.
package runtimeimage

import "github.com/distribution/reference"

const maxReferenceBytes = 512

// ValidReference reports whether value is one canonical sha256-digested image
// name suitable for equality across configuration and deployment manifests.
func ValidReference(value string) bool {
	if len(value) == 0 || len(value) > maxReferenceBytes {
		return false
	}
	named, err := reference.ParseNormalizedNamed(value)
	if err != nil || named.String() != value {
		return false
	}
	digested, ok := named.(reference.Digested)
	return ok && digested.Digest().Algorithm().String() == "sha256"
}
