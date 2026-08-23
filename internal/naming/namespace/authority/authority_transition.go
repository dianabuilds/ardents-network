package authority

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/admission"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
)

const transitionDomain = "ardents-name-authority-transition-v1"

// TransitionSigner owns one Authority private-key operation for an exact
// Namespace-derived transition. It receives no caller-constructible record or
// transcript bytes.
type TransitionSigner interface {
	Sign(TransitionSigningRequest) ([]byte, error)
}

// ControlSigner owns the paired Authority operations that prepare one
// existing-Name control submission. Namespace derives both requests and never
// gives the caller a lifecycle record or transcript to construct.
type ControlSigner interface {
	SignTransition(TransitionSigningRequest) ([]byte, error)
	SignRecord(record.RecordSigningRequest) ([]byte, error)
}

// TransitionSigningRequest is the sealed transition transcript generated from
// one canonical predecessor and lifecycle operation.
type TransitionSigningRequest struct {
	authority  [ed25519.PublicKeySize]byte
	generation uint64
	revision   uint64
	transcript []byte
}

// Authority returns the exact Authority key required for this transition.
func (request TransitionSigningRequest) Authority() [ed25519.PublicKeySize]byte {
	return request.authority
}

// Predecessor returns the exact current Record generation and revision from
// which Namespace derived this transition.
func (request TransitionSigningRequest) Predecessor() (uint64, uint64) {
	return request.generation, request.revision
}

// Transcript returns a copy of the exact Namespace transition transcript.
func (request TransitionSigningRequest) Transcript() []byte {
	return append([]byte(nil), request.transcript...)
}

// SignTransition authenticates one exact control transition with the current
// Name Authority. A child claim is authenticated by its immediate parent.
func SignTransition(network [32]byte, current record.Record, op record.Op,
	private ed25519.PrivateKey,
) ([]byte, error) {
	if len(private) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid Name Authority transition signer or operation")
	}
	return SignTransitionWith(network, current, op, directTransitionSigner{private: private})
}

// SignTransitionWith authenticates one exact transition through a sealed
// Namespace signer request. It is the custody-facing signing boundary.
func SignTransitionWith(network [32]byte, current record.Record, op record.Op, signer TransitionSigner) ([]byte, error) {
	if signer == nil {
		return nil, errors.New("invalid Name Authority transition signer or operation")
	}
	request, err := newTransitionSigningRequest(network, current, op)
	if err != nil {
		return nil, err
	}
	signature, err := signer.Sign(request)
	if err != nil {
		return nil, err
	}
	return request.seal(signature)
}

type directTransitionSigner struct{ private ed25519.PrivateKey }

func (signer directTransitionSigner) Sign(request TransitionSigningRequest) ([]byte, error) {
	if len(signer.private) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid Name Authority transition signer")
	}
	public, ok := signer.private.Public().(ed25519.PublicKey)
	if !ok || len(public) != ed25519.PublicKeySize {
		return nil, errors.New("invalid Name Authority transition signer")
	}
	var actual [ed25519.PublicKeySize]byte
	copy(actual[:], public)
	if actual != request.authority {
		return nil, errors.New("invalid Name Authority transition signer")
	}
	return ed25519.Sign(signer.private, request.transcript), nil
}

func newTransitionSigningRequest(network [32]byte, current record.Record, op record.Op) (TransitionSigningRequest, error) {
	if network == [32]byte{} || !supportedTransition(current, op) {
		return TransitionSigningRequest{}, errors.New("invalid Name Authority transition signer or operation")
	}
	public, err := record.AuthorityKey(current.Authority)
	if err != nil {
		return TransitionSigningRequest{}, errors.New("invalid Name Authority transition signer or operation")
	}
	transcript, err := transitionTranscript(network, current, op)
	if err != nil {
		return TransitionSigningRequest{}, err
	}
	request := TransitionSigningRequest{generation: current.Generation, revision: current.Revision, transcript: transcript}
	copy(request.authority[:], public)
	return request, nil
}

func (request TransitionSigningRequest) seal(signature []byte) ([]byte, error) {
	if len(request.transcript) == 0 || len(signature) != ed25519.SignatureSize ||
		!ed25519.Verify(ed25519.PublicKey(request.authority[:]), request.transcript, signature) {
		return nil, errors.New("invalid Name Authority transition signature")
	}
	return append([]byte(nil), signature...), nil
}

// TransitionDigest binds anonymous admission to one exact current Record and
// lifecycle operation without disclosing an Endpoint identity.
func TransitionDigest(network [32]byte, current record.Record, op record.Op) ([32]byte, error) {
	transcript, err := transitionTranscript(network, current, op)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(transcript), nil
}

