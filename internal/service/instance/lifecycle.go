package instance

import (
	"bytes"
	"crypto"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/sha256"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

const introductionInfo = "ardents-interactive-route-v1\x00sealed-introduction\x00"

// Accept performs the one durable at-most-once response transition.
func (root *Root) Accept(raw []byte) (Acceptance, error) {
	root.mu.Lock()
	defer root.mu.Unlock()
	if root.closed || !root.state.present() {
		return Acceptance{}, ErrClosed
	}
	if len(raw) == 0 || len(raw) > 1024 {
		if root.state.Phase == StatePending {
			return Acceptance{}, root.terminalResponse(StateRejected, raw)
		}
		if root.state.Phase == StateAccepted {
			return Acceptance{}, root.terminalResponse(StateConflicting, raw)
		}
		return Acceptance{}, ErrUnavailable
	}
	switch root.state.Phase {
	case StateAccepted:
		if bytes.Equal(raw, root.state.Response) {
			return acceptanceFor(root.state)
		}
		return Acceptance{}, root.terminalResponse(StateConflicting, raw)
	case StatePending:
		response, err := ParseResponse(raw)
		request, requestErr := root.state.requestView()
		if err != nil || requestErr != nil {
			return Acceptance{}, root.terminalResponse(StateRejected, raw)
		}
		if response.RequestCommitment != request.Commitment || !credentialMatchesRequest(response.Credential, request) {
			return Acceptance{}, root.terminalResponse(StateConflicting, raw)
		}
		next := cloneState(root.state)
		next.Phase, next.Response = StateAccepted, append([]byte(nil), raw...)
		if err := writeState(root.path, next); err != nil {
			next.erase()
			return Acceptance{}, err
		}
		root.replaceState(next)
		return acceptanceFor(root.state)
	case StateConsumed:
		return acceptanceFor(root.state, ErrSuccessorRequired)
	case StateWithdrawn, StateRejected, StateConflicting:
		return Acceptance{}, ErrUnavailable
	default:
		return Acceptance{}, ErrInvalid
	}
}

// Credential returns a copy of the accepted public delegation without
// opening its signing binding. Runtime composition uses it to reconcile the
// durable publication floor before any Instance authority becomes usable.
func (root *Root) Credential() (publication.Credential, error) {
	root.mu.Lock()
	defer root.mu.Unlock()
	if root.closed || !root.state.present() {
		return publication.Credential{}, ErrClosed
	}
	credential, err := root.state.credential()
	if err != nil {
		if root.state.Phase == StatePending {
			return publication.Credential{}, ErrPending
		}
		return publication.Credential{}, ErrUnavailable
	}
	if root.state.Phase != StateAccepted {
		return publication.Credential{}, ErrUnavailable
	}
	return credential, nil
}

func (root *Root) terminalResponse(phase State, raw []byte) error {
	next := cloneState(root.state)
	next.Phase, next.TerminalDigest = phase, sha256.Sum256(raw)
	if phase == StateRejected {
		zero(next.Response)
		next.Response = nil
	}
	next.eraseSecrets()
	if err := writeState(root.path, next); err != nil {
		next.erase()
		return err
	}
	root.replaceState(next)
	return ErrUnavailable
}

// OpenBinding reconciles the accepted generation against the actual durable
// publication floor before exposing any signing authority.
func (root *Root) OpenBinding(publicationFloor uint64) (*Binding, error) {
	root.mu.Lock()
	defer root.mu.Unlock()
	if root.closed || !root.state.present() {
		return nil, ErrClosed
	}
	credential, err := root.state.credential()
	if err != nil {
		if root.state.Phase == StatePending {
			return nil, ErrPending
		}
		return nil, ErrUnavailable
	}
	switch root.state.Phase {
	case StateAccepted:
		if publicationFloor >= credential.Generation {
			next := cloneState(root.state)
			next.Phase = StateConsumed
			next.eraseSecrets()
			if err := writeState(root.path, next); err != nil {
				next.erase()
				return nil, err
			}
			root.replaceState(next)
			return nil, ErrSuccessorRequired
		}
		if root.bindingOpen || !root.state.secretsMatchPublic() {
			return nil, ErrBusy
		}
		root.bindingOpen = true
		return &Binding{root: root, credentialGeneration: credential.Generation}, nil
	case StatePending:
		return nil, ErrPending
	case StateConsumed:
		return nil, ErrSuccessorRequired
	case StateWithdrawn, StateRejected, StateConflicting:
		return nil, ErrUnavailable
	default:
		return nil, ErrInvalid
	}
}

// Public returns only a copy of the accepted Instance public key.
func (binding *Binding) Public() crypto.PublicKey {
	root, ok := binding.activeRoot()
	if !ok {
		return nil
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if !binding.usableLocked(root) {
		return nil
	}
	return ed25519.PublicKey(append([]byte(nil), root.state.InstancePublic[:]...))
}

// Sign performs an Instance signature without returning private key bytes.
func (binding *Binding) Sign(random io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	root, ok := binding.activeRoot()
	if !ok {
		return nil, ErrUnavailable
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if !binding.usableLocked(root) {
		return nil, ErrUnavailable
	}
	return root.state.InstancePrivate.Sign(random, digest, opts)
}

// Credential returns the authenticated public Credential for this binding.
func (binding *Binding) Credential() publication.Credential {
	root, ok := binding.activeRoot()
	if !ok {
		return publication.Credential{}
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if !binding.usableLocked(root) {
		return publication.Credential{}
	}
	credential, _ := root.state.credential()
	return credential
}

// IntroductionPublic returns only the accepted Introduction recipient public
// key. It returns zero after the binding becomes unavailable.
func (binding *Binding) IntroductionPublic() [32]byte {
	root, ok := binding.activeRoot()
	if !ok {
		return [32]byte{}
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if !binding.usableLocked(root) {
		return [32]byte{}
	}
	return root.state.IntroductionPublic
}

// OpenIntroduction performs only the fixed SealedIntroduction v1 HPKE
// operation without returning a private key or a general recipient object.
func (binding *Binding) OpenIntroduction(encapsulation, info, authenticatedHeader, ciphertext []byte) ([]byte, error) {
	root, ok := binding.activeRoot()
	if !ok {
		return nil, ErrUnavailable
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if !binding.usableLocked(root) || len(encapsulation) != 32 || !bytes.Equal(info, []byte(introductionInfo)) ||
		len(authenticatedHeader) == 0 || len(ciphertext) < 16 || len(ciphertext) > 65535 {
		return nil, ErrUnavailable
	}
	private, err := ecdh.X25519().NewPrivateKey(root.state.IntroductionPrivate)
	if err != nil {
		return nil, ErrInvalid
	}
	recipient, err := hpke.NewDHKEMPrivateKey(private)
	if err != nil {
		return nil, ErrInvalid
	}
	receiver, err := hpke.NewRecipient(encapsulation, recipient, hpke.HKDFSHA256(), hpke.AES128GCM(), info)
	if err != nil {
		return nil, err
	}
	return receiver.Open(authenticatedHeader, ciphertext)
}

// CommitPublished redacts durable private material only after publication has
// committed this exact generation. The live process binding remains usable
// until orderly withdrawal; a restart cannot revive it.
func (binding *Binding) CommitPublished(generation uint64) error {
	root, ok := binding.activeRoot()
	if !ok {
		return ErrUnavailable
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if !binding.usableLocked(root) || generation != binding.credentialGeneration {
		return ErrInvalid
	}
	if root.state.Phase == StateConsumed {
		return nil
	}
	if root.state.Phase != StateAccepted {
		return ErrUnavailable
	}
	next := cloneState(root.state)
	next.Phase = StateConsumed
	next.eraseSecrets()
	if err := writeState(root.path, next); err != nil {
		next.erase()
		return err
	}
	root.state.Phase = StateConsumed
	return nil
}

// Withdraw terminally closes the generation and erases its live private keys.
// The caller must first stop acquisition and drain publication users.
func (binding *Binding) Withdraw() error {
	root, ok := binding.activeRoot()
	if !ok {
		return ErrUnavailable
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if !binding.usableLocked(root) {
		return ErrUnavailable
	}
	next := cloneState(root.state)
	next.Phase = StateWithdrawn
	next.eraseSecrets()
	if err := writeState(root.path, next); err != nil {
		next.erase()
		return err
	}
	root.replaceState(next)
	root.bindingOpen, binding.closed = false, true
	return nil
}

func (binding *Binding) activeRoot() (*Root, bool) {
	if binding == nil || binding.root == nil || binding.closed {
		return nil, false
	}
	return binding.root, true
}

func (binding *Binding) usableLocked(root *Root) bool {
	return !root.closed && root.bindingOpen && !binding.closed && root.state.secretsMatchPublic() &&
		(root.state.Phase == StateAccepted || root.state.Phase == StateConsumed) &&
		binding.credentialGeneration != 0
}

func acceptanceFor(state durableState, causes ...error) (Acceptance, error) {
	credential, err := state.credential()
	return Acceptance{State: state.Phase, Generation: credential.Generation}, errors.Join(err, errors.Join(causes...))
}

func cloneState(state durableState) durableState {
	state.InstancePrivate = append(ed25519.PrivateKey(nil), state.InstancePrivate...)
	state.IntroductionPrivate = append([]byte(nil), state.IntroductionPrivate...)
	state.Response = append([]byte(nil), state.Response...)
	return state
}

func (root *Root) replaceState(next durableState) {
	root.state.erase()
	root.state = next
}
