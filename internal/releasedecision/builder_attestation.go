package releasedecision

import (
	"encoding/hex"
	"errors"
)

// builderAttestation is an authenticated H3 rebuild record. H3 deliberately
// makes no independent-builder claim: Targets signs these project-controlled
// records, and every record is bound to the exact target and build inputs.
type builderAttestation struct {
	BuilderIdentity      string `json:"builder_identity"`
	BuildIdentity        string `json:"build_identity"`
	SourceRevision       string `json:"source_revision"`
	BuildInputCommitment string `json:"build_input_commitment"`
	TargetSHA256         string `json:"target_sha256"`
}

func validateBuilderAttestations(value targetIdentityDescriptor) error {
	if len(value.BuilderAttestations) != 2 {
		return errors.New("target identity requires two builder attestations")
	}
	first := value.BuilderAttestations[0]
	second := value.BuilderAttestations[1]
	if first.BuilderIdentity == "" || second.BuilderIdentity == "" ||
		first.BuilderIdentity == second.BuilderIdentity {
		return errors.New("target identity requires two distinct builder identities")
	}
	for _, record := range value.BuilderAttestations {
		if record.BuildIdentity != value.BuildIdentity ||
			record.SourceRevision != value.SourceRevision ||
			record.BuildInputCommitment != value.BuildInputCommitment {
			return errors.New("builder attestation does not match the authenticated build inputs")
		}
		decoded, err := hex.DecodeString(record.TargetSHA256)
		if err != nil || len(decoded) != 32 {
			return errors.New("builder attestation has an invalid target digest")
		}
	}
	return nil
}

func verifyBuilderAttestations(value targetIdentityDescriptor, digest []byte) error {
	expected := hex.EncodeToString(digest)
	for _, record := range value.BuilderAttestations {
		if record.TargetSHA256 != expected {
			return errors.New("builder attestation does not match the target digest")
		}
	}
	return nil
}
