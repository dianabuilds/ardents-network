//go:build ignore

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// SourceAddress is the literal "host:port" a source server binds inside the
// Docker network. The fixed ports 4101 and 4102 match the rendezvous
// qualification convention: unprivileged, never reused. The host half is a
// literal IP because the ardents-node source and ardents refresh-sources
// consumers both reject hostnames and Docker DNS names with
// "source address must be a literal IP and port"; the compose file wires
// the source-a/b containers to a custom bridge network with static IPs
// 172.30.0.10 / 172.30.0.11, and the compose env vars override these defaults
// for any deployment that wants different addresses.
const (
	DefaultSourceAddressA = "172.30.0.10:4101"
	DefaultSourceAddressB = "172.30.0.11:4102"
	SourceListenA         = "0.0.0.0:4101"
	SourceListenB         = "0.0.0.0:4102"
)

// ResolveSourceAddress returns the address from the named environment
// variable, falling back to the default if the variable is unset or empty.
func ResolveSourceAddress(envName, fallback string) string {
	if value, ok := os.LookupEnv(envName); ok && value != "" {
		return value
	}
	return fallback
}

// WriteCerts writes one source CA plus two source-server leaves (one for
// source-a, one for source-b) plus a separate client CA and one source-
// client leaf, all ed25519, in PEM form. The returned pins are:
//   - clientPin     sha256(ardents-h3-source-transport-key-v1\x00 || client-public)
//     — the source-server plans pin this in their client_key_digests.
//   - sourceAPin    sha256(ardents-h3-source-transport-key-v1\x00 || source-a-public)
//     — the source-client plan pins this as source-a's leaf_key_digest.
//   - sourceBPin    sha256(ardents-h3-source-transport-key-v1\x00 || source-b-public)
//     — the source-client plan pins this as source-b's leaf_key_digest.
//
// The client, source, and Epoch signer keys must all be separate per the
// production source client validation; sharing one pin across the three
// roles triggers "client, source, and Epoch signer keys must be separate"
// from internal/network/source/credentials.go.
func WriteCerts(sourceCA, sourceACert, sourceAKey, sourceBCert, sourceBKey, clientCA, clientCert, clientKey string,
	now time.Time) (clientPin, sourceAPin, sourceBPin [32]byte, err error) {
	sourceAuthorityPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x51}, ed25519.SeedSize))
	if err := writeSelfSignedCert(sourceAuthorityPrivate, "pilot-source-ca", true, now, sourceCA, 0x51); err != nil {
		return [32]byte{}, [32]byte{}, [32]byte{}, fmt.Errorf("pilot: write source CA: %w", err)
	}
	clientAuthorityPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x61}, ed25519.SeedSize))
	if err := writeSelfSignedCert(clientAuthorityPrivate, "pilot-client-ca", true, now, clientCA, 0x61); err != nil {
		return [32]byte{}, [32]byte{}, [32]byte{}, fmt.Errorf("pilot: write client CA: %w", err)
	}
	sourceAPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x71}, ed25519.SeedSize))
	if err := writeLeafCert(sourceAPrivate, sourceAuthorityPrivate, sourceCA, "pilot-source-a.test", true, now, sourceACert, sourceAKey, 0x71); err != nil {
		return [32]byte{}, [32]byte{}, [32]byte{}, fmt.Errorf("pilot: write source-a server cert: %w", err)
	}
	sourceBPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x72}, ed25519.SeedSize))
	if err := writeLeafCert(sourceBPrivate, sourceAuthorityPrivate, sourceCA, "pilot-source-b.test", true, now, sourceBCert, sourceBKey, 0x72); err != nil {
		return [32]byte{}, [32]byte{}, [32]byte{}, fmt.Errorf("pilot: write source-b server cert: %w", err)
	}
	clientPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x81}, ed25519.SeedSize))
	if err := writeLeafCert(clientPrivate, clientAuthorityPrivate, clientCA, "pilot-source-client.test", false, now, clientCert, clientKey, 0x81); err != nil {
		return [32]byte{}, [32]byte{}, [32]byte{}, fmt.Errorf("pilot: write source client cert: %w", err)
	}
	prefix := []byte("ardents-h3-source-transport-key-v1\x00")
	clientPin = sha256.Sum256(append(prefix, clientPrivate.Public().(ed25519.PublicKey)...))
	sourceAPin = sha256.Sum256(append(prefix, sourceAPrivate.Public().(ed25519.PublicKey)...))
	sourceBPin = sha256.Sum256(append(prefix, sourceBPrivate.Public().(ed25519.PublicKey)...))
	return clientPin, sourceAPin, sourceBPin, nil
}

