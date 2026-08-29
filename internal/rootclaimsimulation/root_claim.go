package rootclaimsimulation

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

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/admission"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/claim"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/epoch"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
)

const (
	reportSchema    = "ardents-h4-4c-root-claim-simulation-v1"
	contractVersion = "h4-4c-project-control-root-claims-v1"
)

// Cell binds one root-claim case to its exact observed outcome.
type Cell struct {
	Case    string `json:"case"`
	Outcome string `json:"outcome"`
}

// Report records one bounded H4-4C project-control simulation.
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

type recordSigner func(record.RecordSigningRequest) ([]byte, error)

func (sign recordSigner) Sign(request record.RecordSigningRequest) ([]byte, error) {
	return sign(request)
}

// RunWithSourceRevision admits committed claims, verifies their authenticated
// close, and makes only the deterministic winner threshold-current.
func RunWithSourceRevision(sourceRevision string) (Report, error) {
	if sourceRevision == "" {
		return Report{}, errors.New("root claim simulation source revision is required")
	}
	const epochNumber = uint64(9)
	network := [32]byte{44}
	base := time.Unix(2_000_000_100, 0).UTC()
	policy, materialize := simulationPolicy(network)
	gate, err := admission.NewAdmission([32]byte{1}, network, epochNumber-1, [32]byte{2})
	if err != nil {
		return Report{}, err
	}
	first, firstInput, firstKey, err := admittedClaim(gate, network, epochNumber, "winner", "claimant-first", [32]byte{3}, 0)
	if err != nil {
		return Report{}, err
	}
	second, _, _, err := admittedClaim(gate, network, epochNumber, "winner", "claimant-second", [32]byte{4}, 1)
	if err != nil {
		return Report{}, err
	}
	order, proof, err := authenticatedClose(network, epochNumber, []claim.Claim{first, second})
	if err != nil {
		return Report{}, err
	}
	winner, err := firstInput.VerifyClose(order, 0, proof)
	if err != nil || winner.Name() != "winner" {
		return Report{}, errors.New("deterministic root claim winner is unavailable")
	}
	if !withholdingIsRejected(order, proof) || !incompleteEvidenceIsRejected(order, proof) ||
		!ruleForkIsRejected(order, proof) || !controlForkIsRejected(order, proof) {
		return Report{}, errors.New("root claim fail-closed case was accepted")
	}

	root, err := os.MkdirTemp("", "ardents-h4-4c-root-claim-")
	if err != nil {
		return Report{}, err
	}
	defer os.RemoveAll(root)
	store, err := epoch.Open(root, policy)
	if err != nil {
		return Report{}, err
	}
	defer store.Close()
	currentEpoch := epoch.Epoch{Number: epochNumber, Digest: winner.CloseDigest(), CutoffOffset: 1,
		TransitionRoot: sha256.Sum256([]byte("h4-4c-transitions")), TransitionLength: 0,
		RejectionRoot: proof.RejectionRoot, RejectionLength: proof.RejectionLength}
	leasePolicy := record.Policy{DefaultLeaseDuration: time.Hour, DefaultGraceDuration: time.Hour}
	installation, err := store.BeginEpochInstallation(currentEpoch, base, leasePolicy)
	if err != nil {
		return Report{}, err
	}
	if err := installation.MaterializeClaim(winner, recordSigner(func(request record.RecordSigningRequest) ([]byte, error) {
		return ed25519.Sign(firstKey, request.Transcript()), nil
	})); err != nil {
		return Report{}, err
	}
	if err := installation.Commit(materialize); err != nil {
		return Report{}, err
	}
	current, _, err := store.CurrentRecords()
	if err != nil || len(current) != 1 || current["winner"].Generation != 1 ||
		current["winner"].Authority != hex.EncodeToString(firstKey.Public().(ed25519.PublicKey)) ||
		current["winner"].LeaseExpiresAt != base.Add(time.Hour).Unix() ||
		current["winner"].GraceExpiresAt != base.Add(2*time.Hour).Unix() {
		return Report{}, errors.New("root claim was not materialized as threshold current")
	}
	if _, err := store.Lookup("winner", epochNumber); err != nil {
		return Report{}, errors.New("threshold current root claim has no Namespace proof")
	}

	report := Report{Schema: reportSchema, Contract: contractVersion, SimulationResult: "passed", DeclaredSourceRevision: sourceRevision,
		Simulation: true, Qualified: false,
		Limitation: "project-controlled simulation; no public Namespace, governance legitimacy, Sybil resistance, anti-squatting, Endpoint authority, independent operation, or Public Beta qualification",
		Passed: []Cell{{"commit-reveal-lowest-ordinal", "winner-materialized"}, {"authenticated-epoch-close", "threshold-accepted"},
			{"current-namespace-materialization", "threshold-current"}, {"bounded-lease", "active-then-grace"}},
		Rejected: []string{"withheld-reveal", "incomplete-evidence", "rule-fork", "control-fork"}}
	report.ReceiptDigest = reportDigest(report)
	return report, nil
}

