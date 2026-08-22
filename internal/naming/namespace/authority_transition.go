package namespace

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"time"
)

const transitionDomain = "ardents-name-authority-transition-v1"

// SignTransition authenticates one exact control transition with the current
// Name Authority. A child claim is authenticated by its immediate parent.
func SignTransition(network [32]byte, current Record, op Op,
	private ed25519.PrivateKey,
) ([]byte, error) {
	if network == [32]byte{} || len(private) != ed25519.PrivateKeySize ||
		current.Authority != hex.EncodeToString(private.Public().(ed25519.PublicKey)) ||
		!supportedTransition(current, op) {
		return nil, errors.New("invalid Name Authority transition signer or operation")
	}
	transcript, err := transitionTranscript(network, current, op)
	if err != nil {
		return nil, err
	}
	return ed25519.Sign(private, transcript), nil
}

// TransitionDigest binds anonymous admission to one exact current Record and
// lifecycle operation without disclosing an Endpoint identity.
func TransitionDigest(network [32]byte, current Record, op Op) ([32]byte, error) {
	transcript, err := transitionTranscript(network, current, op)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(transcript), nil
}

// ApplyAdmittedTransition verifies anonymous admission and the operation's
// predecessor or threshold authorization before applying it.
func ApplyAdmittedTransition(admission *Admission, admissionProof Proof,
	admissionAt int64, admissionDigest [32]byte, network [32]byte, current Record, op Op,
	proof []byte, now int64, policy Policy,
) (Record, error) {
	_, err := TransitionDigest(network, current, op)
	surface, allowed := transitionSurface(op.Kind)
	if err != nil || !allowed || admission == nil || admissionProof.Challenge.Surface != surface ||
		admissionDigest == [32]byte{} || admissionProof.Challenge.OperationDigest != admissionDigest {
		return Record{}, errors.New("invalid Name Authority transition admission")
	}
	if accepted, _ := admission.Verify(admissionAt, admissionProof); !accepted {
		return Record{}, errors.New("denied Name Authority transition admission")
	}
	if recoveryThresholdTransition(op.Kind) {
		if len(proof) != 0 || op.Authority != "" {
			return Record{}, errors.New("recovery transition contains an authority bypass")
		}
		return Apply(&current, now, op, policy)
	}
	return applyTransition(network, current, op, proof, now, policy)
}

func applyTransition(network [32]byte, current Record, op Op,
	proof []byte, now int64, policy Policy,
) (Record, error) {
	public, err := canonicalAuthority(current.Authority)
	if err != nil || len(proof) != ed25519.SignatureSize || !supportedTransition(current, op) {
		return Record{}, errors.New("invalid Name Authority transition proof")
	}
	transcript, err := transitionTranscript(network, current, op)
	if err != nil || !ed25519.Verify(public, transcript, proof) {
		return Record{}, errors.New("invalid Name Authority transition signature")
	}
	if op.Kind == "claim" {
		return Apply(nil, now, op, policy)
	}
	return Apply(&current, now, op, policy)
}

func supportedTransition(current Record, op Op) bool {
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

func transitionTranscript(network [32]byte, current Record, op Op) ([]byte, error) {
	currentWire, err := EncodeRecord(current)
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
