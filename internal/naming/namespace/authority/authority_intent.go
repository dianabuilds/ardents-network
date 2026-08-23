package authority

import (
	"bytes"
	"errors"
)

// Intent is one canonical unsigned existing-Name control request. It is the
// only input accepted by Prepare; it cannot carry a caller-supplied Authority
// proof or successor Record.
type Intent struct {
	raw    []byte
	digest [32]byte
}

// OpenIntent admits the custody-derived control wire before authentication.
// Transport binds only this opaque canonical value and its static digest.
func OpenIntent(raw []byte) (Intent, error) {
	var operation controlOperation
	if len(raw) == 0 || len(raw) > 16<<10 || decodeCanonical(raw, &operation) != nil ||
		operation.Network != [32]byte{} || operation.Nonce != [32]byte{} || operation.Deadline != 0 ||
		operation.SigningMode != custodyDerivedSigningMode || len(operation.AuthorityProof) != 0 ||
		len(operation.SuccessorRecord) != 0 || operation.OperationDigest == [32]byte{} {
		return Intent{}, errors.New("name Authority control intent is invalid")
	}
	if operation.OperationDigest != canonicalControlDigest(operation) {
		return Intent{}, errors.New("name Authority control intent digest is invalid")
	}
	if !validControlIntent(operation) {
		return Intent{}, errors.New("name Authority control intent shape is invalid")
	}
	return Intent{raw: append([]byte(nil), raw...), digest: operation.OperationDigest}, nil
}

// Digest returns the exact static intent digest that anonymous admission and
// the prepared Submission bind.
func (intent Intent) Digest() [32]byte { return intent.digest }

func (intent Intent) operation() (controlOperation, error) {
	operation, err := decodeControlIntent(intent.raw)
	if err != nil || operation.OperationDigest != intent.digest {
		return controlOperation{}, errors.New("name Authority control intent is invalid")
	}
	return operation, nil
}

func decodeControlIntent(raw []byte) (controlOperation, error) {
	intent, err := OpenIntent(raw)
	if err != nil || !bytes.Equal(intent.raw, raw) {
		return controlOperation{}, errors.New("name Authority control intent is invalid")
	}
	var operation controlOperation
	if decodeCanonical(raw, &operation) != nil {
		return controlOperation{}, errors.New("name Authority control intent is invalid")
	}
	return operation, nil
}

func validControlIntent(value controlOperation) bool {
	if value.SigningMode != custodyDerivedSigningMode || len(value.AuthorityProof) != 0 ||
		len(value.SuccessorRecord) != 0 {
		return false
	}
	signed := value
	signed.AuthorityProof = []byte{1}
	switch value.Kind {
	case "renew", "record", "release", "transfer", "policy":
		return validControlOperation(signed)
	case "recovery":
		return value.RecoveryStep == "resume" && validControlOperation(signed)
	default:
		return false
	}
}
