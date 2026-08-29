package publiccontrolsimulation

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	reportSchema    = "ardents-h4-6c-simulation-v1"
	contractVersion = "h4-6c-project-control-simulation-v1"
)

// Report records one project-controlled mechanics simulation. Qualified is
// always false because simulated identities cannot prove real independence.
type Report struct {
	Schema                 string   `json:"schema"`
	Contract               string   `json:"contract"`
	SimulationResult       string   `json:"simulation_result"`
	DeclaredSourceRevision string   `json:"declared_source_revision"`
	ReceiptDigest          string   `json:"receipt_digest"`
	Simulation             bool     `json:"simulation"`
	Qualified              bool     `json:"qualified"`
	Passed                 []string `json:"passed"`
	Rejected               []string `json:"rejected"`
	Limitation             string   `json:"limitation"`
}

type signer struct {
	public  ed25519.PublicKey
	private ed25519.PrivateKey
}

type signature struct {
	public ed25519.PublicKey
	value  []byte
}

type emergencyAction struct {
	Kind    string
	Scope   string
	Expires time.Time
}

type rotation struct {
	Reason      string
	Predecessor string
	Successor   string
	Expires     time.Time
}

type candidateView struct {
	InputDigest string
	Rules       string
	Cutoff      int
	Root        string
	Length      int
	Summary     string
	Proof       string
}

type auditResult struct {
	Auditor      string
	ToolRevision string
	View         candidateView
	Signature    []byte
}

type buildEvidence struct {
	Source        string
	Dependencies  string
	Recipe        string
	SBOM          string
	Qualification string
	Artifact      string
	Attestations  []buildAttestation
}

type buildAttestation struct {
	Builder   int
	Signature []byte
}

// Run exercises a complete local mechanics profile: threshold custody,
// constrained emergency, lifecycle rotation, full deterministic Candidate View
// reconstruction, reproducible build attestation, and reader failure matrix.
func Run() (Report, error) {
	return RunWithSourceRevision("unrecorded")
}

// RunWithSourceRevision emits a self-contained receipt for one exact source
// revision. The caller owns recording the JSON outside the repository.
func RunWithSourceRevision(sourceRevision string) (Report, error) {
	if sourceRevision == "" {
		return Report{}, errors.New("simulation source revision is required")
	}
	custodians, err := newSigners(5)
	if err != nil {
		return Report{}, err
	}
	successors, err := newSigners(5)
	if err != nil {
		return Report{}, err
	}
	builders, err := newSigners(2)
	if err != nil {
		return Report{}, err
	}
	auditors, err := newSigners(2)
	if err != nil {
		return Report{}, err
	}
	report := Report{Schema: reportSchema, Contract: contractVersion, DeclaredSourceRevision: sourceRevision, Simulation: true,
		Limitation: "project-controlled simulation; not independent custody, audit, build evidence, or Public Beta qualification"}
	if err := exerciseCustody(&report, custodians, successors); err != nil {
		return Report{}, err
	}
	if err := exerciseCandidateView(&report, auditors); err != nil {
		return Report{}, err
	}
	if err := exerciseBuild(&report, builders); err != nil {
		return Report{}, err
	}
	passed, rejected, err := exerciseReaderMatrix()
	if err != nil {
		return Report{}, err
	}
	report.Passed = append(report.Passed, passed...)
	report.Rejected = append(report.Rejected, rejected...)
	report.SimulationResult = "passed"
	report.ReceiptDigest = digest([]byte(strings.Join([]string{report.Schema, report.Contract, report.SimulationResult, report.DeclaredSourceRevision,
		strings.Join(report.Passed, ","), strings.Join(report.Rejected, ","), report.Limitation}, "\n")))
	return report, nil
}

