package update

func planNoCandidateTerminal(facts inventoryResult, transaction uint64, state transactionState, custodyNotice string,
	hasGenerations, hasStaging, hasTemporaryStaging bool) (recoveryPlan, bool, error) {
	if hasGenerations || hasStaging || hasTemporaryStaging {
		return recoveryPlan{}, false, nil
	}
	switch state {
	case stateRolledBack:
		plan, err := planRolledBack(facts, transaction, custodyNotice)
		return plan, true, err
	case stateRepairRequired:
		plan, err := planRepairRequired(facts, transaction, custodyNotice)
		return plan, true, err
	}
	return recoveryPlan{}, false, nil
}
