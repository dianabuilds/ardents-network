package transport

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	db "ardents/internal/persistence"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

type transportKeyStore struct {
	path string
}

type transportKeyLedger struct {
	PrivateKey string `json:"private_key"`
}

func newTransportKeyStore(path string) transportKeyStore {
	return transportKeyStore{path: strings.TrimSpace(path)}
}

func (s transportKeyStore) Ensure(requireExisting bool) (*ecdsa.PrivateKey, error) {
	if key, err := s.load(); err != nil || key != nil {
		return key, err
	}
	if requireExisting {
		return nil, fmt.Errorf("transport key is missing while persistent Waku Store exists; restore matching backup")
	}
	key, err := gethcrypto.GenerateKey()
	if err != nil {
		return nil, err
	}
	if err := s.save(key); err != nil {
		return nil, err
	}
	return key, nil
}

func (s transportKeyStore) load() (*ecdsa.PrivateKey, error) {
	if s.path == "" {
		return nil, nil
	}
	raw, found, err := db.ReadPrivateFile(s.path)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("transport key file is empty")
	}
	var stored transportKeyLedger
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, err
	}
	if strings.TrimSpace(stored.PrivateKey) == "" {
		return nil, fmt.Errorf("transport key file does not contain a private key")
	}
	decoded, err := hex.DecodeString(stored.PrivateKey)
	if err != nil {
		return nil, err
	}
	return gethcrypto.ToECDSA(decoded)
}

func (s transportKeyStore) save(key *ecdsa.PrivateKey) error {
	if s.path == "" || key == nil {
		return nil
	}
	raw, err := json.MarshalIndent(transportKeyLedger{
		PrivateKey: hex.EncodeToString(gethcrypto.FromECDSA(key)),
	}, "", "  ")
	if err != nil {
		return err
	}
	return db.AtomicWritePrivateFile(s.path, raw)
}
