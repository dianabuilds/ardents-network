package update

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
)

type planOpKind byte

const (
	opRemoveFile planOpKind = iota + 1
	opRemoveDirectory
	opMoveDirectory
	opAtomicReplaceCurrent
	opSyncDirectory
)

type planOperation struct {
	Kind           planOpKind
	Path, DestPath string
	Payload        []byte
}

type recoveryPlan struct {
	Row                   string
	Outcome, State        string
	Generation            uint64
	CurrentDigest         [32]byte
	RollbackDigest        [32]byte
	StagingPresent        bool
	SafeNotice            string
	Operations            []planOperation
	PredecessorCurrent    []byte
	NeedPredecessorVerify bool
}

var errPlanInvalid = errors.New("update transaction plan is invalid")

// planRecovery is the deterministic pure classifier. It consumes only
// its arguments and never performs a filesystem read, uses no
// package-global state, and does not re-encode a current record when
// exact observed bytes already exist in the inventory. The
// predecessor-current bytes for R10/R11 come from the canonical
// predecessor the inventory already admitted.
func planRecovery(facts inventoryResult, validation journalValidation, records recoveryRecords) (recoveryPlan, error) {
	if len(facts.Transactions) > 1 {
		return recoveryPlan{}, fmt.Errorf("%w: second transaction", errPlanInvalid)
	}
	if len(facts.RollbackRetirement.Bytes) != 0 {
		return planRollbackRetirement(facts)
	}
	if err := validatePhysicalSelection(facts); err != nil {
		return recoveryPlan{}, err
	}
	if err := validateTemporaryStagingBinding(facts, validation); err != nil {
		return recoveryPlan{}, err
	}
	interrupted := facts.InterruptedSelection
	if interrupted == 0 {
		return planIdle(facts), nil
	}
	raws := facts.journalLookup(interrupted)
	if len(raws) == 0 {
		return recoveryPlan{}, fmt.Errorf("%w: missing journal for transaction %d", errPlanInvalid, interrupted)
	}
	if err := journalFirstPredecessorConfirmed(validation, records.predecessorCommitment); err != nil {
		return recoveryPlan{}, err
	}
	plan, err := planClassify(facts, validation, records, interrupted)
	if err != nil {
		return recoveryPlan{}, err
	}
	return applySchemaTempRecovery(plan, facts, validation)
}

// validatePhysicalSelection is the pure semantic boundary between the raw
// inventory and R00-R14 classification. It performs no filesystem operation
// and rejects every contradictory selection or candidate placement before a
// cleanup plan can be constructed.
func validatePhysicalSelection(facts inventoryResult) error {
	selection, err := decodeCurrent(facts.Current.Bytes)
	if err != nil {
		return fmt.Errorf("%w: decode current: %v", errPlanInvalid, err)
	}
	if err := verifyGenerationMatchesTuple(facts, selection.Current); err != nil {
		return err
	}
	if selection.Transaction == 0 {
		if selection.Rollback != nil {
			return fmt.Errorf("%w: predecessor current has rollback", errPlanInvalid)
		}
	} else {
		if selection.Current.Generation != selection.Transaction || selection.Rollback == nil {
			return fmt.Errorf("%w: successor current shape invalid", errPlanInvalid)
		}
		if err := verifyGenerationMatchesTuple(facts, *selection.Rollback); err != nil {
			return err
		}
	}
	if err := validateSchemaSelection(facts, selection); err != nil {
		return err
	}
	if len(facts.Transactions) == 1 {
		transaction := facts.Transactions[0]
		if transaction.Generation == 0 || transaction.Generation != selection.Current.Generation+1 && transaction.Generation != selection.Transaction {
			return fmt.Errorf("%w: transaction generation mismatch", errPlanInvalid)
		}
	}
	if len(facts.StagingDirs) > 1 || len(facts.CurrentTemps) > 1 || len(facts.SchemaTemps) > 1 {
		return fmt.Errorf("%w: multiple staging or current-temp entries", errPlanInvalid)
	}
	if err := validateCandidatePlacement(facts, facts.InterruptedSelection, selection); err != nil {
		return err
	}
	if len(facts.CurrentTemps) == 1 {
		return validateCurrentTemp(facts, selection, facts.InterruptedSelection)
	}
	return nil
}

