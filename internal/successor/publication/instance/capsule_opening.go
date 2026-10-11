package instance

import (
	"context"
	"crypto/ecdh"
	"crypto/hpke"
	"crypto/sha256"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/capsule"
)

// Opening retains the original recipient through one fixed capsule operation
// and downstream checks. Decrypted candidate facts grant no acceptance. Close
// joins its cancellation link and clears retained secrets before returning.
type Opening struct {
	mu                sync.Mutex
	recipient         *Recipient
	ctx               context.Context
	cancel            context.CancelFunc
	stopRecipient     func() bool
	recipientCallback chan struct{}
	request           capsule.Request
	digest            [32]byte
	expiry            time.Time
	closed            bool
	once              sync.Once
}

// Context is this original opening's interruption signal. It grants no
// authority and never detaches downstream work from recipient retirement.
func (opening *Opening) Context() context.Context {
	if opening == nil {
		return nil
	}
	return opening.ctx
}

// OpenCapsule performs only the bound v3 opening. The live Publisher must reserve
// rate/replay capacity before this cryptographic work and independently accept
// downstream facts before committing a nonce. No private key or HPKE context
// leaves this owner; Close of recipient/binding waits for this original borrow.
func (recipient *Recipient) OpenCapsule(ctx context.Context, envelope capsule.Envelope) (_ *Opening, result error) {
	if recipient == nil || recipient.binding == nil || recipient.registration == nil || ctx == nil {
		return nil, ErrUnavailable
	}
	if err := recipient.registration.CheckReceipt(recipient.receipt); err != nil {
		return nil, err
	}
	root := recipient.binding.root
	root.mu.Lock()
	if err := recipient.check(ctx); err != nil {
		root.mu.Unlock()
		return nil, err
	}
	header, facts := envelope.Header(), recipient.receipt.Facts()
	at := time.Now()
	if header.Slot != facts.Slot || header.Revision != facts.Revision || header.DeliveryNonce == [32]byte{} ||
		header.Expiry.After(recipient.end) || !at.Before(header.Expiry) || header.Expiry.After(at.Add(10*time.Second)) {
		root.mu.Unlock()
		return nil, ErrUnavailable
	}
	child, cancel := context.WithCancel(ctx)
	opening := &Opening{recipient: recipient, ctx: child, cancel: cancel, expiry: header.Expiry, recipientCallback: make(chan struct{})}
	opening.stopRecipient = context.AfterFunc(recipient.ctx, func() { defer close(opening.recipientCallback); cancel() })
	recipient.users.Add(1)
	private := append([]byte(nil), recipient.private...)
	value := recipient.binding.credential.Delegation()
	publicationDigest := sha256.Sum256(recipient.binding.record)
	root.mu.Unlock()
	defer clear(private)
	defer func() {
		if result != nil {
			opening.Close()
		}
	}()
	enc, cipher := envelope.Encapsulation(), envelope.Ciphertext()
	raw, err := openHPKE(private, enc[:], capsule.Info(facts.Profile), envelope.AssociatedData(facts.Profile), cipher[:])
	if err != nil {
		return nil, err
	}
	defer clear(raw)
	request, err := capsule.ParseRequest(raw)
	if err != nil || request.Network != value.Network || request.Target != value.Target || request.PublicationDigest != publicationDigest ||
		request.ProfileDigest != facts.Profile || request.Revision != facts.Revision || request.Deadline != header.Expiry {
		return nil, errors.Join(ErrInvalid, err)
	}
	opening.request, opening.digest = request, sha256.Sum256(raw)
	if _, _, err := opening.Request(ctx); err != nil {
		return nil, err
	}
	return opening, nil
}

// This unexported suite mechanism has no application authority or key lifetime.
// Its sole runtime caller is the fixed, borrowed capsule operation above.
func openHPKE(private, enc, info, aad, cipher []byte) ([]byte, error) {
	key, err := ecdh.X25519().NewPrivateKey(private)
	if err != nil {
		return nil, err
	}
	secret, err := hpke.NewDHKEMPrivateKey(key)
	if err != nil {
		return nil, err
	}
	context, err := hpke.NewRecipient(enc, secret, hpke.HKDFSHA256(), hpke.AES128GCM(), info)
	if err != nil {
		return nil, err
	}
	return context.Open(aad, cipher)
}

// Request returns a detached bounded candidate and its exact plaintext digest
// only while the original registration, recipient, caller and expiry remain
// usable. It cannot authenticate Rendezvous, Connection or Work Safety facts.
func (opening *Opening) Request(ctx context.Context) (capsule.Request, [32]byte, error) {
	if opening == nil || opening.recipient == nil || opening.recipient.registration == nil || opening.ctx == nil || ctx == nil {
		return capsule.Request{}, [32]byte{}, ErrUnavailable
	}
	r := opening.recipient
	if err := r.registration.CheckReceipt(r.receipt); err != nil {
		return capsule.Request{}, [32]byte{}, err
	}
	opening.mu.Lock()
	defer opening.mu.Unlock()
	if opening.closed || opening.ctx.Err() != nil || ctx.Err() != nil || !time.Now().Before(opening.expiry) {
		return capsule.Request{}, [32]byte{}, ErrUnavailable
	}
	r.binding.root.mu.Lock()
	err := r.check(ctx)
	r.binding.root.mu.Unlock()
	if err != nil {
		return capsule.Request{}, [32]byte{}, err
	}
	return opening.request, opening.digest, nil
}

func (opening *Opening) Close() {
	if opening == nil || opening.recipient == nil || opening.cancel == nil {
		return
	}
	opening.once.Do(func() {
		opening.cancel()
		if !opening.stopRecipient() {
			<-opening.recipientCallback
		}
		opening.mu.Lock()
		opening.closed = true
		opening.request = capsule.Request{}
		opening.digest = [32]byte{}
		opening.mu.Unlock()
		opening.recipient.users.Done()
	})
}
