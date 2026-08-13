package servicesmoke

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
	"github.com/dianabuilds/ardents-network/internal/qualification/routesmoke"
	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

const fixtureMarker = "ardents-h3-service-smoke-fixture-v1\n"
const evidenceMarker = "ardents-h3-service-smoke-evidence-v1\n"

type prepared struct {
	network, authority, target, manifest [32]byte
	at                                   time.Time
	credentials                          [2]serviceconn.Credential
	acknowledgements                     [2][32]byte
}

func prepare(input Config) (prepared, error) {
	if input.Duration < 10*time.Minute || input.Duration > 30*time.Minute || !newAbsolute(input.FixtureRoot) ||
		!newAbsolute(input.EvidenceRoot) || !filepath.IsAbs(input.ComposeFile) || !filepath.IsAbs(input.SourceRoot) ||
		input.FixtureRoot == input.EvidenceRoot {
		return prepared{}, errors.New("new disjoint absolute roots, source, Compose file, and 10m..30m duration are required")
	}
	for root, marker := range map[string]string{input.FixtureRoot: fixtureMarker, input.EvidenceRoot: evidenceMarker} {
		if err := os.Mkdir(root, 0o700); err != nil {
			return prepared{}, err
		}
		if err := os.WriteFile(filepath.Join(root, ".ardents-owned"), []byte(marker), 0o600); err != nil {
			return prepared{}, err
		}
	}
	at := time.Now().UTC().Truncate(time.Second)
	route, err := routesmoke.PrepareStreamFixture(filepath.Join(input.FixtureRoot, "route"),
		"/run/ardents/client-route/route.sock", "/run/ardents/publisher-route/route.sock", at)
	if err != nil {
		return prepared{}, err
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return prepared{}, err
	}
	defer erase(private)
	var authority [32]byte
	copy(authority[:], public)
	value := prepared{network: route.NetworkID, authority: authority, at: at}
	for index := range 2 {
		instancePublic, instancePrivate, keyErr := ed25519.GenerateKey(rand.Reader)
		if keyErr != nil {
			return prepared{}, keyErr
		}
		var instance [32]byte
		copy(instance[:], instancePublic)
		credential, issueErr := serviceconn.IssueCredential(private, serviceconn.Credential{InstancePublic: instance,
			Generation: uint64(index + 1), NotBefore: at.Add(-time.Minute).Unix(), NotAfter: at.Add(60 * time.Minute).Unix(),
			NetworkID: route.NetworkID, Capabilities: 3})
		if issueErr != nil {
			erase(instancePrivate)
			return prepared{}, issueErr
		}
		value.credentials[index] = credential
		if index == 0 {
			value.target = credential.Target
		}
		acknowledgement, writeErr := writeGeneration(input.FixtureRoot, index+1, at, credential, instancePrivate, authority)
		if writeErr != nil {
			erase(instancePrivate)
			return prepared{}, writeErr
		}
		value.acknowledgements[index] = acknowledgement
		erase(instancePrivate)
	}
	commitment := make([]byte, 0, 32*5)
	for _, field := range [][32]byte{route.ManifestDigest, value.network, value.authority, value.target,
		sha256.Sum256(append(credentialBytes(value.credentials[0]), credentialBytes(value.credentials[1])...))} {
		commitment = append(commitment, field[:]...)
	}
	value.manifest = sha256.Sum256(commitment)
	if err := byteio.WriteJSON(filepath.Join(input.FixtureRoot, "manifest.json"), map[string]any{
		"schema": "ardents-h3-service-fixture-v1", "created_at": at, "network_id": hex32(value.network),
		"target": hex32(value.target), "authority_public": hex32(value.authority), "manifest_digest": hex32(value.manifest)}, 64<<10); err != nil {
		return prepared{}, err
	}
	return value, nil
}

func newAbsolute(path string) bool {
	if path == "" || !filepath.IsAbs(path) {
		return false
	}
	_, err := os.Lstat(path)
	return errors.Is(err, os.ErrNotExist)
}

func hex32(value [32]byte) string { return hex.EncodeToString(value[:]) }
func random32() ([32]byte, error) {
	var value [32]byte
	_, err := rand.Read(value[:])
	return value, err
}
func erase(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