func validateCandidatePlacement(facts inventoryResult, interrupted uint64, selection currentSelection) error {
	allowed := map[uint64]bool{selection.Current.Generation: true}
	if selection.Rollback != nil {
		allowed[selection.Rollback.Generation] = true
	}
	if interrupted != 0 {
		allowed[interrupted] = true
	}
	for _, generation := range facts.Generations {
		if !allowed[generation.Generation] {
			return fmt.Errorf("%w: unexpected generation %d", errPlanInvalid, generation.Generation)
		}
	}
	for _, staging := range facts.StagingDirs {
		if interrupted == 0 || staging.Generation != interrupted || generationByID(facts.Generations, interrupted) != nil {
			return fmt.Errorf("%w: contradictory staging generation %d", errPlanInvalid, staging.Generation)
		}
	}
	return nil
}

func validateCurrentTemp(facts inventoryResult, current currentSelection, interrupted uint64) error {
	if interrupted == 0 || current.Transaction != 0 || current.Rollback != nil {
		return fmt.Errorf("%w: current temp without predecessor selection", errPlanInvalid)
	}
	candidate := generationByID(facts.Generations, interrupted)
	if candidate == nil {
		return fmt.Errorf("%w: current temp without published candidate", errPlanInvalid)
	}
	temporary, err := decodeCurrent(facts.CurrentTemps[0].Bytes)
	if err != nil || temporary.Transaction != interrupted || temporary.Rollback == nil ||
		!tupleMatchesGeneration(temporary.Current, *candidate) || *temporary.Rollback != current.Current {
		return fmt.Errorf("%w: current temp bytes mismatch", errPlanInvalid)
	}
	return nil
}

func tupleMatchesGeneration(tuple inspectedTuple, generation generationFacts) bool {
	return tuple.Generation == generation.Generation && tuple.Length == uint64(len(generation.Artifact.Bytes)) &&
		tuple.Artifact == sha256.Sum256(generation.Artifact.Bytes) && tuple.Manifest == sha256.Sum256(generation.Manifest.Bytes)
}

func planIdle(facts inventoryResult) recoveryPlan {
	plan := recoveryPlan{Row: "R00", Outcome: "recovered", State: "idle", Generation: 0, CurrentDigest: digestOfGeneration(facts, 0), SafeNotice: "update interrupted"}
	if len(facts.Transactions) == 1 {
		generation := strconv.FormatUint(facts.Transactions[0].Generation, 10)
		plan.Operations = []planOperation{
			{Kind: opRemoveDirectory, Path: filepath.Join("transactions", generation, "journal")},
			{Kind: opSyncDirectory, Path: filepath.Join("transactions", generation)},
			{Kind: opRemoveDirectory, Path: filepath.Join("transactions", generation)},
			{Kind: opSyncDirectory, Path: "transactions"},
		}
	}
	return plan
}

func planClassify(facts inventoryResult, validation journalValidation, records recoveryRecords, transaction uint64) (recoveryPlan, error) {
	predecessorDigest := digestOfGeneration(facts, 0)
	lastState := byte(0)
	for _, e := range validation.Entries {
		lastState = byte(e.State)
	}
	if lastState == 0 {
		return recoveryPlan{}, fmt.Errorf("%w: empty chain", errPlanInvalid)
	}
	hasStaging := hasStagingKind(facts.StagingDirs, transaction, false)
	hasTemporaryStaging := hasStagingKind(facts.StagingDirs, transaction, true)
	hasGenerations := hasOneGen(facts.Generations, transaction)
	last := validation.Entries[len(validation.Entries)-1]
	if last.AdapterResult == adapterFailed && (last.State == stateStopNewWork || last.State == stateDraining) {
		if hasGenerations || hasTemporaryStaging || !hasStaging {
			return recoveryPlan{}, fmt.Errorf("%w: failed adapter physical state is ambiguous", errPlanInvalid)
		}
		return planFailedAdapterAbort(facts, transaction, last.State, predecessorDigest)
	}
	if last.State == stateActivated && last.AdapterResult == adapterUnavailable {
		if hasGenerations || hasTemporaryStaging || !hasStaging {
			return recoveryPlan{}, fmt.Errorf("%w: unavailable activation physical state is ambiguous", errPlanInvalid)
		}
		return planActivationUnavailableAbort(facts, transaction, predecessorDigest)
	}
	if len(facts.Current.Bytes) > 0 {
		if selection, decodeErr := decodeCurrent(facts.Current.Bytes); decodeErr == nil {
			if selection.Transaction == 0 && hasGenerations && lastState >= byte(stateActivated) {
				return recoveryPlan{}, fmt.Errorf("%w: predecessor current with %s prefix", errPlanInvalid, stateNameForByte(lastState))
			}
			if selection.Transaction > 0 && selection.Rollback != nil && hasGenerations && lastState < byte(stateDraining) {
				return recoveryPlan{}, fmt.Errorf("%w: successor current with pre-draining prefix", errPlanInvalid)
			}
		}
	}
	if plan, handled, err := planNoCandidateTerminal(facts, transaction, transactionState(lastState),
		hasGenerations, hasStaging, hasTemporaryStaging); handled {
		return plan, err
	}
	if hasGenerations {
		if lastState == byte(stateRepairRequired) {
			return planRollbackRefused(facts, transaction)
		}
		if lastState == byte(stateRollbackPending) {
			return planRollbackPending(facts, transaction)
		}
		if lastState < byte(stateDraining) {
			return recoveryPlan{}, fmt.Errorf("%w: candidate published before draining", errPlanInvalid)
		}
		if lastState >= byte(stateCommitted) {
			return buildCommittedPlan(facts, transaction)
		}
		if lastState >= byte(stateActivated) {
			return planR12R13(facts, transaction, lastState)
		}
		return planR8ToR11(facts, records, transaction, firstCanonicalTemp(facts.CurrentTemps), predecessorDigest)
	}
	return planEarlyNonterminal(facts, transaction, lastState, hasStaging, hasTemporaryStaging, predecessorDigest)
}

