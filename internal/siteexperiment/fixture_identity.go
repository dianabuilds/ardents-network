package siteexperiment

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"time"
)

const fixtureSchema = "gatec-fixture/v1"

type authorityFixture struct {
	runID, networkID   string
	target             string
	namePublic         ed25519.PublicKey
	namePrivate        ed25519.PrivateKey
	servicePublic      ed25519.PublicKey
	servicePrivate     ed25519.PrivateKey
	instancePublic     ed25519.PublicKey
	instancePrivate    ed25519.PrivateKey
	instanceGeneration uint64
	credential         instanceCredential
	rootCertificate    *x509.Certificate
	rootPEM            []byte
	instanceChainPEM   []byte
	instanceKeyPEM     []byte
}

type instanceCredential struct {
	Schema             string `json:"schema"`
	RunID              string `json:"run_id"`
	NetworkID          string `json:"network_id"`
	Target             string `json:"target"`
	InstancePublicKey  string `json:"instance_public_key"`
	InstanceGeneration uint64 `json:"instance_generation"`
	NotBeforeUnix      int64  `json:"not_before_unix"`
	NotAfterUnix       int64  `json:"not_after_unix"`
	Signature          string `json:"signature"`
}

type fixtureRecord struct {
	Schema             string              `json:"schema"`
	Type               string              `json:"type"`
	RunID              string              `json:"run_id"`
	NetworkID          string              `json:"network_id"`
	Nonce              string              `json:"nonce"`
	DeadlineUnix       int64               `json:"deadline_unix"`
	Name               string              `json:"name,omitempty"`
	Target             string              `json:"target"`
	NameGeneration     uint64              `json:"name_generation,omitempty"`
	NameRevision       uint64              `json:"name_revision,omitempty"`
	InstancePublicKey  string              `json:"instance_public_key,omitempty"`
	InstanceGeneration uint64              `json:"instance_generation,omitempty"`
	Endpoint           string              `json:"endpoint,omitempty"`
	Credential         *instanceCredential `json:"credential,omitempty"`
	Signature          string              `json:"signature"`
}

func newAuthorityFixture(runID, networkID string, now time.Time, random io.Reader) (*authorityFixture, error) {
	if runID == "" || networkID == "" || random == nil {
		return nil, errors.New("fixture identities and randomness are required")
	}
	namePublic, namePrivate, err := ed25519.GenerateKey(random)
	if err != nil {
		return nil, err
	}
	servicePublic, servicePrivate, err := ed25519.GenerateKey(random)
	if err != nil {
		return nil, err
	}
	targetDigest := sha256.Sum256(servicePublic)
	fixture := &authorityFixture{
		runID: runID, networkID: networkID, target: "target:sha256:" + hex.EncodeToString(targetDigest[:]),
		namePublic: namePublic, namePrivate: namePrivate, servicePublic: servicePublic, servicePrivate: servicePrivate,
	}
	rootSerial, err := randomFixtureSerial(random)
	if err != nil {
		return nil, err
	}
	rootTemplate := &x509.Certificate{
		SerialNumber: rootSerial, Subject: pkix.Name{CommonName: "Gate C Service Target"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	rootDER, err := x509.CreateCertificate(random, rootTemplate, rootTemplate, servicePublic, servicePrivate)
	if err != nil {
		return nil, err
	}
	fixture.rootCertificate, err = x509.ParseCertificate(rootDER)
	if err != nil {
		return nil, err
	}
	fixture.rootPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER})
	if err := fixture.replaceInstance(now, random); err != nil {
		return nil, err
	}
	return fixture, nil
}

func (fixture *authorityFixture) migrate(now time.Time, random io.Reader) error {
	return fixture.replaceInstance(now, random)
}

