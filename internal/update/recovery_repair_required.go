package update

import "fmt"

func planRepairRequired(facts inventoryResult, transaction uint64, evidenceNotice string) (recoveryPlan, error) {
	selection, err := decodeCurrent(facts.Current.Bytes)
	if err != nil || selection.Transaction+1 != transaction || selection.Rollback != nil {
		return recoveryPlan{}, fmt.Errorf("%w: repair-required selection is invalid", errPlanInvalid)
	}
	return recoveryPlan{Row: "R24", Outcome: "repair-required", State: "repair-required", Generation: transaction,
		CurrentDigest: selection.Current.Artifact, StagingPresent: false,
		SafeNotice: "update repair required", EvidenceNotice: evidenceNotice}, nil
}
