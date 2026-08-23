package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/release"
	"github.com/dianabuilds/ardents-network/internal/update"
)

type jsonQuoter struct{ err error }

func (quoter *jsonQuoter) value(value string) string {
	encoded, err := json.Marshal(value)
	quoter.err = errors.Join(quoter.err, err)
	return string(encoded)
}
func renderDecision(decision release.Decision) ([]byte, error) {
	quoter := &jsonQuoter{}
	floors := decision.Floors
	format := `{"schema":"ardents-release-decision-v1","outcome":%s,"path":%s,"length":%d,"digest":%s,"platform":%s,"architecture":%s,"environment":%s,"network":%s,"release_identity":%s,"release_version":%d,"source_revision":%s,"build_input_commitment":%s,"build_identity":%s,"dependency_identity":%s,"sbom_identity":%s,"attestation_policy":%s,"qualification":%s,"build_state":%s,"protocol_phase":%s,"build_safety":%s,"protocol":%s,"root_version":%d,"floors":{"root_version":%d,"root_digest":%s,"timestamp_version":%d,"timestamp_digest":%s,"snapshot_version":%d,"snapshot_digest":%s,"targets_version":%d,"targets_digest":%s},"notice":%s,"custody_notice":%s}` + "\n"
	rendered := fmt.Sprintf(format, quoter.value(string(decision.Outcome)), quoter.value(decision.Path), decision.Length,
		quoter.value(hex.EncodeToString(decision.Digest)), quoter.value(decision.Platform), quoter.value(decision.Architecture),
		quoter.value(decision.Environment), quoter.value(decision.Network), quoter.value(decision.ReleaseIdentity), decision.ReleaseVersion,
		quoter.value(decision.SourceRevision), quoter.value(decision.BuildInputCommitment), quoter.value(decision.BuildIdentity),
		quoter.value(decision.DependencyIdentity), quoter.value(decision.SBOMIdentity), quoter.value(decision.AttestationPolicy),
		quoter.value(decision.Qualification), quoter.value(decision.BuildState), quoter.value(decision.ProtocolPhase),
		quoter.value(string(decision.BuildSafety)), quoter.value(string(decision.Protocol)), decision.RootVersion,
		floors.RootVersion, quoter.value(hex.EncodeToString(floors.RootDigest)),
		floors.TimestampVersion, quoter.value(hex.EncodeToString(floors.TimestampDigest)),
		floors.SnapshotVersion, quoter.value(hex.EncodeToString(floors.SnapshotDigest)),
		floors.TargetsVersion, quoter.value(hex.EncodeToString(floors.TargetsDigest)),
		quoter.value(decision.Notice), quoter.value(decision.EvidenceNotice))
	return []byte(rendered), quoter.err
}
func renderUpdateResult(result update.Result) ([]byte, error) {
	quoter := &jsonQuoter{}
	format := `{"schema":"ardents-update-result-v1","outcome":%s,"state":%s,"transaction_generation":%d,"current_sha256":%s,"rollback_sha256":%s,"staging_present":%t,"safe_notice":%s,"custody_notice":%s}` + "\n"
	rendered := fmt.Sprintf(format, quoter.value(result.Outcome), quoter.value(result.State), result.Generation,
		quoter.value(hex.EncodeToString(result.CurrentDigest[:])), quoter.value(hex.EncodeToString(result.RollbackDigest[:])),
		result.StagingPresent, quoter.value(result.SafeNotice), quoter.value(result.EvidenceNotice))
	return []byte(rendered), quoter.err
}
