package instance

import (
	"crypto/ecdh"
	"crypto/rand"
	"time"
)

// PrivateRecipient is one volatile non-exporting Introduction recipient.
// Its registration owns its closure; a public key alone supplies no authority.
type PrivateRecipient struct {
	root                        *Root
	binding                     *Binding
	private                     []byte
	public                      [32]byte
	notBefore, expiry, retireAt time.Time
}

// NewPrivateRecipient reserves a higher revision on the consumed Instance.
// At most two recipients support current publication and bounded predecessor;
// the revision floor survives their closure within this consumed generation.
func (binding *Binding) NewPrivateRecipient(revision uint64, at, expiry time.Time) (*PrivateRecipient, error) {
	if binding == nil || binding.root == nil {
		return nil, ErrUnavailable
	}
	root := binding.root
	root.mu.Lock()
	defer root.mu.Unlock()
	credential, err := root.state.credential()
	if err != nil || !binding.usableLocked(root) || root.state.Phase != StateConsumed || revision == 0 || revision <= root.privateRevision ||
		!canonicalTime(at) || !canonicalTime(expiry) || !at.Before(expiry) || expiry.Sub(at) > 600*time.Second ||
		at.Unix() < credential.NotBefore || expiry.Unix() > credential.NotAfter {
		return nil, ErrUnavailable
	}
	vacant := -1
	for i, prior := range root.privateRecipients {
		if prior != nil && !at.Before(prior.retireAt) {
			root.closeOnePrivateRecipientLocked(prior)
		}
		if root.privateRecipients[i] == nil && vacant < 0 {
			vacant = i
		}
	}
	if vacant < 0 {
		return nil, ErrUnavailable
	}
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	recipient := &PrivateRecipient{root: root, binding: binding, private: key.Bytes(), notBefore: at, expiry: expiry, retireAt: expiry}
	copy(recipient.public[:], key.PublicKey().Bytes())
	root.privateRevision, root.privateRecipients[vacant] = revision, recipient
	return recipient, nil
}

// RetainPredecessor limits a replaced recipient to at most another 60 seconds.
// It can only shorten its original lifetime, including on repeated calls.
func (recipient *PrivateRecipient) RetainPredecessor(at, until time.Time) error {
	if recipient == nil || recipient.root == nil {
		return ErrUnavailable
	}
	root := recipient.root
	root.mu.Lock()
	defer root.mu.Unlock()
	if !root.ownsPrivateRecipientLocked(recipient) || !recipient.binding.usableLocked(root) ||
		!canonicalTime(at) || !canonicalTime(until) || at.Before(recipient.notBefore) || !at.Before(until) ||
		until.After(at.Add(60*time.Second)) || until.After(recipient.retireAt) {
		return ErrUnavailable
	}
	recipient.retireAt = until
	return nil
}

func (recipient *PrivateRecipient) Public(at time.Time) [32]byte {
	if recipient == nil || recipient.root == nil {
		return [32]byte{}
	}
	root := recipient.root
	root.mu.Lock()
	defer root.mu.Unlock()
	if !root.ownsPrivateRecipientLocked(recipient) || !recipient.binding.usableLocked(root) || len(recipient.private) != 32 || !at.Before(recipient.retireAt) {
		root.closeOnePrivateRecipientLocked(recipient)
		return [32]byte{}
	}
	if at.Before(recipient.notBefore) {
		return [32]byte{}
	}
	return recipient.public
}

func (recipient *PrivateRecipient) Close() error {
	if recipient == nil || recipient.root == nil {
		return nil
	}
	root := recipient.root
	root.mu.Lock()
	defer root.mu.Unlock()
	root.closeOnePrivateRecipientLocked(recipient)
	return nil
}
func (root *Root) ownsPrivateRecipientLocked(recipient *PrivateRecipient) bool {
	return recipient != nil && (root.privateRecipients[0] == recipient || root.privateRecipients[1] == recipient)
}
func (root *Root) closeOnePrivateRecipientLocked(recipient *PrivateRecipient) {
	for i, retained := range root.privateRecipients {
		if recipient != nil && retained == recipient {
			clear(recipient.private)
			recipient.private = nil
			recipient.public = [32]byte{}
			root.privateRecipients[i] = nil
		}
	}
}
func (root *Root) closePrivateRecipientLocked() {
	for _, recipient := range root.privateRecipients {
		root.closeOnePrivateRecipientLocked(recipient)
	}
}