func planEarlyNonterminal(facts inventoryResult, transaction uint64, lastState byte, hasStaging, hasTemporaryStaging bool, predecessorDigest [32]byte) (recoveryPlan, error) {
	state, ok := stateName(lastState)
	if !ok {
		return recoveryPlan{}, fmt.Errorf("%w: invalid state %d", errPlanInvalid, lastState)
	}
	if state == "release-accepted" && !hasStaging && !hasTemporaryStaging {
		return buildRecoveredPlan("R01", state, transaction, predecessorDigest, false), nil
	}
	if state == "artifact-verified" {
		if hasTemporaryStaging {
			plan := buildRecoveredPlan("R03", state, transaction, predecessorDigest, false)
			plan.Operations = stagingRemovalOperations(stagingFacts(facts.StagingDirs, transaction, true))
			return plan, nil
		}
		if hasStaging {
			plan := buildRecoveredPlan("R03", state, transaction, predecessorDigest, false)
			plan.Operations = stagingRemovalOperations(stagingFacts(facts.StagingDirs, transaction, false))
			return plan, nil
		}
		return buildRecoveredPlan("R02", state, transaction, predecessorDigest, false), nil
	}
	if !hasStaging {
		return recoveryPlan{}, fmt.Errorf("%w: missing staging for %s", errPlanInvalid, state)
	}
	return buildRecoveredPlan(rowForNonterminal(state), state, transaction, predecessorDigest, true), nil
}

func stagingRemovalOperations(staging *generationFacts) []planOperation {
	if staging == nil {
		return nil
	}
	name := strconv.FormatUint(staging.Generation, 10)
	if staging.Temporary {
		name += ".tmp"
	}
	operations := make([]planOperation, 0, 6)
	for _, file := range []struct {
		name    string
		present bool
	}{{"artifact", staging.HasArtifact}, {"manifest.bin", staging.HasManifest}} {
		if file.present {
			operations = append(operations,
				planOperation{Kind: opRemoveFile, Path: filepath.Join("staging", name, file.name)},
				planOperation{Kind: opSyncDirectory, Path: filepath.Join("staging", name)},
			)
		}
	}
	return append(operations, []planOperation{
		{Kind: opRemoveDirectory, Path: filepath.Join("staging", name)},
		{Kind: opSyncDirectory, Path: "staging"},
	}...)
}

func hasStagingKind(generations []generationFacts, generation uint64, temporary bool) bool {
	return stagingFacts(generations, generation, temporary) != nil
}

func stagingFacts(generations []generationFacts, generation uint64, temporary bool) *generationFacts {
	for _, candidate := range generations {
		if candidate.Generation == generation && candidate.Temporary == temporary {
			return &candidate
		}
	}
	return nil
}

