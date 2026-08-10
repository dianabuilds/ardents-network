package nativecircuit

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

var nativeNodeRoles = []string{
	"user-entry", "user-interior", "rendezvous", "service-interior", "data-service-entry",
	"introduction-forwarder", "introduction-node", "introduction-interior", "introduction-entry",
}

var nativeApplicationRoles = append([]string{"user", "service"}, nativeNodeRoles...)

type nativeFixture struct {
	root               string
	roleEvidence       map[string]string
	toolEvidence       map[string]string
	captureDirectory   string
	controlDirectory   string
	forbiddenSentinels [][]byte
}

func prepareNativeFixture(runDirectory, runID, fault string) (nativeFixture, error) {
	root := filepath.Join(runDirectory, "native")
	fixture := nativeFixture{
		root: root, roleEvidence: make(map[string]string), toolEvidence: make(map[string]string),
		captureDirectory: filepath.Join(root, "raw-capture"), controlDirectory: filepath.Join(root, "control"),
	}
	for _, directory := range []string{root, filepath.Join(root, "configs"), filepath.Join(root, "evidence"), filepath.Join(root, "tool-configs"), filepath.Join(root, "tool-evidence"), fixture.captureDirectory, fixture.controlDirectory} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			return nativeFixture{}, err
		}
	}
	for _, directory := range []string{filepath.Join(root, "evidence"), filepath.Join(root, "tool-evidence"), fixture.captureDirectory, fixture.controlDirectory} {
		if err := os.Chmod(directory, 0o777); err != nil {
			return nativeFixture{}, err
		}
	}
	hops := make(map[string]roleHop, len(nativeNodeRoles))
	for _, role := range nativeNodeRoles {
		directory, evidence, err := prepareRoleDirectories(root, role)
		if err != nil {
			return nativeFixture{}, err
		}
		fixture.roleEvidence[role] = evidence
		hops[role], err = generateNodeIdentity(directory, role)
		if err != nil {
			return nativeFixture{}, err
		}
	}
	for _, role := range []string{"user", "service"} {
		_, evidence, err := prepareRoleDirectories(root, role)
		if err != nil {
			return nativeFixture{}, err
		}
		fixture.roleEvidence[role] = evidence
	}
	endpoint, err := generateEndpointFixture()
	if err != nil {
		return nativeFixture{}, err
	}
	hpkePrivate, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nativeFixture{}, err
	}
	slot, err := randomCryptoHandle()
	if err != nil {
		return nativeFixture{}, err
	}
	seedBytes := make([]byte, 32)
	if _, err := rand.Read(seedBytes); err != nil {
		return nativeFixture{}, err
	}
	payloadSeed := hex.EncodeToString(seedBytes)
	payload := seededPayload(payloadSeed, 64*1024)
	fixture.forbiddenSentinels = [][]byte{endpoint.targetMarker, payload[:32]}
	if err := writeEndpointInputs(root, endpoint, hpkePrivate); err != nil {
		return nativeFixture{}, err
	}
	configs := fixedNativeRoleConfigs(runID, hops, slot, endpoint.leafSHA256, payloadSeed, fault)
	for role, config := range configs {
		if err := writeFixtureJSON(filepath.Join(root, "configs", role, "role.json"), config); err != nil {
			return nativeFixture{}, err
		}
	}
	if err := prepareNativeToolConfigs(&fixture, runID); err != nil {
		return nativeFixture{}, err
	}
	return fixture, nil
}

func prepareRoleDirectories(root, role string) (string, string, error) {
	config := filepath.Join(root, "configs", role)
	evidence := filepath.Join(root, "evidence", role)
	for _, directory := range []string{config, evidence} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			return "", "", err
		}
	}
	if err := os.Chmod(evidence, 0o777); err != nil {
		return "", "", err
	}
	return config, evidence, nil
}

func writeEndpointInputs(root string, endpoint endpointFixture, hpkePrivate *ecdh.PrivateKey) error {
	inputs := map[string]map[string][]byte{
		"user":    {"target-root.pem": endpoint.rootPEM, "hpke-public.bin": hpkePrivate.PublicKey().Bytes()},
		"service": {"instance-chain.pem": endpoint.chainPEM, "instance.key": endpoint.privatePEM, "hpke-private.bin": hpkePrivate.Bytes()},
	}
	for role, files := range inputs {
		for name, data := range files {
			if err := os.WriteFile(filepath.Join(root, "configs", role, name), data, 0o600); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeFixtureJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write fixture %s: %w", filepath.Base(path), err)
	}
	return nil
}
