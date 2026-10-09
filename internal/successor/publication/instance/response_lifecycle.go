package instance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
)

type Acceptance struct {
	State            State
	Generation       uint64
	CredentialDigest [32]byte
}

// Accept consumes one external untrusted response. Identical accepted retries
// are idempotent; a different response cannot retain the original private key.
func (root *Root) Accept(ctx context.Context, raw []byte) (Acceptance, error) {
	if root == nil || ctx == nil {
		return Acceptance{}, ErrUnavailable
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.check(ctx); err != nil {
		return Acceptance{}, err
	}
	switch root.state.phase {
	case Accepted:
		if bytes.Equal(raw, root.state.response) {
			return root.acceptance()
		}
		return Acceptance{}, root.terminal(ctx, Conflicting, raw)
	case Pending:
		credential, err := verifyResponse(raw, root.state.request)
		if err != nil {
			phase := Rejected
			if errors.Is(err, ErrUnavailable) {
				phase = Conflicting
			}
			return Acceptance{}, root.terminal(ctx, phase, raw)
		}
		next := root.state.clone()
		next.phase, next.response = Accepted, append([]byte(nil), raw...)
		if err = root.writeState(ctx, next); err != nil {
			next.erase()
			return Acceptance{}, err
		}
		root.replace(next)
		return Acceptance{State: Accepted, Generation: credential.Delegation().Generation, CredentialDigest: credential.Digest()}, nil
	case Consumed:
		result, err := root.acceptance()
		return result, errors.Join(err, ErrSuccessorRequired)
	default:
		return Acceptance{}, ErrUnavailable
	}
}

func (root *Root) terminal(ctx context.Context, phase State, raw []byte) error {
	next := root.state.clone()
	next.phase, next.terminal = phase, sha256.Sum256(raw)
	next.redact()
	if phase == Rejected {
		clear(next.response)
		next.response = nil
	}
	if err := root.writeState(ctx, next); err != nil {
		next.erase()
		return err
	}
	root.replace(next)
	return ErrUnavailable
}

func (root *Root) acceptance() (Acceptance, error) {
	credential, err := verifyResponse(root.state.response, root.state.request)
	if err != nil {
		return Acceptance{}, err
	}
	return Acceptance{State: root.state.phase, Generation: credential.Delegation().Generation, CredentialDigest: credential.Digest()}, nil
}

func (root *Root) replace(next generationState) { root.state.erase(); root.state = next }
