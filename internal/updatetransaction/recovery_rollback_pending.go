package updatetransaction

import "fmt"

// planRollbackPending preserves the forward-selected candidate until a later
// explicit Apply has supplied and validated the retained-payload decision.
func planRollbackPending(facts inventoryResult, transaction uint64, custodyNotice string) (recoveryPlan, error) {
	selection, err := decodeCurrent(facts.Current.Bytes)
	if err != nil || selection.Transaction != transaction || selection.Rollback == nil {
		return recoveryPlan{}, fmt.Errorf("%w: rollback-pending selection is invalid", errPlanInvalid)
	}
	return recoveryPlan{Row: "R18", Outcome: "recovered", State: "rollback-pending", Generation: transaction,
		CurrentDigest: selection.Current.Artifact, RollbackDigest: selection.Rollback.Artifact, StagingPresent: false,
		SafeNotice: "update interrupted", CustodyNotice: custodyNotice}, nil
}
