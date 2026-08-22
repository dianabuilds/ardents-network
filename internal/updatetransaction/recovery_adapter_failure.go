package updatetransaction

import (
	"fmt"
	"path/filepath"
	"strconv"
)

// planFailedAdapterAbort is deliberately narrower than ordinary interrupted
// recovery: only a durably recorded failure at StopNewWork or Drain permits an
// immediate abort. A success-coded prefix remains interrupted because recovery
// cannot infer the external work-control state.
func planFailedAdapterAbort(facts inventoryResult, transaction uint64, state transactionState,
	predecessorDigest [32]byte, custodyNotice string) (recoveryPlan, error) {
	resultState := ""
	switch state {
	case stateStopNewWork:
		resultState = "rollback-reserved"
	case stateDraining:
		resultState = "stop-new-work"
	default:
		return recoveryPlan{}, fmt.Errorf("%w: failed adapter at %s", errPlanInvalid, stateNameForByte(byte(state)))
	}
	staging := stagingFacts(facts.StagingDirs, transaction, false)
	if staging == nil {
		return recoveryPlan{}, fmt.Errorf("%w: failed adapter staging absent", errPlanInvalid)
	}
	plan := recoveryPlan{Row: "S7.2-04-adapter-failure", Outcome: "drain-expired", State: resultState,
		Generation: transaction, CurrentDigest: predecessorDigest, StagingPresent: false,
		SafeNotice: "update drain expired", CustodyNotice: custodyNotice}
	plan.Operations = append(plan.Operations, stagingRemovalOperations(staging)...)
	transactionName := strconv.FormatUint(transaction, 10)
	for entryState := stateReleaseAccepted; entryState <= state; entryState++ {
		name, err := journalFileName(entryState)
		if err != nil {
			return recoveryPlan{}, err
		}
		plan.Operations = append(plan.Operations,
			planOperation{Kind: opRemoveFile, Path: filepath.Join("transactions", transactionName, "journal", name)},
			planOperation{Kind: opSyncDirectory, Path: filepath.Join("transactions", transactionName, "journal")},
		)
	}
	plan.Operations = append(plan.Operations,
		planOperation{Kind: opRemoveDirectory, Path: filepath.Join("transactions", transactionName, "journal")},
		planOperation{Kind: opSyncDirectory, Path: filepath.Join("transactions", transactionName)},
		planOperation{Kind: opRemoveDirectory, Path: filepath.Join("transactions", transactionName)},
		planOperation{Kind: opSyncDirectory, Path: "transactions"},
	)
	return plan, nil
}
