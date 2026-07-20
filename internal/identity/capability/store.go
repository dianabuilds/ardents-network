package capability

import (
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	db "ardents/internal/persistence"

	"golang.org/x/crypto/chacha20poly1305"
)

const storeBucket = "identity-capabilities"
const storeKey = "sealed-ledger"
const storeAAD = "ardents-capability-store/1"

type Store struct {
	path string
	key  [chacha20poly1305.KeySize]byte
}

type sealedLedger struct {
	Version    uint32 `json:"version"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

type ledger struct {
	Grants             map[string]persistedGrant      `json:"grants"`
	SenderGrants       map[string]persistedGrant      `json:"sender_grants"`
	Revocations        map[string]persistedRevocation `json:"revocations"`
	DeliveryPrivateKey []byte                         `json:"delivery_private_key,omitempty"`
}

func newStore(path string, key []byte) (*Store, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("capability store key must be 32 bytes")
	}
	store := &Store{path: path}
	derived, err := hkdf.Key(sha256.New, key, nil, "ardents-capability-store-encryption/1", len(store.key))
	if err != nil {
		return nil, err
	}
	copy(store.key[:], derived)
	return store, nil
}

func (s *Store) load() (ledger, error) {
	var sealed sealedLedger
	found, err := db.LoadJSON(s.path, storeBucket, storeKey, &sealed)
	if err != nil || !found {
		return emptyLedger(), err
	}
	if sealed.Version != 1 {
		return ledger{}, fmt.Errorf("capability store version is unsupported")
	}
	plain, err := s.open(sealed)
	if err != nil {
		return ledger{}, fmt.Errorf("capability store authentication failed")
	}
	var stored ledger
	if err := json.Unmarshal(plain, &stored); err != nil {
		return ledger{}, fmt.Errorf("capability store decode failed")
	}
	normalizeLedger(&stored)
	return stored, nil
}

func (s *Store) save(stored ledger) error {
	plain, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	aead, err := chacha20poly1305.NewX(s.key[:])
	if err != nil {
		return err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	sealed := sealedLedger{
		Version:    1,
		Nonce:      nonce,
		Ciphertext: aead.Seal(nil, nonce, plain, []byte(storeAAD)),
	}
	return db.SaveJSON(s.path, storeBucket, storeKey, sealed)
}

func (s *Store) open(sealed sealedLedger) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(s.key[:])
	if err != nil {
		return nil, err
	}
	if len(sealed.Nonce) != aead.NonceSize() {
		return nil, fmt.Errorf("capability store nonce is invalid")
	}
	return aead.Open(nil, sealed.Nonce, sealed.Ciphertext, []byte(storeAAD))
}

func emptyLedger() ledger {
	return ledger{
		Grants:       map[string]persistedGrant{},
		SenderGrants: map[string]persistedGrant{},
		Revocations:  map[string]persistedRevocation{},
	}
}

func normalizeLedger(stored *ledger) {
	if stored.Grants == nil {
		stored.Grants = map[string]persistedGrant{}
	}
	if stored.SenderGrants == nil {
		stored.SenderGrants = map[string]persistedGrant{}
	}
	if stored.Revocations == nil {
		stored.Revocations = map[string]persistedRevocation{}
	}
}
