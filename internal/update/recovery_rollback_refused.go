package update

import "fmt"

func planRollbackRefused(facts inventoryResult, transaction uint64, evidenceNotice string) (recoveryPlan, error) {
	selection, err := decodeCurrent(facts.Current.Bytes)
	if err != nil || selection.Transaction != transaction || selection.Rollback == nil {
		return recoveryPlan{}, fmt.Errorf("%w: repair-required selection is invalid", errPlanInvalid)
	}
	return recoveryPlan{Row: "R23", Outcome: "rollback-refused", State: "repair-required", Generation: transaction,
		CurrentDigest: selection.Current.Artifact, RollbackDigest: selection.Rollback.Artifact, StagingPresent: false,
		SafeNotice: "update rollback refused", EvidenceNotice: evidenceNotice}, nil
}
