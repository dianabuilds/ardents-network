package nativecircuit

import (
	"bytes"
	"crypto/ecdh"
	"crypto/hpke"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	invitationSchema  = "carrier-lab-c5-c2-invitation/v1"
	maximumInvitation = 4_096
)

var hpkeInfo = []byte("ardents/carrier-lab/c5-c2/invitation/v1")

type handle [32]byte

type invitation struct {
	SchemaVersion  string `json:"schema_version"`
	Profile        string `json:"profile"`
	RunID          string `json:"run_id"`
	Rendezvous     string `json:"rendezvous"`
	JoinToken      handle `json:"join_token"`
	HandshakeNonce handle `json:"handshake_nonce"`
	ExpiresUnix    int64  `json:"expires_unix"`
}

type invitationGuard struct {
	profile    string
	runID      string
	rendezvous string
	now        time.Time
	used       map[handle]struct{}
}

func sealInvitation(publicKey *ecdh.PublicKey, value invitation) ([]byte, error) {
	value.SchemaVersion = invitationSchema
	plaintext, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode introduction invitation: %w", err)
	}
	key, err := hpke.NewDHKEMPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("prepare X25519 HPKE public key: %w", err)
	}
	sealed, err := hpke.Seal(key, hpke.HKDFSHA256(), hpke.ChaCha20Poly1305(), hpkeInfo, plaintext)
	if err != nil {
		return nil, fmt.Errorf("seal introduction invitation: %w", err)
	}
	if len(sealed) > maximumInvitation {
		return nil, errors.New("sealed introduction invitation exceeds 4,096 bytes")
	}
	return sealed, nil
}

func newInvitationGuard(profile, runID, rendezvous string, now time.Time) *invitationGuard {
	return &invitationGuard{profile: profile, runID: runID, rendezvous: rendezvous, now: now, used: make(map[handle]struct{})}
}

func (guard *invitationGuard) open(privateKey *ecdh.PrivateKey, sealed []byte) (invitation, error) {
	if len(sealed) == 0 || len(sealed) > maximumInvitation {
		return invitation{}, errors.New("sealed introduction invitation is outside the fixed bound")
	}
	key, err := hpke.NewDHKEMPrivateKey(privateKey)
	if err != nil {
		return invitation{}, fmt.Errorf("prepare X25519 HPKE private key: %w", err)
	}
	plaintext, err := hpke.Open(key, hpke.HKDFSHA256(), hpke.ChaCha20Poly1305(), hpkeInfo, sealed)
	if err != nil {
		return invitation{}, errors.New("introduction invitation authentication failed")
	}
	decoder := json.NewDecoder(bytes.NewReader(plaintext))
	decoder.DisallowUnknownFields()
	var value invitation
	if err := decoder.Decode(&value); err != nil || decoder.More() {
		return invitation{}, errors.New("introduction invitation has invalid encoding")
	}
	if value.SchemaVersion != invitationSchema || value.Profile != guard.profile || value.RunID != guard.runID || value.Rendezvous != guard.rendezvous {
		return invitation{}, errors.New("introduction invitation binding does not match the active route")
	}
	if value.ExpiresUnix <= guard.now.Unix() || value.JoinToken == (handle{}) || value.HandshakeNonce == (handle{}) {
		return invitation{}, errors.New("introduction invitation is expired or incomplete")
	}
	if _, exists := guard.used[value.JoinToken]; exists {
		return invitation{}, errors.New("introduction invitation join token was replayed")
	}
	guard.used[value.JoinToken] = struct{}{}
	return value, nil
}