func exerciseCustody(report *Report, custodians, successors []signer) error {
	routine := []byte("h4-6c-simulation/routine/candidate-2")
	if !verifyQuorum(routine, custodians, sign(routine, custodians[:3]), 3) {
		return errors.New("simulation routine quorum did not verify")
	}
	report.Passed = append(report.Passed, "routine-3-of-5")
	if verifyQuorum(routine, custodians, sign(routine, custodians[:2]), 3) {
		return errors.New("simulation accepted an under-threshold routine action")
	}
	report.Rejected = append(report.Rejected, "routine-under-threshold")

	at := time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)
	emergency := emergencyAction{Kind: "emergency-stop", Scope: "disable-only", Expires: at.Add(10 * time.Minute)}
	message := []byte(canonicalEmergency(emergency))
	if !validEmergency(emergency, at) || !verifyQuorum(message, custodians, sign(message, custodians[:4]), 4) {
		return errors.New("simulation emergency did not verify")
	}
	report.Passed = append(report.Passed, "emergency-4-of-5-disable-only")
	if validEmergency(emergency, at) && verifyQuorum(message, custodians, sign(message, custodians[:3]), 4) {
		return errors.New("simulation accepted an under-threshold emergency")
	}
	report.Rejected = append(report.Rejected, "emergency-under-threshold")
	if validEmergency(emergencyAction{Kind: "emergency-stop", Scope: "install-code", Expires: emergency.Expires}, at) || validEmergency(emergency, emergency.Expires) {
		return errors.New("simulation accepted an escalated or expired emergency")
	}
	report.Rejected = append(report.Rejected, "emergency-escalation", "emergency-expired")

	predecessor, successor := digest([]byte("candidate-1")), digest([]byte("candidate-2"))
	for _, reason := range []string{"loss", "compromise", "removal", "replacement", "emergency-recovery"} {
		value := rotation{Reason: reason, Predecessor: predecessor, Successor: successor, Expires: at.Add(time.Hour)}
		rotationMessage := []byte(canonicalRotation(value))
		if !validRotation(value, predecessor, at) || !verifyQuorum(rotationMessage, custodians, sign(rotationMessage, custodians[:3]), 3) ||
			!verifyQuorum(rotationMessage, successors, sign(rotationMessage, successors[:3]), 3) {
			return fmt.Errorf("simulation %s rotation did not verify", reason)
		}
	}
	report.Passed = append(report.Passed, "bidirectional-rotation-lifecycle")
	invalid := rotation{Reason: "replacement", Predecessor: digest([]byte("wrong")), Successor: successor, Expires: at.Add(time.Hour)}
	if validRotation(invalid, predecessor, at) {
		return errors.New("simulation accepted a rotation with wrong predecessor")
	}
	report.Rejected = append(report.Rejected, "rotation-predecessor-mismatch")
	return nil
}

func exerciseCandidateView(report *Report, auditors []signer) error {
	input := []byte("candidate-a\ncandidate-b\ncandidate-c\ncutoff=3\nrules=candidate-view-v1\nmaterialization=materialization-v1\n")
	first, err := fullAudit("auditor-1", input, auditors[0])
	if err != nil {
		return err
	}
	second, err := fullAudit("auditor-2", input, auditors[1])
	if err != nil {
		return err
	}
	if first.Auditor == second.Auditor || first.ToolRevision != second.ToolRevision || first.View != second.View || first.View.Length != 2 {
		return errors.New("simulation full Candidate View audits disagreed")
	}
	if !verifyAudit(first, auditors[0]) || !verifyAudit(second, auditors[1]) {
		return errors.New("simulation full Candidate View audit signatures did not verify")
	}
	report.Passed = append(report.Passed, "two-full-candidate-view-audits")
	disagreed, err := fullAudit("auditor-2", []byte("candidate-a\ncandidate-c\ncutoff=2\nrules=candidate-view-v1\nmaterialization=materialization-v1\n"), auditors[1])
	if err == nil && disagreed.View == first.View {
		return errors.New("simulation accepted Candidate View disagreement")
	}
	report.Rejected = append(report.Rejected, "candidate-view-disagreement")
	first.Signature[0] ^= 1
	if verifyAudit(first, auditors[0]) {
		return errors.New("simulation accepted forged Candidate View auditor attestation")
	}
	report.Rejected = append(report.Rejected, "auditor-attestation-forged")
	return nil
}

func exerciseBuild(report *Report, builders []signer) error {
	evidence := newBuildEvidence(builders)
	if !verifyBuild(evidence, builders) {
		return errors.New("simulation builder attestations did not verify")
	}
	report.Passed = append(report.Passed, "two-reproducible-builder-attestations")
	evidence.Artifact = digest([]byte("different-artifact"))
	if verifyBuild(evidence, builders) {
		return errors.New("simulation accepted mismatched builder artifact")
	}
	report.Rejected = append(report.Rejected, "builder-artifact-mismatch")
	return nil
}

func fullAudit(auditor string, input []byte, signer signer) (auditResult, error) {
	view, err := reconstructView(input)
	if err != nil {
		return auditResult{}, err
	}
	result := auditResult{Auditor: auditor, ToolRevision: "candidate-view-simulator-v1", View: view}
	result.Signature = ed25519.Sign(signer.private, auditMessage(result))
	return result, nil
}