func writeSelfSignedCert(private ed25519.PrivateKey, name string, isCA bool, now time.Time, path string, serial int64) error {
	serialNumber := big.NewInt(serial)
	template := &x509.Certificate{SerialNumber: serialNumber, Subject: pkix.Name{CommonName: name},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(2 * time.Hour)}
	if isCA {
		template.IsCA = true
		template.BasicConstraintsValid = true
		template.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	}
	raw, err := x509.CreateCertificate(nil, template, template, private.Public(), private)
	if err != nil {
		return err
	}
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw}), 0o600)
}

func writeLeafCert(private ed25519.PrivateKey, authority ed25519.PrivateKey, authorityCertPath, name string, server bool,
	now time.Time, certPath, keyPath string, serial int64) error {
	authorityCert, err := readCert(authorityCertPath)
	if err != nil {
		return err
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name},
		DNSNames: []string{name}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(2 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature}
	if server {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	} else {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}
	raw, err := x509.CreateCertificate(nil, template, authorityCert, private.Public(), authority)
	if err != nil {
		return err
	}
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw}), 0o600); err != nil {
		return err
	}
	key, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return err
	}
	return os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0o600)
}

func readCert(path string) (*x509.Certificate, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("pilot: cert PEM is empty")
	}
	return x509.ParseCertificate(block.Bytes)
}

// SourceServerPlan is the on-disk JSON shape that
// "ardents-node source --config <path>" parses. It mirrors the Go struct
// defined in cmd/ardents-node/config.go and is deliberately written as a
// separate type here so the pilot does not import a command-internal type.
type SourceServerPlan struct {
	Schema               string   `json:"schema"`
	StateRoot            string   `json:"state_root"`
	LocalRoleStateRoot   string   `json:"local_role_state_root"`
	NetworkID            string   `json:"network_id"`
	AuthorityPublic      []string `json:"authority_public"`
	Threshold            int      `json:"threshold"`
	At                   string   `json:"at"`
	Listen               string   `json:"listen"`
	ServerCertificate    string   `json:"server_certificate"`
	ServerKey            string   `json:"server_key"`
	ClientRoot           string   `json:"client_root"`
	ClientKeyDigests     []string `json:"client_key_digests"`
	MaterializationIndex uint32   `json:"materialization_index"`
}

// SourceClientPlan is the on-disk JSON shape that
// "ardents refresh-sources --source-plan <path>" parses. All six consumer
// nodes share the same plan; only their --state-root differs.
type SourceClientPlan struct {
	Schema               string             `json:"schema"`
	NetworkID            string             `json:"network_id"`
	AuthorityPublic      []string           `json:"authority_public"`
	Threshold            int                `json:"threshold"`
	ClockObservedAt      string             `json:"clock_observed_at"`
	ClockObservationFile string             `json:"clock_observation_file"`
	OrderSeed            string             `json:"order_seed"`
	MaterializationIndex uint32             `json:"materialization_index"`
	LocalRoleStateRoot   string             `json:"local_role_state_root"`
	ClientCertificate    string             `json:"client_certificate"`
	ClientKey            string             `json:"client_key"`
	Sources              []SourceClientSpec `json:"sources"`
}

type SourceClientSpec struct {
	Address        string `json:"address"`
	ServerName     string `json:"server_name"`
	Identity       string `json:"identity"`
	Family         string `json:"family"`
	EndpointHandle string `json:"endpoint_handle"`
	RootCA         string `json:"root_ca"`
	LeafKeyDigest  string `json:"leaf_key_digest"`
}