// ApplyAdmittedTransition verifies anonymous admission and the operation's
// predecessor or threshold authorization before applying it.
func ApplyAdmittedTransition(gate *admission.Admission, admissionProof admission.Proof,
	admissionAt int64, admissionDigest [32]byte, network [32]byte, current record.Record, op record.Op,
	proof []byte, now int64, policy record.Policy,
) (record.Record, error) {
	_, err := TransitionDigest(network, current, op)
	surface, allowed := transitionSurface(op.Kind)
	if err != nil || !allowed || gate == nil || admissionProof.Challenge.Surface != surface ||
		admissionDigest == [32]byte{} || admissionProof.Challenge.OperationDigest != admissionDigest {
		return record.Record{}, errors.New("invalid Name Authority transition admission")
	}
	if accepted, _ := gate.Verify(admissionAt, admissionProof); !accepted {
		return record.Record{}, errors.New("denied Name Authority transition admission")
	}
	if recoveryThresholdTransition(op.Kind) {
		if len(proof) != 0 || op.Authority != "" {
			return record.Record{}, errors.New("recovery transition contains an authority bypass")
		}
		return record.ApplyLegacy(&current, now, op, policy)
	}
	return applyTransition(network, current, op, proof, now, policy)
}

func applyTransition(network [32]byte, current record.Record, op record.Op,
	proof []byte, now int64, policy record.Policy,
) (record.Record, error) {
	public, err := record.AuthorityKey(current.Authority)
	if err != nil || len(proof) != ed25519.SignatureSize || !supportedTransition(current, op) {
		return record.Record{}, errors.New("invalid Name Authority transition proof")
	}
	transcript, err := transitionTranscript(network, current, op)
	if err != nil || !ed25519.Verify(public, transcript, proof) {
		return record.Record{}, errors.New("invalid Name Authority transition signature")
	}
	if op.Kind == "claim" {
		return record.ApplyLegacy(nil, now, op, policy)
	}
	return record.ApplyLegacy(&current, now, op, policy)
}

func supportedTransition(current record.Record, op record.Op) bool {
	switch op.Kind {
	case "renew", "release", "publish", "rotate", "transfer", "schedule-recovery-policy", "resume-recovery":
		return current.Name == op.Name
	case "claim":
		return len(op.Parents) > 0 && op.Parents[0] == current && current.Name != op.Name
	default:
		return false
	}
}

func transitionSurface(kind string) (string, bool) {
	switch kind {
	case "claim", "renew", "release", "publish", "rotate", "transfer":
		return "renewal-update", true
	case "schedule-recovery-policy", "start-recovery", "cancel-recovery", "complete-recovery", "resume-recovery":
		return "policy-recovery", true
	default:
		return "", false
	}
}

func recoveryThresholdTransition(kind string) bool {
	return kind == "start-recovery" || kind == "cancel-recovery" || kind == "complete-recovery"
}

func transitionTranscript(network [32]byte, current record.Record, op record.Op) ([]byte, error) {
	currentWire, err := record.EncodeRecord(current)
	if err != nil {
		return nil, errors.New("current Name Record is invalid")
	}
	out := appendTransitionText(nil, transitionDomain)
	out = append(out, network[:]...)
	out = appendTransitionBytes(out, currentWire)
	out = appendTransitionText(out, op.Kind)
	out = appendTransitionText(out, op.Name)
	out = binary.BigEndian.AppendUint64(out, op.Generation)
	out = binary.BigEndian.AppendUint32(out, op.ClaimOrdinal)
	out = binary.BigEndian.AppendUint64(out, op.ExpectedGeneration)
	out = binary.BigEndian.AppendUint64(out, op.ExpectedRevision)
	out = appendTransitionText(out, op.Authority)
	out = appendTransitionText(out, op.SuccessorAuthority)
	out = append(out, op.Target[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(op.RecordNotAfter))
	out = binary.BigEndian.AppendUint64(out, uint64(op.LeaseDuration))
	out = binary.BigEndian.AppendUint64(out, uint64(op.GraceDuration))
	out = appendTransitionText(out, op.ConflictContext)
	out = append(out, op.PolicyDigest[:]...)
	out = binary.BigEndian.AppendUint64(out, op.PolicyRevision)
	out = binary.BigEndian.AppendUint64(out, uint64(op.PolicyDelay/time.Millisecond))
	out = binary.BigEndian.AppendUint64(out, uint64(op.PolicyActivatesAt))
	authorization := op.RecoveryAuthorization
	out = appendTransitionText(out, authorization.Operation)
	out = append(out, authorization.PolicyDigest[:]...)
	out = binary.BigEndian.AppendUint64(out, authorization.PolicyRevision)
	out = append(out, authorization.OperationID[:]...)
	out = append(out, authorization.Successor[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(authorization.StartedAt))
	out = binary.BigEndian.AppendUint64(out, uint64(authorization.CompletesAt))
	out = append(out, authorization.ValidSigners)
	return out, nil
}

func appendTransitionText(out []byte, value string) []byte {
	return appendTransitionBytes(out, []byte(value))
}

func appendTransitionBytes(out, value []byte) []byte {
	out = binary.BigEndian.AppendUint64(out, uint64(len(value)))
	return append(out, value...)
}
