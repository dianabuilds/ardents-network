package update

func planNoCandidateTerminal(facts inventoryResult, transaction uint64, state transactionState, evidenceNotice string,
	hasGenerations, hasStaging, hasTemporaryStaging bool) (recoveryPlan, bool, error) {
	if hasGenerations || hasStaging || hasTemporaryStaging {
		return recoveryPlan{}, false, nil
	}
	switch state {
	case stateRolledBack:
		plan, err := planRolledBack(facts, transaction, evidenceNotice)
		return plan, true, err
	case stateRepairRequired:
		plan, err := planRepairRequired(facts, transaction, evidenceNotice)
		return plan, true, err
	}
	return recoveryPlan{}, false, nil
}