// WritePlans emits the two source-server plans and the one shared source-
// client plan into fixturesDir. The four pins are:
//   - clientPin    the source-client leaf cert digest, pinned by both
//     source servers as a permitted client key.
//   - sourceAPin   the source-a server leaf cert digest, pinned by the
//     source-client plan as source-a's leaf_key_digest.
//   - sourceBPin   the source-b server leaf cert digest, pinned by the
//     source-client plan as source-b's leaf_key_digest.
//
// sourceAState and sourceBState are the per-source state roots the prebake
// step populates via ardents accept-offline; the plans reference them by
// absolute path so source-a and source-b containers can open them as
// their owned state root. sourceAAddress and sourceBAddress are the
// literal IP:port the consumer uses to reach each source; both must be
// literal addresses (not hostnames), and the production source and
// consumer paths reject anything else.
func WritePlans(fixturesDir, sourceAState, sourceBState, sourceAAddress, sourceBAddress string,
	fixtures Fixtures, clientPin, sourceAPin, sourceBPin [32]byte, now time.Time) error {
	authorityHex := fmt.Sprintf("%x", fixtures.AuthorityPublic)
	sourceRoot := filepath.Join(fixturesDir, "source-ca.pem")
	clientRoot := filepath.Join(fixturesDir, "client-ca.pem")
	clientCert := filepath.Join(fixturesDir, "client.pem")
	clientKey := filepath.Join(fixturesDir, "client-key.pem")
	orderSeedDigest := sha256.Sum256([]byte("pilot-order-seed-1"))
	orderSeed := fmt.Sprintf("%x", orderSeedDigest)
	at := now.UTC().Format(time.RFC3339)
	identityADigest := sha256.Sum256([]byte("pilot-source-a-identity"))
	identityA := fmt.Sprintf("%x", identityADigest)
	identityBDigest := sha256.Sum256([]byte("pilot-source-b-identity"))
	identityB := fmt.Sprintf("%x", identityBDigest)

	if err := writeJSON(filepath.Join(fixturesDir, "source-a.json"), SourceServerPlan{
		Schema: "ardents-source-server-v1", StateRoot: sourceAState,
		LocalRoleStateRoot: filepath.Join(sourceAState, "local-roles"), NetworkID: fixtures.NetworkIDHex,
		AuthorityPublic: []string{authorityHex}, Threshold: 1, At: at, Listen: SourceListenA,
		ServerCertificate: filepath.Join(fixturesDir, "source-a.pem"),
		ServerKey:         filepath.Join(fixturesDir, "source-a-key.pem"),
		ClientRoot:        clientRoot, ClientKeyDigests: []string{fmt.Sprintf("%x", clientPin)},
		MaterializationIndex: 0,
	}); err != nil {
		return fmt.Errorf("pilot: write source-a plan: %w", err)
	}
	if err := writeJSON(filepath.Join(fixturesDir, "source-b.json"), SourceServerPlan{
		Schema: "ardents-source-server-v1", StateRoot: sourceBState,
		LocalRoleStateRoot: filepath.Join(sourceBState, "local-roles"), NetworkID: fixtures.NetworkIDHex,
		AuthorityPublic: []string{authorityHex}, Threshold: 1, At: at, Listen: SourceListenB,
		ServerCertificate: filepath.Join(fixturesDir, "source-b.pem"),
		ServerKey:         filepath.Join(fixturesDir, "source-b-key.pem"),
		ClientRoot:        clientRoot, ClientKeyDigests: []string{fmt.Sprintf("%x", clientPin)},
		MaterializationIndex: 0,
	}); err != nil {
		return fmt.Errorf("pilot: write source-b plan: %w", err)
	}
	if err := writeJSON(filepath.Join(fixturesDir, "client.json"), SourceClientPlan{
		Schema: "ardents-source-plan-v1", NetworkID: fixtures.NetworkIDHex,
		AuthorityPublic: []string{authorityHex}, Threshold: 1, ClockObservedAt: at,
		ClockObservationFile: filepath.Join(fixturesDir, "clock.observation"),
		OrderSeed:            orderSeed, MaterializationIndex: 0,
		LocalRoleStateRoot: filepath.Join(fixturesDir, "consumer-local-roles"),
		ClientCertificate:  clientCert, ClientKey: clientKey,
		Sources: []SourceClientSpec{
			{Address: sourceAAddress, ServerName: "pilot-source-a.test", Identity: identityA,
				Family: "pilot-source-a-family", EndpointHandle: "pilot-source-a-handle",
				RootCA: sourceRoot, LeafKeyDigest: fmt.Sprintf("%x", sourceAPin)},
			{Address: sourceBAddress, ServerName: "pilot-source-b.test", Identity: identityB,
				Family: "pilot-source-b-family", EndpointHandle: "pilot-source-b-handle",
				RootCA: sourceRoot, LeafKeyDigest: fmt.Sprintf("%x", sourceBPin)},
		},
	}); err != nil {
		return fmt.Errorf("pilot: write client plan: %w", err)
	}
	return nil
}

func writeJSON(path string, value any) error {
	marshaled, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(marshaled, '\n'), 0o600)
}
