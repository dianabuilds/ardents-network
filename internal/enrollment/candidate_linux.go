//go:build linux

package enrollment

import (
	"encoding/hex"
	"errors"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/release"
)

// Candidate contains self-consistent, untrusted distribution bytes. Unlike
// Verified, it has no independent first-install pin provenance. Its manifest
// checksum detects substitution during loading, not authenticity. A successor
// consumer must authenticate both targets against established Release floors.
type Candidate struct {
	Inputs              release.Inputs
	ProtectedDescriptor []byte
	ProtectedFiles      map[string][]byte
}

// ReadHeadlessCandidate loads the existing exact bounded distribution grammar
// without accepting enrollment or a new trust root. It never executes artifacts,
// opens floors, issues permissions or returns a first-install verification.
func ReadHeadlessCandidate(root, executable string, at time.Time) (Candidate, error) {
	if root == "" || executable == "" || at.IsZero() {
		return Candidate{}, errors.New("headless candidate input is incomplete")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return Candidate{}, err
	}
	manifest, err := readEnrollmentFile(filepath.Join(abs, manifestName), false)
	if err != nil {
		return Candidate{}, err
	}
	// The private inventory reader rechecks this checksum when opening files.
	// It is derived locally and never presented as an independently delivered pin.
	request, err := requestFromManifest(abs, executable, manifest, hex.EncodeToString(digest(manifest)), at)
	if err != nil {
		return Candidate{}, err
	}
	loaded, err := verify(request)
	if err != nil {
		return Candidate{}, err
	}
	if loaded.NodeArtifactName == "" || loaded.CustodyArtifactName == "" {
		return Candidate{}, errors.New("headless candidate lacks Node or custody artifact")
	}
	return Candidate{Inputs: loaded.Inputs, ProtectedDescriptor: loaded.ProtectedDescriptor, ProtectedFiles: loaded.ProtectedFiles}, nil
}
