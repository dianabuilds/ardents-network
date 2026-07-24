package access

import (
	"bytes"
	"crypto/ed25519"
	"fmt"
	"time"

	"ardents/internal/storage"
)

func prepareEnrollmentCredential(node, principal string, credential *Artifact, now time.Time) ([]byte, []byte, error) {
	if credential == nil {
		return nil, nil, errInvalid
	}
	payload := credential.KeyCredentialPayload()
	if payload == nil || payload.Subject != principal {
		return nil, nil, errInvalid
	}
	raw, err := credential.MarshalBinary()
	if err != nil {
		return nil, nil, errInvalid
	}
	verified, err := ParseAndVerifyKeyCredential(raw, now)
	if err != nil || verified.ID() != credential.ID() {
		return nil, nil, errInvalid
	}
	key := tuple([]byte(node), []byte(principal), []byte(payload.DeviceId), []byte(credential.ID()))
	return key, raw, nil
}

func recordEnrollmentCredential(tx storage.WriteTransaction, key, raw []byte) error {
	existing, found, err := tx.Get(enrollmentCredentialsBucket, key)
	if err != nil {
		return err
	}
	if found && !bytes.Equal(existing, raw) {
		return fmt.Errorf("conflicting enrollment Credential")
	}
	if found {
		return nil
	}
	return tx.Put(enrollmentCredentialsBucket, key, raw)
}

func loadEnrollmentCredential(raw []byte, now time.Time) (*Artifact, error) {
	credential, err := loadEnrollmentCredentialIntegrity(raw)
	if err != nil {
		return nil, err
	}
	if enrollmentCredentialTemporalState(credential, now) != recoveryCredentialActive {
		return nil, fmt.Errorf("enrollment Credential is not temporally eligible")
	}
	return credential, nil
}

func loadEnrollmentCredentialIntegrity(raw []byte) (*Artifact, error) {
	credential, err := parseAndVerifyKeyCredentialIntegrity(raw)
	if err != nil {
		return nil, fmt.Errorf("enrollment Credential record is corrupt")
	}
	payload := credential.KeyCredentialPayload()
	if payload == nil || len(payload.RootPublicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("enrollment Credential record is corrupt")
	}
	return credential, nil
}

type recoveryCredentialState uint8

const (
	recoveryCredentialUnknown recoveryCredentialState = iota
	recoveryCredentialActive
	recoveryCredentialNotYetValid
	recoveryCredentialExpired
	recoveryCredentialRevoked
)

func enrollmentCredentialTemporalState(credential *Artifact, now time.Time) recoveryCredentialState {
	switch credentialTemporalEligibilityAt(credential.KeyCredentialPayload(), now) {
	case temporalNotYetValid:
		return recoveryCredentialNotYetValid
	case temporalExpired:
		return recoveryCredentialExpired
	default:
		return recoveryCredentialActive
	}
}