func (fixture *authorityFixture) replaceInstance(now time.Time, random io.Reader) error {
	publicKey, privateKey, err := ed25519.GenerateKey(random)
	if err != nil {
		return err
	}
	fixture.instanceGeneration++
	fixture.instancePublic, fixture.instancePrivate = publicKey, privateKey
	leafSerial, err := randomFixtureSerial(random)
	if err != nil {
		return err
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: leafSerial, Subject: pkix.Name{CommonName: "Gate C active Instance"},
		DNSNames: []string{"carrier.invalid"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(5 * time.Minute),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(random, leafTemplate, fixture.rootCertificate, publicKey, fixture.servicePrivate)
	if err != nil {
		return err
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return err
	}
	fixture.instanceChainPEM = append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}), fixture.rootPEM...)
	fixture.instanceKeyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})
	credential := instanceCredential{
		Schema: fixtureSchema, RunID: fixture.runID, NetworkID: fixture.networkID, Target: fixture.target,
		InstancePublicKey: hex.EncodeToString(publicKey), InstanceGeneration: fixture.instanceGeneration,
		NotBeforeUnix: now.Add(-time.Second).Unix(), NotAfterUnix: now.Add(5 * time.Minute).Unix(),
	}
	signature, err := signCredential(credential, fixture.servicePrivate)
	if err != nil {
		return err
	}
	credential.Signature = signature
	fixture.credential = credential
	return nil
}

func (fixture *authorityFixture) routeIdentity() (root, chain, key []byte) {
	return bytes.Clone(fixture.rootPEM), bytes.Clone(fixture.instanceChainPEM), bytes.Clone(fixture.instanceKeyPEM)
}

func randomFixtureSerial(random io.Reader) (*big.Int, error) {
	value := make([]byte, 16)
	if _, err := io.ReadFull(random, value); err != nil {
		return nil, err
	}
	value[0] &= 0x7f
	serial := new(big.Int).SetBytes(value)
	if serial.Sign() == 0 {
		return big.NewInt(1), nil
	}
	return serial, nil
}

func (fixture *authorityFixture) signedNameRecord(nonce []byte, deadline time.Time) ([]byte, error) {
	record := fixtureRecord{
		Schema: fixtureSchema, Type: "name", RunID: fixture.runID, NetworkID: fixture.networkID,
		Nonce: hex.EncodeToString(nonce), DeadlineUnix: deadline.Unix(), Name: "site.reference", Target: fixture.target,
		NameGeneration: 1, NameRevision: 1,
	}
	return signRecord(record, fixture.namePrivate)
}

func (fixture *authorityFixture) signedDescriptor(nonce []byte, deadline time.Time) ([]byte, error) {
	credential := fixture.credential
	record := fixtureRecord{
		Schema: fixtureSchema, Type: "descriptor", RunID: fixture.runID, NetworkID: fixture.networkID,
		Nonce: hex.EncodeToString(nonce), DeadlineUnix: deadline.Unix(), Target: fixture.target,
		InstancePublicKey: hex.EncodeToString(fixture.instancePublic), InstanceGeneration: fixture.instanceGeneration,
		Endpoint: fmt.Sprintf("gatec-instance-%d", fixture.instanceGeneration), Credential: &credential,
	}
	return signRecord(record, fixture.instancePrivate)
}

func signRecord(record fixtureRecord, privateKey ed25519.PrivateKey) ([]byte, error) {
	unsigned, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	record.Signature = hex.EncodeToString(ed25519.Sign(privateKey, unsigned))
	return json.Marshal(record)
}

func signCredential(credential instanceCredential, privateKey ed25519.PrivateKey) (string, error) {
	credential.Signature = ""
	unsigned, err := json.Marshal(credential)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(ed25519.Sign(privateKey, unsigned)), nil
}

func decodeRecord(data []byte) (fixtureRecord, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record fixtureRecord
	if err := decoder.Decode(&record); err != nil {
		return fixtureRecord{}, errors.New("fixture record has invalid encoding")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fixtureRecord{}, errors.New("fixture record has trailing data")
	}
	return record, nil
}
