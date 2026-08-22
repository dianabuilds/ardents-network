package updatetransaction

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
)

// planRollbackRetirement completes the one marker-bound retention transition.
// It is deliberately independent of ordinary transaction planning: no new
// candidate exists yet and a marker is never accepted beside a new journal.
func planRollbackRetirement(facts inventoryResult, custody string) (recoveryPlan, error) {
	retirement, err := decodeRollbackRetirement(facts.RollbackRetirement.Bytes)
	if err != nil {
		return recoveryPlan{}, fmt.Errorf("%w: rollback retirement: %v", errPlanInvalid, err)
	}
	previous, err := decodeCurrent(retirement.PreviousCurrent)
	if err != nil || previous.Rollback == nil {
		return recoveryPlan{}, fmt.Errorf("%w: retirement predecessor", errPlanInvalid)
	}
	current, err := decodeCurrent(facts.Current.Bytes)
	if err != nil {
		return recoveryPlan{}, fmt.Errorf("%w: retirement current", errPlanInvalid)
	}
	oldCurrent := bytes.Equal(facts.Current.Bytes, retirement.PreviousCurrent)
	newCurrent := current.Transaction == previous.Transaction && current.Rollback == nil && current.Current == previous.Current
	if !oldCurrent && !newCurrent {
		return recoveryPlan{}, fmt.Errorf("%w: retirement current does not bind marker", errPlanInvalid)
	}
	if generationByID(facts.Generations, previous.Current.Generation) == nil {
		return recoveryPlan{}, fmt.Errorf("%w: retirement current generation missing", errPlanInvalid)
	}
	if len(facts.CurrentTemps) > 1 {
		return recoveryPlan{}, fmt.Errorf("%w: multiple retirement current temps", errPlanInvalid)
	}
	if len(facts.CurrentTemps) == 1 {
		temporary, tempErr := decodeCurrent(facts.CurrentTemps[0].Bytes)
		if tempErr != nil || temporary.Transaction != previous.Transaction || temporary.Rollback != nil || temporary.Current != previous.Current {
			return recoveryPlan{}, fmt.Errorf("%w: retirement current temp does not bind marker", errPlanInvalid)
		}
	}
	retired := generationByID(facts.Generations, previous.Rollback.Generation)
	if oldCurrent && retired == nil {
		return recoveryPlan{}, fmt.Errorf("%w: retirement rollback generation missing", errPlanInvalid)
	}
	if len(facts.Transactions) == 1 && facts.Transactions[0].Generation != previous.Transaction {
		return recoveryPlan{}, fmt.Errorf("%w: retirement transaction mismatch", errPlanInvalid)
	}
	plan := recoveryPlan{Row: "R-retire", Outcome: "recovered", State: "idle", Generation: previous.Transaction,
		CurrentDigest: previous.Current.Artifact, SafeNotice: "update interrupted", CustodyNotice: custody}
	if len(facts.CurrentTemps) == 1 {
		plan.Operations = append(plan.Operations, planOperation{Kind: opRemoveFile, Path: facts.CurrentTemps[0].Name})
	}
	if oldCurrent {
		raw, encodeErr := encodeCurrent(currentSelection{Transaction: previous.Transaction, Current: previous.Current})
		if encodeErr != nil {
			return recoveryPlan{}, encodeErr
		}
		plan.Operations = append(plan.Operations,
			planOperation{Kind: opAtomicReplaceCurrent, Payload: raw},
			planOperation{Kind: opSyncDirectory, Path: "."})
	}
	if retired != nil {
		plan.Operations = append(plan.Operations, generationRemovalOperations("generations", previous.Rollback.Generation)...)
	}
	if len(facts.Transactions) == 1 {
		plan.Operations = append(plan.Operations, transactionRemovalOperations(facts.Transactions[0])...)
	}
	plan.Operations = append(plan.Operations,
		planOperation{Kind: opRemoveFile, Path: rollbackRetireName},
		planOperation{Kind: opSyncDirectory, Path: "."})
	return plan, nil
}

func generationRemovalOperations(kind string, generation uint64) []planOperation {
	directory := filepath.Join(kind, strconv.FormatUint(generation, 10))
	return []planOperation{
		{Kind: opRemoveFile, Path: filepath.Join(directory, "artifact")},
		{Kind: opRemoveFile, Path: filepath.Join(directory, "manifest.bin")},
		{Kind: opRemoveDirectory, Path: directory},
		{Kind: opSyncDirectory, Path: kind},
	}
}

func transactionRemovalOperations(transaction transactionFacts) []planOperation {
	directory := filepath.Join("transactions", strconv.FormatUint(transaction.Generation, 10))
	names := make([]string, 0, len(transaction.Journal))
	for name := range transaction.Journal {
		names = append(names, name)
	}
	sort.Strings(names)
	operations := make([]planOperation, 0, len(names)+3)
	for _, name := range names {
		operations = append(operations, planOperation{Kind: opRemoveFile, Path: filepath.Join(directory, "journal", name)})
	}
	return append(operations,
		planOperation{Kind: opRemoveDirectory, Path: filepath.Join(directory, "journal")},
		planOperation{Kind: opRemoveDirectory, Path: directory},
		planOperation{Kind: opSyncDirectory, Path: "transactions"})
}
