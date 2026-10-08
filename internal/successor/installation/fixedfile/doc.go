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
package fixedfile

import "errors"

var (
	ErrInput   = errors.New("installation fixed file: invalid input")
	ErrBinding = errors.New("installation fixed file: binding mismatch")
)