func planR8ToR11(facts inventoryResult, records recoveryRecords, transaction uint64, currentTemp rawFile, predecessorDigest [32]byte) (recoveryPlan, error) {
	generation := strconv.FormatUint(transaction, 10)
	plan := buildRecoveredPlan("R08", "draining", transaction, predecessorDigest, true)
	if currentTemp.Name != "" {
		plan.Operations = append(plan.Operations,
			planOperation{Kind: opRemoveFile, Path: currentTemp.Name},
			planOperation{Kind: opSyncDirectory, Path: "."},
		)
		plan.Row = "R09"
	}
	if selection, decodeErr := decodeCurrent(facts.Current.Bytes); decodeErr == nil && selection.Rollback != nil {
		plan.Row = "R10-R11"
		plan.NeedPredecessorVerify = true
		if len(records.predecessorCurrent) == 0 {
			return recoveryPlan{}, fmt.Errorf("%w: missing predecessor current bytes", errPlanInvalid)
		}
		plan.PredecessorCurrent = append([]byte(nil), records.predecessorCurrent...)
		plan.Operations = append(plan.Operations,
			planOperation{Kind: opAtomicReplaceCurrent, Payload: plan.PredecessorCurrent},
			planOperation{Kind: opSyncDirectory, Path: "."},
		)
	}
	plan.Operations = append(plan.Operations,
		planOperation{Kind: opMoveDirectory, Path: filepath.Join("generations", generation), DestPath: filepath.Join("staging", generation)},
		planOperation{Kind: opSyncDirectory, Path: "generations"},
		planOperation{Kind: opSyncDirectory, Path: "staging"},
	)
	return plan, nil
}

func planR12R13(facts inventoryResult, transaction uint64, lastState byte) (recoveryPlan, error) {
	state := "activated"
	if lastState == byte(stateSelfTesting) {
		state = "self-testing"
	}
	successor, err := decodeCurrent(facts.Current.Bytes)
	if err != nil {
		return recoveryPlan{}, err
	}
	return recoveryPlan{
		Row: rowForTerminal(state), Outcome: "recovered", State: state, Generation: transaction,
		CurrentDigest: successor.Current.Artifact, RollbackDigest: predecessorRollbackDigest(successor),
		StagingPresent: false, SafeNotice: "update interrupted",
	}, nil
}

func buildCommittedPlan(facts inventoryResult, transaction uint64) (recoveryPlan, error) {
	successor, err := decodeCurrent(facts.Current.Bytes)
	if err != nil {
		return recoveryPlan{}, err
	}
	return recoveryPlan{
		Row: "R14", Outcome: "committed", State: "committed", Generation: transaction,
		CurrentDigest: successor.Current.Artifact, RollbackDigest: predecessorRollbackDigest(successor),
		StagingPresent: false, SafeNotice: "update committed",
	}, nil
}

func digestOfGeneration(facts inventoryResult, generation uint64) [32]byte {
	for _, g := range facts.Generations {
		if g.Generation == generation {
			return sha256.Sum256(g.Artifact.Bytes)
		}
	}
	return [32]byte{}
}

func generationByID(facts []generationFacts, generation uint64) *generationFacts {
	for i := range facts {
		if facts[i].Generation == generation {
			return &facts[i]
		}
	}
	return nil
}

func hasOneGen(facts []generationFacts, generation uint64) bool {
	return generationByID(facts, generation) != nil
}

func firstCanonicalTemp(temps []rawFile) rawFile {
	for _, temp := range temps {
		if temp.Name != "" {
			return temp
		}
	}
	return rawFile{}
}

func predecessorRollbackDigest(selection currentSelection) [32]byte {
	if selection.Rollback == nil {
		return [32]byte{}
	}
	return selection.Rollback.Artifact
}

func stateNameForByte(state byte) string {
	switch transactionState(state) {
	case stateReleaseAccepted:
		return "release-accepted"
	case stateArtifactVerified:
		return "artifact-verified"
	case stateStaged:
		return "staged"
	case stateRollbackReserved:
		return "rollback-reserved"
	case stateStopNewWork:
		return "stop-new-work"
	case stateDraining:
		return "draining"
	case stateActivated:
		return "activated"
	case stateSelfTesting:
		return "self-testing"
	case stateCommitted:
		return "committed"
	case stateRollbackPending:
		return "rollback-pending"
	case stateRolledBack:
		return "rolled-back"
	case stateRepairRequired:
		return "repair-required"
	}
	return "invalid"
}

func stateName(state byte) (string, bool) {
	if name := stateNameForByte(state); name != "invalid" {
		return name, true
	}
	return "", false
}

func rowForNonterminal(state string) string {
	switch state {
	case "staged":
		return "R04"
	case "rollback-reserved":
		return "R05"
	case "stop-new-work":
		return "R06"
	case "draining":
		return "R07"
	}
	return "R07"
}

func rowForTerminal(state string) string {
	if state == "self-testing" {
		return "R13"
	}
	return "R12"
}

func buildRecoveredPlan(row, state string, generation uint64, predecessorDigest [32]byte, staging bool) recoveryPlan {
	return recoveryPlan{Row: row, Outcome: "recovered", State: state, Generation: generation, CurrentDigest: predecessorDigest, RollbackDigest: [32]byte{}, StagingPresent: staging, SafeNotice: "update interrupted"}
}
