package updatetransaction

import "fmt"

func planRolledBack(facts inventoryResult, transaction uint64, custodyNotice string) (recoveryPlan, error) {
	selection, err := decodeCurrent(facts.Current.Bytes)
	if err != nil || selection.Transaction+1 != transaction || selection.Rollback != nil {
		return recoveryPlan{}, fmt.Errorf("%w: rolled-back selection is invalid", errPlanInvalid)
	}
	return recoveryPlan{Row: "R22", Outcome: "rolled-back", State: "rolled-back", Generation: transaction,
		CurrentDigest: selection.Current.Artifact, StagingPresent: false,
		SafeNotice: "update rolled back", CustodyNotice: custodyNotice}, nil
}