func admittedClaim(gate *admission.Admission, network [32]byte, epochNumber uint64, name, label string,
	secret [32]byte, ordinal uint32,
) (claim.Claim, claim.EpochClaimInput, ed25519.PrivateKey, error) {
	key := simulationKey(label)
	value := claim.Claim{Name: name, Secret: secret}
	copy(value.Authority[:], key.Public().(ed25519.PublicKey))
	commitment := claim.CommitmentFor(network, epochNumber, value)
	challenge, err := gate.Issue(100, "root-claim", commitment, sha256.Sum256([]byte(label)), 1_000, [16]byte{byte(ordinal + 1)})
	if err != nil {
		return claim.Claim{}, claim.EpochClaimInput{}, nil, err
	}
	admitted, err := claim.AdmitClaimCommitment(gate, 100, commitment, mustSolve(challenge))
	if err != nil {
		return claim.Claim{}, claim.EpochClaimInput{}, nil, err
	}
	input, err := admitted.EpochInput()
	if err != nil {
		return claim.Claim{}, claim.EpochClaimInput{}, nil, err
	}
	copy(value.Signature[:], ed25519.Sign(key, claim.RevealTranscript(network, epochNumber, claim.Claim{Commitment: commitment})))
	revealed, err := admitted.Reveal(name, secret, value.Authority, value.Signature)
	if err != nil {
		return claim.Claim{}, claim.EpochClaimInput{}, nil, err
	}
	revealed.Ordinal = ordinal
	copy(revealed.Signature[:], ed25519.Sign(key, claim.RevealTranscript(network, epochNumber, revealed)))
	return revealed, input, key, nil
}

func mustSolve(challenge admission.Challenge) admission.Proof {
	proof, _ := challenge.Solve()
	return proof
}

func authenticatedClose(network [32]byte, epochNumber uint64, claims []claim.Claim) (claim.ClaimOrder, claim.ClaimProof, error) {
	proof, err := claim.NewClaimProof(network, epochNumber, 1, claims)
	if err != nil {
		return claim.ClaimOrder{}, claim.ClaimProof{}, err
	}
	keys := []ed25519.PrivateKey{simulationKey("close-a"), simulationKey("close-b"), simulationKey("close-c")}
	order := claim.ClaimOrder{Network: network, Rule: proof.Rule, MinimumEpoch: epochNumber, MaximumClaims: 32, Threshold: 2,
		Authorities: make(map[[32]byte]ed25519.PublicKey, len(keys))}
	for _, key := range keys {
		order.Authorities[sha256.Sum256(key.Public().(ed25519.PublicKey))] = key.Public().(ed25519.PublicKey)
	}
	signClose(&proof, keys[:2])
	return order, proof, nil
}

func withholdingIsRejected(order claim.ClaimOrder, proof claim.ClaimProof) bool {
	proof.Claims = nil
	return rejectedAs(order, proof, "unavailable")
}
func incompleteEvidenceIsRejected(order claim.ClaimOrder, proof claim.ClaimProof) bool {
	proof.RejectionLength, proof.RejectionRoot = 0, sha256.Sum256([]byte{2})
	signClose(&proof, closeKeys())
	return rejectedAs(order, proof, "unavailable")
}
func ruleForkIsRejected(order claim.ClaimOrder, proof claim.ClaimProof) bool {
	proof.Rule = "ardents-name-claim-order-v2"
	signClose(&proof, closeKeys())
	return rejectedAs(order, proof, "fork")
}
func controlForkIsRejected(order claim.ClaimOrder, proof claim.ClaimProof) bool {
	alternate := proof
	alternate.Claims, alternate.SignerIDs, alternate.Signatures, alternate.AlternateSets = nil, nil, nil, nil
	alternate.RejectionRoot = sha256.Sum256([]byte("different control close"))
	alternate.RejectionLength = 2
	signClose(&alternate, closeKeys())
	proof.AlternateSets = []claim.ClaimProof{alternate}
	return rejectedAs(order, proof, "fork")
}

func rejectedAs(order claim.ClaimOrder, proof claim.ClaimProof, expected string) bool {
	result, err := order.Verify(proof)
	return result.Outcome == expected && (err != nil || expected == "fork")
}

func closeKeys() []ed25519.PrivateKey {
	return []ed25519.PrivateKey{simulationKey("close-a"), simulationKey("close-b")}
}
func signClose(proof *claim.ClaimProof, keys []ed25519.PrivateKey) {
	proof.SignerIDs, proof.Signatures = nil, nil
	type signed struct {
		id        [32]byte
		signature []byte
	}
	values := make([]signed, 0, len(keys))
	for _, key := range keys {
		id := sha256.Sum256(key.Public().(ed25519.PublicKey))
		values = append(values, signed{id, ed25519.Sign(key, claim.StatementTranscript(*proof))})
	}
	sort.Slice(values, func(i, j int) bool { return bytes.Compare(values[i].id[:], values[j].id[:]) < 0 })
	for _, value := range values {
		proof.SignerIDs, proof.Signatures = append(proof.SignerIDs, value.id), append(proof.Signatures, value.signature)
	}
}

func simulationPolicy(network [32]byte) (epoch.MaterializationPolicy, func([]byte) ([][32]byte, [][]byte, error)) {
	keys := []ed25519.PrivateKey{simulationKey("materialize-a"), simulationKey("materialize-b"), simulationKey("materialize-c")}
	policy := epoch.MaterializationPolicy{Network: network, Rule: "ardents-namespace-materialization-v1", Threshold: 2, Authorities: make(map[[32]byte]ed25519.PublicKey, len(keys))}
	for _, key := range keys {
		policy.Authorities[sha256.Sum256(key.Public().(ed25519.PublicKey))] = key.Public().(ed25519.PublicKey)
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
	seed := sha256.Sum256([]byte("h4-4c:" + label))
	return ed25519.NewKeyFromSeed(seed[:])
}
func reportDigest(report Report) string {
	passed := make([]string, 0, len(report.Passed))
	for _, value := range report.Passed {
		passed = append(passed, value.Case+":"+value.Outcome)
	}
	sort.Strings(passed)
	raw := strings.Join([]string{report.Schema, report.Contract, report.SimulationResult, report.DeclaredSourceRevision, strings.Join(passed, ","), strings.Join(report.Rejected, ","), report.Limitation}, "\n")
	digest := sha256.Sum256([]byte(raw))
	return "sha256:" + hex.EncodeToString(digest[:])
}
