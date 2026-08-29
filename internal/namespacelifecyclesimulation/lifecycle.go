package namespacelifecyclesimulation

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/epoch"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
)

const (
	reportSchema    = "ardents-h4-4b-lifecycle-simulation-v1"
	contractVersion = "h4-4b-project-control-lifecycle-v1"
)

// Cell binds one lifecycle matrix case to its exact observed outcome.
type Cell struct {
	Case    string `json:"case"`
	Outcome string `json:"outcome"`
}

// Report records one bounded H4-4B project-control simulation.
type Report struct {
	Schema                 string   `json:"schema"`
	Contract               string   `json:"contract"`
	SimulationResult       string   `json:"simulation_result"`
	DeclaredSourceRevision string   `json:"declared_source_revision"`
	ReceiptDigest          string   `json:"receipt_digest"`
	Simulation             bool     `json:"simulation"`
	Qualified              bool     `json:"qualified"`
	Passed                 []Cell   `json:"passed"`
	Rejected               []string `json:"rejected"`
	Limitation             string   `json:"limitation"`
}

// RunWithSourceRevision materializes the complete H4-4B lifecycle through a
// durable pending journal and threshold-attested Epoch state.
func RunWithSourceRevision(sourceRevision string) (Report, error) {
	if sourceRevision == "" {
		return Report{}, errors.New("namespace lifecycle simulation source revision is required")
	}
	root, err := os.MkdirTemp("", "ardents-h4-4b-lifecycle-")
	if err != nil {
		return Report{}, err
	}
	defer os.RemoveAll(root)

	network := [32]byte{4}
	policy, attester := simulationPolicy(network)
	store, err := epoch.Open(root, policy)
	if err != nil {
		return Report{}, err
	}
	defer func() { _ = store.Close() }()
	base := time.Unix(2_000_000_000, 0).UTC()
	leasePolicy := record.Policy{DefaultLeaseDuration: time.Hour, DefaultGraceDuration: time.Hour}
	authority := simulationKey("name-authority")
	authorityID := hex.EncodeToString(authority.Public().(ed25519.PublicKey))

	install := func(value record.Record, signer ed25519.PrivateKey, number uint64) (epoch.Epoch, error) {
		signed, signErr := record.SignRecord(network, value, signer)
		if signErr != nil {
			return epoch.Epoch{}, signErr
		}
		if appendErr := store.AppendPending(value.Name, []byte("h4-4b-lifecycle"), signed, base.UnixMilli()+int64(number)); appendErr != nil {
			return epoch.Epoch{}, appendErr
		}
		pending, pendingErr := store.Pending()
		if pendingErr != nil || len(pending) == 0 {
			return epoch.Epoch{}, errors.New("namespace lifecycle pending evidence is unavailable")
		}
		valueEpoch := simulatedEpoch(number)
		candidate, beginErr := store.BeginEpochInstallation(valueEpoch, base.Add(time.Duration(number)*time.Minute), leasePolicy)
		if beginErr != nil {
			return epoch.Epoch{}, beginErr
		}
		if includeErr := candidate.IncludePendingThrough(pending[len(pending)-1].Sequence); includeErr != nil {
			return epoch.Epoch{}, includeErr
		}
		if commitErr := candidate.Commit(attester); commitErr != nil {
			return epoch.Epoch{}, commitErr
		}
		return valueEpoch, nil
	}

	claim, err := record.ApplyAtLegacy(nil, base, record.Op{Kind: "claim", Name: "alice", Generation: 1,
		ExpectedGeneration: 0, ExpectedRevision: 0, Authority: authorityID}, leasePolicy)
	if err != nil {
		return Report{}, err
	}
	if _, err = install(claim, authority, 1); err != nil {
		return Report{}, err
	}
	published, err := publish(claim, authorityID, [32]byte{1}, base)
	if err != nil {
		return Report{}, err
	}
	publishEpoch, err := install(published, authority, 2)
	if err != nil || !bindingCurrent(store, policy, publishEpoch, base, [32]byte{1}) {
		return Report{}, errors.New("namespace publication did not become threshold current")
	}
	updated, err := publish(published, authorityID, [32]byte{2}, base.Add(time.Minute))
	if err != nil {
		return Report{}, err
	}
	updateEpoch, err := install(updated, authority, 3)
	if err != nil || !bindingCurrent(store, policy, updateEpoch, base, [32]byte{2}) {
		return Report{}, errors.New("namespace update did not become threshold current")
	}
	proof, err := store.Lookup("alice", updateEpoch.Number)
	if err != nil {
		return Report{}, err
	}
	_, warning, _, graceErr := epoch.VerifyBinding(policy, proof, updateEpoch.Number, updateEpoch.Digest, updated.LeaseExpiresAt*1_000+1)
	if graceErr != nil || warning == "" {
		return Report{}, errors.New("namespace expiry did not expose grace")
	}
	released, err := record.ApplyAtLegacy(&updated, time.Unix(updated.GraceExpiresAt+1, 0).UTC(), record.Op{Kind: "advance", Name: "alice",
		ExpectedGeneration: updated.Generation, ExpectedRevision: updated.Revision}, leasePolicy)
	if err != nil {
		return Report{}, err
	}
	releaseEpoch, err := install(released, authority, 5)
	if err != nil {
		return Report{}, err
	}
	if proof, err = store.Lookup("alice", releaseEpoch.Number); err != nil {
		return Report{}, err
	} else if _, _, _, err = epoch.VerifyBinding(policy, proof, releaseEpoch.Number, releaseEpoch.Digest, updated.GraceExpiresAt*1_000+1); err == nil {
		return Report{}, errors.New("released Name remained available")
	}
	if _, err = record.ApplyAtLegacy(&released, base, record.Op{Kind: "claim", Name: "alice", Generation: 1,
		ExpectedGeneration: released.Generation, ExpectedRevision: released.Revision, Authority: authorityID}, leasePolicy); err == nil {
		return Report{}, errors.New("old generation reclaim was accepted")
	}
	reclaimed, err := record.ApplyAtLegacy(&released, base, record.Op{Kind: "claim", Name: "alice", Generation: 2,
		ExpectedGeneration: released.Generation, ExpectedRevision: released.Revision, Authority: authorityID}, leasePolicy)
	if err != nil {
		return Report{}, err
	}
	_, err = install(reclaimed, authority, 6)
	if err != nil {
		return Report{}, err
	}
	if records, _, currentErr := store.CurrentRecords(); currentErr != nil || records["alice"].Generation != 2 {
		return Report{}, errors.New("reclaim did not become threshold current")
	}
	stale, err := store.BeginEpochInstallation(simulatedEpoch(7), base.Add(7*time.Minute), leasePolicy)
	if err != nil {
		return Report{}, err
	}
	refreshed, err := record.ApplyAtLegacy(&reclaimed, base.Add(8*time.Minute), record.Op{Kind: "renew", Name: "alice",
		Authority: authorityID, ExpectedGeneration: reclaimed.Generation, ExpectedRevision: reclaimed.Revision}, leasePolicy)
	if err != nil {
		return Report{}, err
	}
	if _, err = install(refreshed, authority, 8); err != nil {
		return Report{}, err
	}
	if err = stale.Commit(attester); err == nil {
		return Report{}, errors.New("stale Epoch replay was accepted")
	}
	if !forkedSuccessorsAreRejected(network, policy, attester, authority, authorityID, base, leasePolicy) {
		return Report{}, errors.New("forked pending successors were accepted")
	}
	if err = store.Close(); err != nil {
		return Report{}, err
	}
	store, err = epoch.Open(root, policy)
	if err != nil {
		return Report{}, err
	}
	if records, _, currentErr := store.CurrentRecords(); currentErr != nil || records["alice"].Generation != 2 {
		return Report{}, errors.New("restart lost threshold current state")
	}
	conflicted, err := record.ApplyAtLegacy(&refreshed, base.Add(9*time.Minute), record.Op{Kind: "conflict", Name: "alice",
		ExpectedGeneration: refreshed.Generation, ExpectedRevision: refreshed.Revision, ConflictContext: "h4-4b-fork"}, leasePolicy)
	if err != nil {
		return Report{}, err
	}
	conflictEpoch, err := install(conflicted, authority, 9)
	if err != nil {
		return Report{}, err
	}
	if proof, err = store.Lookup("alice", conflictEpoch.Number); err != nil {
		return Report{}, err
	} else if _, _, _, err = epoch.VerifyBinding(policy, proof, conflictEpoch.Number, conflictEpoch.Digest, base.UnixMilli()); err == nil {
		return Report{}, errors.New("conflicting Namespace state resolved")
	}

	report := Report{Schema: reportSchema, Contract: contractVersion, SimulationResult: "passed", DeclaredSourceRevision: sourceRevision,
		Simulation: true, Qualified: false, Limitation: "project-controlled simulation; no public Namespace, Endpoint authority, independent operation, or Public Beta qualification",
		Passed: []Cell{{"publication-current", "threshold-current"}, {"update-current", "threshold-current"}, {"expiry-grace", "grace-warning"},
			{"released-unavailable", "unavailable"}, {"reclaim-next-generation", "threshold-current"}, {"restart-preserves-current", "threshold-current"}},
		Rejected: []string{"stale-replay", "forked-successor", "conflicting-current-state", "old-generation-reclaim"}}
	report.ReceiptDigest = reportDigest(report)
	return report, nil
}

