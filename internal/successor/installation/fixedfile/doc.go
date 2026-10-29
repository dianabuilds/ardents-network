// Package fixedfile owns original fixed-file creation and replacement handles.
// Installation admits the path, bytes and effect, records the original inode in
// its durable journal, then calls Commit under the same original caller. This
// Module owns exclusive birth, no-follow opens, original parent/file checks,
// bounded prefix repair, same-inode writes, access and durability through Close.
// It neither reads a journal nor grants Release, recovery, selection or startup
// authority. ReplacementPrefixAllowed exposes only read-only copy grammar,
// shared by actual recovery admission and physical Replace. Complete candidate
// images are resynchronized without truncation, preserving ordered interruption
// prefixes across a further crash. Metadata alone grants no mutation right. Methods are serialized;
// failures retain their first cause and Close never removes or adopts residue.
// ContinueBirth independently retains an already recorded empty root0600 inode
// for same-inode byte delivery/access promotion. It grants no original-cut or
// journal authority, and preserves unknown bytes/inodes instead of repairing
// them. Installation must separately admit and durably record the continuation.
// Create and ContinueBirth also support root:root0444 public files after the
// original empty root0600 birth. This grants no replacement of a complete
// read-only file: Replace retains its 0644/0555/0640 target-mode contract.
package fixedfile

import "errors"

var (
	ErrInput   = errors.New("installation fixed file: invalid input")
	ErrBinding = errors.New("installation fixed file: binding mismatch")
)
