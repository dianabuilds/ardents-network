package capability

import (
	"crypto/ecdh"
	"crypto/hpke"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"

	identityapi "ardents/internal/identity"
)

const deliveryInfoDomain = "ardents-capability-delivery/1"

func (s *Service) EnsureDeliveryPublicKey() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	private, err := s.deliveryPrivateKeyLocked()
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), private.PublicKey().Bytes()...), nil
}

func (s *Service) ReceiveDeliveredGrant(ciphertext []byte) (identityapi.CapabilityRef, error) {
	s.mu.Lock()
	grant, err := s.openDeliveredGrantLocked(ciphertext, s.localPrincipal)
	s.mu.Unlock()
	if err != nil {
		return "", err
	}
	return s.ImportGrant(grant)
}

func (s *Service) openDeliveredGrantLocked(ciphertext []byte, subject string) (identityapi.CapabilityGrant, error) {
	private, err := s.deliveryPrivateKeyLocked()
	if err != nil {
		return identityapi.CapabilityGrant{}, err
	}
	plain, err := hpke.Open(
		private, hpke.HKDFSHA256(), hpke.ChaCha20Poly1305(),
		deliveryInfo(subject), ciphertext,
	)
	if err != nil {
		return identityapi.CapabilityGrant{}, fmt.Errorf("capability delivery authentication failed")
	}
	var stored persistedGrant
	if err := json.Unmarshal(plain, &stored); err != nil {
		return identityapi.CapabilityGrant{}, fmt.Errorf("capability delivery decode failed")
	}
	grant, err := stored.restore()
	if err != nil {
		return identityapi.CapabilityGrant{}, err
	}
	if grant.SubjectPrincipal != subject {
		return identityapi.CapabilityGrant{}, fmt.Errorf("capability delivery subject mismatch")
	}
	return grant, nil
}

func SealGrantForRecipient(grant identityapi.CapabilityGrant, attestation identityapi.CapabilityDeliveryAttestation, at time.Time) ([]byte, error) {
	if grant.SubjectPrincipal == "" {
		return nil, fmt.Errorf("capability delivery subject is missing")
	}
	if err := VerifyDeliveryAttestation(attestation, at); err != nil {
		return nil, err
	}
	if attestation.SubjectPrincipal != grant.SubjectPrincipal {
		return nil, fmt.Errorf("capability delivery attestation subject mismatch")
	}
	kem := hpke.DHKEM(ecdh.X25519())
	public, err := kem.NewPublicKey(attestation.DeliveryPublicKey)
	if err != nil {
		return nil, fmt.Errorf("capability delivery public key is invalid")
	}
	plain, err := json.Marshal(persistGrant(grant))
	if err != nil {
		return nil, err
	}
	return hpke.Seal(
		public, hpke.HKDFSHA256(), hpke.ChaCha20Poly1305(),
		deliveryInfo(grant.SubjectPrincipal), plain,
	)
}

func (s *Service) deliveryPrivateKeyLocked() (hpke.PrivateKey, error) {
	kem := hpke.DHKEM(ecdh.X25519())
	if len(s.ledger.DeliveryPrivateKey) != 0 {
		private, err := kem.NewPrivateKey(s.ledger.DeliveryPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("capability delivery private key is invalid")
		}
		return private, nil
	}
	private, err := kem.GenerateKey()
	if err != nil {
		return nil, err
	}
	raw, err := private.Bytes()
	if err != nil {
		return nil, err
	}
	next := cloneLedger(s.ledger)
	next.DeliveryPrivateKey = append([]byte(nil), raw...)
	if err := s.store.save(next); err != nil {
		return nil, err
	}
	s.ledger = next
	return private, nil
}

func deliveryInfo(subject string) []byte {
	raw := []byte(subject)
	out := make([]byte, len(deliveryInfoDomain)+2+len(raw))
	copy(out, deliveryInfoDomain)
	binary.BigEndian.PutUint16(out[len(deliveryInfoDomain):], uint16(len(raw)))
	copy(out[len(deliveryInfoDomain)+2:], raw)
	return out
}
