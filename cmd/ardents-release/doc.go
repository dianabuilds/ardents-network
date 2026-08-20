// Command ardents-release owns the product's thin offline-import adapter for
// one local TUF-compatible H3 release decision. It calls internal/releasedecision
// directly, supplies the caller-owned metadata, root, target path, and
// artifact bytes, opens the owned release-decision state root, and prints
// the bounded decision as JSON. It owns no domain state machine and never
// performs a network operation.
package main
