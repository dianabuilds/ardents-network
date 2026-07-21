package records

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	identityprincipal "ardents/internal/identity/principal"
)

func Canonical(record Record) ([]byte, error) {
	return json.Marshal(struct {
		ID        string    `json:"id"`
		Kind      string    `json:"kind"`
		Subject   string    `json:"subject"`
		Node      string    `json:"node"`
		Device    string    `json:"device"`
		Owner     string    `json:"owner,omitempty"`
		Service   string    `json:"service,omitempty"`
		Mode      string    `json:"mode,omitempty"`
		PublicKey string    `json:"public_key"`
		Endpoints []string  `json:"endpoints"`
		IssuedAt  time.Time `json:"issued_at"`
		ExpiresAt time.Time `json:"expires_at"`
	}{
		ID:        record.ID,
		Kind:      record.Kind,
		Subject:   record.Subject,
		Node:      record.Node,
		Device:    record.Device,
		Owner:     record.Owner,
		Service:   record.Service,
		Mode:      record.Mode,
		PublicKey: record.PublicKey,
		Endpoints: record.Endpoints,
		IssuedAt:  record.IssuedAt,
		ExpiresAt: record.ExpiresAt,
	})
}

func Validate(record Record) error {
	if err := validateRequiredFields(record); err != nil {
		return err
	}
	if err := validateAuthority(record); err != nil {
		return err
	}
	publicKey, signature, err := decodeSignatureInputs(record)
	if err != nil {
		return err
	}
	payload, err := Canonical(record)
	if err != nil {
		return err
	}
	if !ed25519.Verify(publicKey, payload, signature) {
		return errors.New("record signature verification failed")
	}
	return nil
}

func validateRequiredFields(record Record) error {
	switch {
	case record.ID == "":
		return errors.New("record id is required")
	case record.Kind == "":
		return errors.New("record kind is required")
	case record.Subject == "":
		return errors.New("record subject is required")
	case record.PublicKey == "":
		return errors.New("record public key is required")
	case record.Signature == "":
		return errors.New("record signature is required")
	}
	return nil
}

func validateAuthority(record Record) error {
	expectedPrincipal, err := identityprincipal.FromPublicKey(record.PublicKey)
	if err != nil {
		return err
	}
	if record.Node != expectedPrincipal {
		return errors.New("record node does not match signing identity")
	}
	if record.Kind != "node" {
		return validateExpiry(record)
	}
	if record.Subject != expectedPrincipal {
		return errors.New("record subject does not match signing identity")
	}
	if record.ID != expectedPrincipal+":node" {
		return errors.New("record id does not match signing identity")
	}
	return validateExpiry(record)
}

func validateExpiry(record Record) error {
	if !record.ExpiresAt.IsZero() && !record.ExpiresAt.After(time.Now().UTC()) {
		return errors.New("record expired")
	}
	return nil
}

func decodeSignatureInputs(record Record) (ed25519.PublicKey, []byte, error) {
	publicKey, err := base64.StdEncoding.DecodeString(record.PublicKey)
	if err != nil {
		return nil, nil, errors.New("record public key is invalid")
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return nil, nil, errors.New("record public key length is invalid")
	}
	signature, err := base64.StdEncoding.DecodeString(record.Signature)
	if err != nil {
		return nil, nil, errors.New("record signature is invalid")
	}
	if len(signature) != ed25519.SignatureSize {
		return nil, nil, errors.New("record signature length is invalid")
	}
	return publicKey, signature, nil
}