func publish(current record.Record, authority string, target [32]byte, at time.Time) (record.Record, error) {
	return record.ApplyAtLegacy(&current, at, record.Op{Kind: "publish", Name: current.Name, Authority: authority,
		ExpectedGeneration: current.Generation, ExpectedRevision: current.Revision, Target: target, RecordNotAfter: current.GraceExpiresAt * 1_000}, record.Policy{DefaultLeaseDuration: time.Hour, DefaultGraceDuration: time.Hour})
}

func forkedSuccessorsAreRejected(network [32]byte, policy epoch.MaterializationPolicy,
	attester func([]byte) ([][32]byte, [][]byte, error), authority ed25519.PrivateKey, authorityID string,
	base time.Time, leasePolicy record.Policy,
) bool {
	root, err := os.MkdirTemp("", "ardents-h4-4b-fork-")
	if err != nil {
		return false
	}
	defer os.RemoveAll(root)
	store, err := epoch.Open(root, policy)
	if err != nil {
		return false
	}
	defer store.Close()
	claim, err := record.ApplyAtLegacy(nil, base, record.Op{Kind: "claim", Name: "fork", Generation: 1,
		Authority: authorityID}, leasePolicy)
	if err != nil {
		return false
	}
	signed, err := record.SignRecord(network, claim, authority)
	if err != nil || store.AppendPending("fork", []byte("fork-claim"), signed, base.UnixMilli()) != nil {
		return false
	}
	candidate, err := store.BeginEpochInstallation(simulatedEpoch(20), base, leasePolicy)
	if err != nil || candidate.IncludePendingThrough(1) != nil || candidate.Commit(attester) != nil {
		return false
	}
	first, err := publish(claim, authorityID, [32]byte{3}, base.Add(time.Minute))
	if err != nil {
		return false
	}
	second, err := publish(claim, authorityID, [32]byte{4}, base.Add(time.Minute))
	if err != nil {
		return false
	}
	for _, successor := range []record.Record{first, second} {
		signed, err = record.SignRecord(network, successor, authority)
		if err != nil || store.AppendPending("fork", []byte("fork-branch"), signed, base.UnixMilli()+1) != nil {
			return false
		}
	}
	pending, err := store.Pending()
	if err != nil {
		return false
	}
	candidate, err = store.BeginEpochInstallation(simulatedEpoch(21), base.Add(time.Minute), leasePolicy)
	return err == nil && candidate.IncludePendingThrough(pending[len(pending)-1].Sequence) != nil
}

