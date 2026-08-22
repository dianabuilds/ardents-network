package update

import (
	"crypto/sha256"
	"fmt"
)

func validateTemporaryStagingBinding(facts inventoryResult, validation journalValidation) error {
	for _, staging := range facts.StagingDirs {
		if !staging.Temporary {
			continue
		}
		if len(validation.Entries) == 0 {
			return fmt.Errorf("%w: temporary staging without journal", errPlanInvalid)
		}
		expected := validation.Entries[0]
		if staging.HasArtifact && sha256.Sum256(staging.Artifact.Bytes) != expected.ArtifactDigest {
			return fmt.Errorf("%w: temporary artifact mismatch", errPlanInvalid)
		}
		if staging.HasManifest && (sha256.Sum256(staging.Manifest.Bytes) != expected.ManifestCommitment ||
			staging.DecodedManifest.Artifact != expected.ArtifactDigest) {
			return fmt.Errorf("%w: temporary manifest mismatch", errPlanInvalid)
		}
	}
	return nil
}
