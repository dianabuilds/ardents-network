package instance

import (
	"bytes"
	"crypto/ecdh"
	"crypto/hpke"
	"encoding/binary"
	"time"
)

const privateIntroductionInfo = "ardents-introduction-capsule-v3\x00"

// OpenPrivateIntroduction opens only the selected fixed-size capsule under
// the current volatile recipient. It never returns a private key or HPKE
// object. Route owns encoding and Endpoint owns replay and authority checks.
func (recipient *PrivateRecipient) OpenPrivateIntroduction(encapsulation, info, authenticatedHeader, ciphertext []byte, at time.Time) ([]byte, error) {
	if recipient == nil || recipient.root == nil {
		return nil, ErrUnavailable
	}
	root := recipient.root
	root.mu.Lock()
	defer root.mu.Unlock()
	if !root.ownsPrivateRecipientLocked(recipient) || !recipient.binding.usableLocked(root) || len(recipient.private) != 32 ||
		!at.Before(recipient.retireAt) {
		if root.ownsPrivateRecipientLocked(recipient) {
			root.closeOnePrivateRecipientLocked(recipient)
		}
		return nil, ErrUnavailable
	}
	if at.IsZero() || at.Before(recipient.notBefore) {
		return nil, ErrUnavailable
	}
	domainLength := len(privateIntroductionInfo) + 32
	if len(info) != domainLength || !bytes.HasPrefix(info, []byte(privateIntroductionInfo)) ||
		bytes.Equal(info[len(privateIntroductionInfo):], make([]byte, 32)) || len(authenticatedHeader) != domainLength+80 ||
		!bytes.Equal(authenticatedHeader[:domainLength], info) || len(encapsulation) != 32 || len(ciphertext) != 360 {
		return nil, ErrInvalid
	}
	expiry := binary.BigEndian.Uint64(authenticatedHeader[domainLength+40 : domainLength+48])
	if expiry == 0 || expiry > uint64(recipient.expiry.Unix()) || at.Unix() >= int64(expiry) {
		return nil, ErrUnavailable
	}
	private, err := ecdh.X25519().NewPrivateKey(recipient.private)
	if err != nil {
		return nil, ErrInvalid
	}
	key, err := hpke.NewDHKEMPrivateKey(private)
	if err != nil {
		return nil, ErrInvalid
	}
	receiver, err := hpke.NewRecipient(encapsulation, key, hpke.HKDFSHA256(), hpke.AES128GCM(), info)
	if err != nil {
		return nil, err
	}
	return receiver.Open(authenticatedHeader, ciphertext)
}