func reconstructView(input []byte) (candidateView, error) {
	lines := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")
	if len(lines) != 6 || lines[3] != "cutoff=3" || lines[4] != "rules=candidate-view-v1" || lines[5] != "materialization=materialization-v1" ||
		strings.Join(lines[:3], "\n") != "candidate-a\ncandidate-b\ncandidate-c" {
		return candidateView{}, errors.New("simulation Candidate View input is not canonical")
	}
	accepted, rejected := "candidate-a\ncandidate-b", "candidate-c:capacity"
	root := digest([]byte("rules=candidate-view-v1\naccepted=" + accepted + "\nrejected=" + rejected + "\n"))
	return candidateView{InputDigest: digest(input), Rules: "candidate-view-v1", Cutoff: 3, Root: root, Length: 2,
		Summary: "accepted=2;rejected=1;capacity=2", Proof: digest([]byte(root + "\nindex=0\ncandidate-a"))}, nil
}

func verifyAudit(value auditResult, signer signer) bool {
	return ed25519.Verify(signer.public, auditMessage(value), value.Signature)
}

func auditMessage(value auditResult) []byte {
	return []byte(value.Auditor + "\n" + value.ToolRevision + "\n" + value.View.InputDigest + "\n" + value.View.Root + "\n" + value.View.Proof)
}

func newBuildEvidence(builders []signer) buildEvidence {
	value := buildEvidence{Source: digest([]byte("source-revision")), Dependencies: digest([]byte("resolved-dependencies")), Recipe: digest([]byte("build-recipe")),
		SBOM: digest([]byte("sbom")), Qualification: digest([]byte("qualification"))}
	value.Artifact = digest([]byte(value.Source + "\n" + value.Dependencies + "\n" + value.Recipe + "\n" + value.SBOM + "\n" + value.Qualification))
	message := buildMessage(value)
	for index, builder := range builders {
		value.Attestations = append(value.Attestations, buildAttestation{Builder: index, Signature: ed25519.Sign(builder.private, message)})
	}
	return value
}

func verifyBuild(value buildEvidence, builders []signer) bool {
	if len(builders) != 2 || len(value.Attestations) != 2 || value.Artifact != newBuildEvidence(nil).Artifact {
		return false
	}
	message := buildMessage(value)
	for index, attestation := range value.Attestations {
		if attestation.Builder != index || !ed25519.Verify(builders[index].public, message, attestation.Signature) {
			return false
		}
	}
	return true
}

func buildMessage(value buildEvidence) []byte {
	return []byte(value.Source + "\n" + value.Dependencies + "\n" + value.Recipe + "\n" + value.SBOM + "\n" + value.Qualification + "\n" + value.Artifact)
}

func canonicalEmergency(value emergencyAction) string {
	return value.Kind + "\n" + value.Scope + "\n" + value.Expires.Format(time.RFC3339)
}

func validEmergency(value emergencyAction, at time.Time) bool {
	return value.Kind == "emergency-stop" && value.Scope == "disable-only" && at.Before(value.Expires)
}

func canonicalRotation(value rotation) string {
	return value.Reason + "\n" + value.Predecessor + "\n" + value.Successor + "\n" + value.Expires.Format(time.RFC3339)
}

func validRotation(value rotation, expectedPredecessor string, at time.Time) bool {
	validReason := map[string]bool{"loss": true, "compromise": true, "removal": true, "replacement": true, "emergency-recovery": true}
	return validReason[value.Reason] && value.Predecessor == expectedPredecessor && value.Successor != value.Predecessor && at.Before(value.Expires)
}

func newSigners(count int) ([]signer, error) {
	result := make([]signer, count)
	for index := range result {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generate simulation signer %d: %w", index+1, err)
		}
		result[index] = signer{public: public, private: private}
	}
	return result, nil
}

func sign(message []byte, signers []signer) []signature {
	result := make([]signature, len(signers))
	for index, value := range signers {
		result[index] = signature{public: value.public, value: ed25519.Sign(value.private, message)}
	}
	return result
}

func verifyQuorum(message []byte, members []signer, signatures []signature, threshold int) bool {
	known, valid := make(map[string]bool, len(members)), make(map[string]bool, len(signatures))
	for _, member := range members {
		known[string(member.public)] = true
	}
	for _, value := range signatures {
		identity := string(value.public)
		if known[identity] && !valid[identity] && ed25519.Verify(value.public, message, value.value) {
			valid[identity] = true
		}
	}
	return len(valid) >= threshold
}

func digest(value []byte) string { sum := sha256.Sum256(value); return fmt.Sprintf("sha256:%x", sum) }