func bindingCurrent(store *epoch.Store, policy epoch.MaterializationPolicy, value epoch.Epoch, at time.Time, target [32]byte) bool {
	proof, err := store.Lookup("alice", value.Number)
	if err != nil {
		return false
	}
	binding, _, _, err := epoch.VerifyBinding(policy, proof, value.Number, value.Digest, at.UnixMilli())
	return err == nil && binding.Target == target
}

func simulationPolicy(network [32]byte) (epoch.MaterializationPolicy, func([]byte) ([][32]byte, [][]byte, error)) {
	keys := []ed25519.PrivateKey{simulationKey("attester-1"), simulationKey("attester-2"), simulationKey("attester-3")}
	policy := epoch.MaterializationPolicy{Network: network, Rule: "ardents-namespace-materialization-v1", Threshold: 2,
		Authorities: make(map[[32]byte]ed25519.PublicKey, len(keys))}
	for _, key := range keys {
		public := key.Public().(ed25519.PublicKey)
		policy.Authorities[sha256.Sum256(public)] = public
	}
	return policy, func(transcript []byte) ([][32]byte, [][]byte, error) {
		ids := make([][32]byte, 2)
		signatures := make([][]byte, 2)
		for index, key := range keys[:2] {
			ids[index] = sha256.Sum256(key.Public().(ed25519.PublicKey))
			signatures[index] = ed25519.Sign(key, transcript)
		}
		if bytes.Compare(ids[0][:], ids[1][:]) > 0 {
			ids[0], ids[1], signatures[0], signatures[1] = ids[1], ids[0], signatures[1], signatures[0]
		}
		return ids, signatures, nil
	}
}

func simulationKey(label string) ed25519.PrivateKey {
	seed := sha256.Sum256([]byte(label))
	return ed25519.NewKeyFromSeed(seed[:])
}

func simulatedEpoch(number uint64) epoch.Epoch {
	digest := sha256.Sum256([]byte("h4-4b-epoch-" + string(rune(number))))
	transition := sha256.Sum256([]byte("h4-4b-transition-" + string(rune(number))))
	rejection := sha256.Sum256([]byte("h4-4b-rejection-" + string(rune(number))))
	return epoch.Epoch{Number: number, Digest: digest, CutoffOffset: int64(number), TransitionRoot: transition, TransitionLength: 1,
		RejectionRoot: rejection, RejectionLength: 1}
}

func reportDigest(report Report) string {
	passed := make([]string, 0, len(report.Passed))
	for _, value := range report.Passed {
		passed = append(passed, value.Case+":"+value.Outcome)
	}
	sort.Strings(passed)
	raw := strings.Join([]string{report.Schema, report.Contract, report.SimulationResult, report.DeclaredSourceRevision,
		strings.Join(passed, ","), strings.Join(report.Rejected, ","), report.Limitation}, "\n")
	digest := sha256.Sum256([]byte(raw))
	return "sha256:" + hex.EncodeToString(digest[:])
}
