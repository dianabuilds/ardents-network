package updatetransaction

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
	CustodyNotice         string
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
func planRecovery(facts inventoryResult, validation journalValidation, records recoveryRecords, custodyNotice string) (recoveryPlan, error) {
	if len(facts.Transactions) > 1 {
		return recoveryPlan{}, fmt.Errorf("%w: second transaction", errPlanInvalid)
	}
	if err := validatePhysicalSelection(facts); err != nil {
		return recoveryPlan{}, err
	}
	if err := validateTemporaryStagingBinding(facts, validation); err != nil {
		return recoveryPlan{}, err
	}
	interrupted := facts.InterruptedSelection
	if interrupted == 0 {
		return planIdle(facts, custodyNotice), nil
	}
	raws := facts.journalLookup(interrupted)
	if len(raws) == 0 {
		return recoveryPlan{}, fmt.Errorf("%w: missing journal for transaction %d", errPlanInvalid, interrupted)
	}
	if err := journalFirstPredecessorConfirmed(validation, records.predecessorCommitment); err != nil {
		return recoveryPlan{}, err
	}
	return planClassify(facts, validation, records, interrupted, custodyNotice)
}

func validateTemporaryStagingBinding(facts inventoryResult, validation journalValidation) error {
	for _, staging := range facts.StagingDirs {
		if !staging.Temporary {
			continue
		}
		if len(validation.Entries) == 0 {
			return fmt.Errorf("%w: temporary staging without journal", errPlanInvalid)
		}
		expected := validation.Entries[0]
		if staging.HasArtifact && sha256.Sum256(staging.Artifact.Bytes) != expected.ArtifactDigest {
			return fmt.Errorf("%w: temporary artifact mismatch", errPlanInvalid)
		}
		if staging.HasManifest && (sha256.Sum256(staging.Manifest.Bytes) != expected.ManifestCommitment ||
			staging.DecodedManifest.Artifact != expected.ArtifactDigest) {
			return fmt.Errorf("%w: temporary manifest mismatch", errPlanInvalid)
		}
	}
	return nil
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
	if len(facts.Transactions) == 1 {
		transaction := facts.Transactions[0]
		if transaction.Generation == 0 || transaction.Generation != selection.Current.Generation+1 && transaction.Generation != selection.Transaction {
			return fmt.Errorf("%w: transaction generation mismatch", errPlanInvalid)
		}
	}
	if len(facts.StagingDirs) > 1 || len(facts.CurrentTemps) > 1 {
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

func verifyGenerationMatchesTuple(facts inventoryResult, tuple inspectedTuple) error {
	generation := generationByID(facts.Generations, tuple.Generation)
	if generation == nil || uint64(len(generation.Artifact.Bytes)) != tuple.Length ||
		sha256.Sum256(generation.Artifact.Bytes) != tuple.Artifact || sha256.Sum256(generation.Manifest.Bytes) != tuple.Manifest {
		return fmt.Errorf("%w: selected generation %d mismatch", errPlanInvalid, tuple.Generation)
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

func planIdle(facts inventoryResult, custodyNotice string) recoveryPlan {
	plan := recoveryPlan{Row: "R00", Outcome: "recovered", State: "idle", Generation: 0, CurrentDigest: digestOfGeneration(facts, 0), SafeNotice: "update interrupted", CustodyNotice: custodyNotice}
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

func planClassify(facts inventoryResult, validation journalValidation, records recoveryRecords, transaction uint64, custodyNotice string) (recoveryPlan, error) {
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
	if last.AdapterResult == adapterFailed {
		if hasGenerations || hasTemporaryStaging || !hasStaging {
			return recoveryPlan{}, fmt.Errorf("%w: failed adapter physical state is ambiguous", errPlanInvalid)
		}
		return planFailedAdapterAbort(facts, transaction, last.State, predecessorDigest, custodyNotice)
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
	if hasGenerations {
		if lastState < byte(stateDraining) {
			return recoveryPlan{}, fmt.Errorf("%w: candidate published before draining", errPlanInvalid)
		}
		if lastState >= byte(stateCommitted) {
			return buildCommittedPlan(facts, transaction, custodyNotice)
		}
		if lastState >= byte(stateActivated) {
			return planR12R13(facts, transaction, lastState, custodyNotice)
		}
		return planR8ToR11(facts, records, transaction, firstCanonicalTemp(facts.CurrentTemps), predecessorDigest, custodyNotice)
	}
	return planEarlyNonterminal(facts, transaction, lastState, hasStaging, hasTemporaryStaging, predecessorDigest, custodyNotice)
}

func planEarlyNonterminal(facts inventoryResult, transaction uint64, lastState byte, hasStaging, hasTemporaryStaging bool, predecessorDigest [32]byte, custodyNotice string) (recoveryPlan, error) {
	state, ok := stateName(lastState)
	if !ok {
		return recoveryPlan{}, fmt.Errorf("%w: invalid state %d", errPlanInvalid, lastState)
	}
	if state == "release-accepted" && !hasStaging && !hasTemporaryStaging {
		return buildRecoveredPlan("R01", state, transaction, predecessorDigest, custodyNotice, false), nil
	}
	if state == "artifact-verified" {
		if hasTemporaryStaging {
			plan := buildRecoveredPlan("R03", state, transaction, predecessorDigest, custodyNotice, false)
			plan.Operations = stagingRemovalOperations(stagingFacts(facts.StagingDirs, transaction, true))
			return plan, nil
		}
		if hasStaging {
			plan := buildRecoveredPlan("R03", state, transaction, predecessorDigest, custodyNotice, false)
			plan.Operations = stagingRemovalOperations(stagingFacts(facts.StagingDirs, transaction, false))
			return plan, nil
		}
		return buildRecoveredPlan("R02", state, transaction, predecessorDigest, custodyNotice, false), nil
	}
	if !hasStaging {
		return recoveryPlan{}, fmt.Errorf("%w: missing staging for %s", errPlanInvalid, state)
	}
	return buildRecoveredPlan(rowForNonterminal(state), state, transaction, predecessorDigest, custodyNotice, true), nil
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

func planR8ToR11(facts inventoryResult, records recoveryRecords, transaction uint64, currentTemp rawFile, predecessorDigest [32]byte, custodyNotice string) (recoveryPlan, error) {
	generation := strconv.FormatUint(transaction, 10)
	plan := buildRecoveredPlan("R08", "draining", transaction, predecessorDigest, custodyNotice, true)
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
		predecessorCustody, custodyErr := custodyNoticeForTuple(facts, *selection.Rollback)
		if custodyErr != nil {
			return recoveryPlan{}, custodyErr
		}
		plan.CustodyNotice = predecessorCustody
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

// custodyNoticeForTuple returns the notice bound to one already-admitted
// payload tuple. The planner uses it when recovery changes the selected
// current generation, so Result custody always describes the normalized
// selection rather than the selection observed at entry.
func custodyNoticeForTuple(facts inventoryResult, tuple inspectedTuple) (string, error) {
	for _, generation := range facts.Generations {
		if generation.Generation == tuple.Generation && tupleMatchesGeneration(tuple, generation) {
			return generation.DecodedManifest.CustodyNotice, nil
		}
	}
	return "", fmt.Errorf("%w: custody manifest missing", errPlanInvalid)
}

func planR12R13(facts inventoryResult, transaction uint64, lastState byte, custodyNotice string) (recoveryPlan, error) {
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
		StagingPresent: false, SafeNotice: "update interrupted", CustodyNotice: custodyNotice,
	}, nil
}

func buildCommittedPlan(facts inventoryResult, transaction uint64, custodyNotice string) (recoveryPlan, error) {
	successor, err := decodeCurrent(facts.Current.Bytes)
	if err != nil {
		return recoveryPlan{}, err
	}
	return recoveryPlan{
		Row: "R14", Outcome: "committed", State: "committed", Generation: transaction,
		CurrentDigest: successor.Current.Artifact, RollbackDigest: predecessorRollbackDigest(successor),
		StagingPresent: false, SafeNotice: "update committed", CustodyNotice: custodyNotice,
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

func buildRecoveredPlan(row, state string, generation uint64, predecessorDigest [32]byte, custodyNotice string, staging bool) recoveryPlan {
	return recoveryPlan{Row: row, Outcome: "recovered", State: state, Generation: generation, CurrentDigest: predecessorDigest, RollbackDigest: [32]byte{}, StagingPresent: staging, SafeNotice: "update interrupted", CustodyNotice: custodyNotice}
}
