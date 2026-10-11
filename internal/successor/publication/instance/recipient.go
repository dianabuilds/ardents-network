package instance

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"
)

// Recipient is one independently generated volatile key owned by an exact
// consumed binding/registration. It exposes neither private bytes nor a signer.
type Recipient struct {
	binding      *Binding
	registration *introduction.HolderRegistration
	receipt      introduction.Receipt
	private      []byte
	public       [32]byte
	before, end  time.Time
	acceptUntil  time.Time
	descriptor   []byte
	closed       bool
	users        sync.WaitGroup
	closing      chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewRecipient reserves a strictly higher real registration revision. Retained
// predecessors occupy capacity until their exact owner explicitly joins Close.
func (binding *Binding) NewRecipient(ctx context.Context, registration *introduction.HolderRegistration) (*Recipient, error) {
	if binding == nil || binding.root == nil || registration == nil || ctx == nil {
		return nil, ErrUnavailable
	}
	receipt, err := registration.Receipt()
	if err != nil {
		return nil, err
	}
	facts := receipt.Facts()
	root := binding.root
	root.mu.Lock()
	recipient, err := binding.newRecipientLocked(ctx, registration, receipt, facts)
	root.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if err = registration.CheckReceipt(receipt); err != nil {
		return nil, errors.Join(err, recipient.Close())
	}
	root.mu.Lock()
	err = recipient.check(ctx)
	root.mu.Unlock()
	if err != nil {
		return nil, errors.Join(err, recipient.Close())
	}
	return recipient, nil
}

func (binding *Binding) newRecipientLocked(ctx context.Context, registration *introduction.HolderRegistration, receipt introduction.Receipt, facts introduction.RegistrationFacts) (*Recipient, error) {
	if err := binding.check(ctx); err != nil {
		return nil, err
	}
	value := binding.credential.Delegation()
	before, end := time.Now().UTC().Truncate(time.Second), facts.Expiry.UTC().Truncate(time.Second)
	if end.After(value.NotAfter) {
		end = value.NotAfter
	}
	if !binding.consumed || len(binding.record) == 0 || facts.Network != value.Network || facts.Revision == 0 || facts.Revision <= binding.revision || before.Before(value.NotBefore) || !before.Before(end) || end.Sub(before) > 600*time.Second {
		return nil, ErrUnavailable
	}
	slot := -1
	for i, retained := range binding.recipients {
		if retained == nil {
			slot = i
			break
		}
	}
	if slot < 0 {
		return nil, ErrUnavailable
	}
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	child, cancel := context.WithCancel(ctx)
	recipient := &Recipient{binding: binding, registration: registration, receipt: receipt, private: key.Bytes(), before: before, end: end, acceptUntil: end, ctx: child, cancel: cancel}
	copy(recipient.public[:], key.PublicKey().Bytes())
	binding.revision, binding.recipients[slot] = facts.Revision, recipient
	return recipient, nil
}

func (recipient *Recipient) check(ctx context.Context) error {
	binding := recipient.binding
	if recipient.closed || recipient.ctx == nil || recipient.ctx.Err() != nil || len(recipient.private) != 32 || binding.recipients[0] != recipient && binding.recipients[1] != recipient || !time.Now().Before(recipient.acceptUntil) {
		return ErrUnavailable
	}
	return binding.check(ctx)
}

// Descriptor signs only this owner's canonical key/registration/Publication
// binding. Exact retries preserve the same private key, interval and bytes.
func (recipient *Recipient) Descriptor(ctx context.Context) ([]byte, error) {
	if recipient == nil || recipient.binding == nil || ctx == nil {
		return nil, ErrUnavailable
	}
	if err := recipient.registration.CheckReceipt(recipient.receipt); err != nil {
		return nil, err
	}
	binding := recipient.binding
	root := binding.root
	root.mu.Lock()
	if err := recipient.check(ctx); err != nil {
		root.mu.Unlock()
		return nil, err
	}
	if len(recipient.descriptor) == 0 {
		value := binding.credential.Delegation()
		facts := recipient.receipt.Facts()
		intro := reachability.Introduction{Revision: facts.Revision, Node: facts.Node, Slot: facts.Slot, RecipientKey: recipient.public, NotBefore: recipient.before, NotAfter: recipient.end}
		draft, err := reachability.PrepareDescriptor(binding.record, value.Target, value.Network, facts.Profile, intro, time.Now())
		if err != nil {
			root.mu.Unlock()
			return nil, err
		}
		if len(binding.private) != ed25519.PrivateKeySize {
			root.mu.Unlock()
			return nil, ErrUnavailable
		}
		proof, err := draft.Complete(ed25519.Sign(binding.private, draft.Transcript()), time.Now())
		if err != nil {
			root.mu.Unlock()
			return nil, err
		}
		recipient.descriptor = proof.Bytes()
	}
	raw := append([]byte(nil), recipient.descriptor...)
	root.mu.Unlock()
	if err := recipient.registration.CheckReceipt(recipient.receipt); err != nil {
		return nil, err
	}
	root.mu.Lock()
	err := recipient.check(ctx)
	root.mu.Unlock()
	if err != nil {
		return nil, errors.Join(ErrUnavailable, err)
	}
	return raw, nil
}

func (recipient *Recipient) eraseLocked() {
	clear(recipient.private)
	recipient.private = nil
	for i, retained := range recipient.binding.recipients {
		if retained == recipient {
			recipient.binding.recipients[i] = nil
		}
	}
}

func (recipient *Recipient) Close() error {
	if recipient == nil || recipient.binding == nil {
		return nil
	}
	root := recipient.binding.root
	root.mu.Lock()
	if recipient.closing != nil {
		done := recipient.closing
		root.mu.Unlock()
		<-done
		return nil
	}
	recipient.closed = true
	recipient.closing = make(chan struct{})
	if recipient.cancel != nil {
		recipient.cancel()
	}
	root.mu.Unlock()
	// Acquisition and Add share the root lock with the closed barrier. A user
	// returns under that lock, so physical/crypto joins cannot hold it.
	recipient.users.Wait()
	root.mu.Lock()
	recipient.eraseLocked()
	close(recipient.closing)
	root.mu.Unlock()
	return nil
}
