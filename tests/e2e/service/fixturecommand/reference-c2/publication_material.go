//go:build browsercompat

package main

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

func writePublication(path string, value publicationEnvelope) error {
	if filepath.Dir(path) == "." {
		return errors.New("publisher publication path is not absolute")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func readPublication(path string) (publicationEnvelope, error) {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 || len(raw) > 8<<10 {
		return publicationEnvelope{}, errors.New("publisher publication is unavailable")
	}
	var value publicationEnvelope
	if err := json.Unmarshal(raw, &value); err != nil || value.AuthorityPublic == "" || value.Publication == "" || value.Descriptor == "" ||
		value.AlphaAuthorityPublic == "" || value.AlphaCorpus == "" || value.AlphaLink == "" {
		return publicationEnvelope{}, errors.New("publisher publication is invalid")
	}
	return value, nil
}

func acknowledgement(credential publication.Credential, private ed25519.PrivateKey, brokerID [32]byte) []byte {
	body := make([]byte, 149)
	copy(body[:4], "ARIA")
	body[4] = 1
	copy(body[5:37], credential.Target[:])
	binary.BigEndian.PutUint64(body[37:45], credential.Generation)
	binary.BigEndian.PutUint64(body[45:53], uint64(credential.NotAfter))
	copy(body[53:85], credential.NetworkID[:])
	copy(body[85:117], brokerID[:])
	body[117] = 1
	signature := ed25519.Sign(private, append([]byte("ardents-service-introduction-ack-v1\x00"), body...))
	return append(body, signature...)
}

func hpkePrivateKey() (hpke.PrivateKey, error) {
	private, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return hpke.NewDHKEMPrivateKey(private)
}

func fixed(encoded string) ([32]byte, error) {
	var value [32]byte
	raw, err := hex.DecodeString(encoded)
	if err != nil || len(raw) != len(value) {
		return value, errors.New("C2 fixture fixed value is invalid")
	}
	copy(value[:], raw)
	return value, nil
}
