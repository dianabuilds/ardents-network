package fixedfile

import "bytes"

// ReplacementPrefixAllowed checks the byte grammar of an interrupted copy.
// Truncation followed by a bounded sequential copy can leave only an old or
// new prefix. A mixed image, extra suffix or unrelated bytes has no provenance.
// This read-only predicate grants no inode custody or mutation authorization.
func ReplacementPrefixAllowed(current, previous, candidate []byte) bool {
	return len(previous) != 0 && len(candidate) != 0 &&
		(bytes.HasPrefix(previous, current) || bytes.HasPrefix(candidate, current))
}
