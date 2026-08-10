package siteexperiment

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

func verifyNameRecord(data []byte, namePublic ed25519.PublicKey, runID, networkID string, nonce []byte, now time.Time) (fixtureRecord, error) {
	record, err := decodeRecord(data)
	if err != nil {
		return fixtureRecord{}, err
	}
	if err := verifyRecordSignature(record, namePublic); err != nil {
		return fixtureRecord{}, err
	}
	if record.Schema != fixtureSchema || record.Type != "name" || record.RunID != runID || record.NetworkID != networkID ||
		record.Nonce != hex.EncodeToString(nonce) || record.DeadlineUnix <= now.Unix() || record.DeadlineUnix > now.Add(15*time.Second).Unix() ||
		record.Name != "site.reference" || record.Target == "" || record.NameGeneration != 1 || record.NameRevision != 1 {
		return fixtureRecord{}, errors.New("name Record binding or freshness is invalid")
	}
	return record, nil
}

func verifyDescriptor(data []byte, servicePublic ed25519.PublicKey, runID, networkID string, nonce []byte, target string, generation uint64, now time.Time) (fixtureRecord, error) {
	record, err := decodeRecord(data)
	if err != nil {
		return fixtureRecord{}, err
	}
	if record.Schema != fixtureSchema || record.Type != "descriptor" || record.RunID != runID || record.NetworkID != networkID ||
		record.Nonce != hex.EncodeToString(nonce) || record.DeadlineUnix <= now.Unix() || record.DeadlineUnix > now.Add(15*time.Second).Unix() ||
		record.Target != target || record.InstanceGeneration != generation || record.Endpoint == "" || record.Credential == nil {
		return fixtureRecord{}, errors.New("descriptor binding or freshness is invalid")
	}
	instancePublic, err := hex.DecodeString(record.InstancePublicKey)
	if err != nil || len(instancePublic) != ed25519.PublicKeySize {
		return fixtureRecord{}, errors.New("descriptor Instance public key is invalid")
	}
	if err := verifyRecordSignature(record, ed25519.PublicKey(instancePublic)); err != nil {
		return fixtureRecord{}, err
	}
	credential := *record.Credential
	if err := verifyCredential(credential, servicePublic, runID, networkID, target, record.InstancePublicKey, generation, now); err != nil {
		return fixtureRecord{}, err
	}
	return record, nil
}

func verifyRecordSignature(record fixtureRecord, publicKey ed25519.PublicKey) error {
	signature, err := hex.DecodeString(record.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize || len(publicKey) != ed25519.PublicKeySize {
		return errors.New("fixture record signature is invalid")
	}
	record.Signature = ""
	unsigned, err := json.Marshal(record)
	if err != nil || !ed25519.Verify(publicKey, unsigned, signature) {
		return errors.New("fixture record signature is invalid")
	}
	return nil
}

func verifyCredential(credential instanceCredential, servicePublic ed25519.PublicKey, runID, networkID, target, instancePublic string, generation uint64, now time.Time) error {
	signature, err := hex.DecodeString(credential.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return errors.New("instance Credential signature is invalid")
	}
	credential.Signature = ""
	unsigned, err := json.Marshal(credential)
	if err != nil || !ed25519.Verify(servicePublic, unsigned, signature) {
		return errors.New("instance Credential signature is invalid")
	}
	if credential.Schema != fixtureSchema || credential.RunID != runID || credential.NetworkID != networkID || credential.Target != target ||
		credential.InstancePublicKey != instancePublic || credential.InstanceGeneration != generation ||
		credential.NotBeforeUnix > now.Unix() || credential.NotAfterUnix <= now.Unix() {
		return errors.New("instance Credential binding, generation, or validity is invalid")
	}
	return nil
}
