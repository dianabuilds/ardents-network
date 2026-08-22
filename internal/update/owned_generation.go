package update

import "crypto/sha256"

// ownedGeneration derives the only new transaction generation from current
// owned state. The accepted candidate may replay the current non-bootstrap
// selection; every other candidate receives the immediate successor.
func ownedGeneration(inspection rootInspection, artifact [sha256.Size]byte) uint64 {
	if inspection.selection.Transaction != 0 && inspection.selection.Current.Artifact == artifact {
		return inspection.selection.Transaction
	}
	return inspection.selection.Transaction + 1
}
