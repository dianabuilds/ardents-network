// Package generation owns one exclusively created immutable Installation
// generation directory and its original files, and independent read-only
// Snapshot custody of sealed generations. Prefix independently retains an
// interrupted original directory and its recorded leaves; after caller-owned
// admission and birth journaling it matches complete preimages, repairs only
// original inodes and seals the complete image. It does not adopt creation
// ownership or grant repair authority. Snapshot never takes a writer lease
// or adopts creation ownership; it retains closed inventory, original identities,
// bytes and caller failure through physical close. It grants no Release, selection,
// journal, manager or startup authority. The caller durably records birth before
// every Write through CreateFile, rechecks its original transaction before each
// effect, and closes even
// partially created Owners only after its original physical borrowers join.
// Methods require serialized ownership. A failed operation retains its first
// error and physical descriptors, including each original file birth, until
// Close; same-inode metadata resets refuse. A fresh context cannot renew it.
// Both completed creation writes and Prefix repairs retain a matched read-only
// descriptor to the same inode before joining their writer close; retained
// executable custody cannot block exec. This physical handoff grants no Start.
package generation

import "errors"

var (
	ErrInput   = errors.New("installation generation: invalid input")
	ErrBinding = errors.New("installation generation: binding mismatch")
)
