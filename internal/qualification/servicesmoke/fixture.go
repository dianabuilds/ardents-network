package servicesmoke

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	network, authority, introduction, target, routeManifest, manifest [32]byte
	at                                                                time.Time
	credentials                                                       [2]serviceconn.Credential
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
	introductionPublic, introductionPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return prepared{}, err
	}
	defer erase(introductionPrivate)
	var introduction [32]byte
	copy(introduction[:], introductionPublic)
	if err := configureIntroduction(filepath.Join(input.FixtureRoot, "route"), introductionPrivate); err != nil {
		return prepared{}, err
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return prepared{}, err
	}
	defer erase(private)
	var authority [32]byte
	copy(authority[:], public)
	value := prepared{network: route.NetworkID, authority: authority, introduction: introduction,
		routeManifest: route.ManifestDigest, at: at}
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
		_, writeErr := writeGeneration(input.FixtureRoot, index+1, at, credential, instancePrivate, authority, introduction)
		if writeErr != nil {
			erase(instancePrivate)
			return prepared{}, writeErr
		}
		erase(instancePrivate)
	}
	commitment := make([]byte, 0, 32*5)
	for _, field := range [][32]byte{route.ManifestDigest, value.network, value.authority, value.introduction, value.target,
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

func configureIntroduction(routeRoot string, private ed25519.PrivateKey) error {
	planPath := filepath.Join(routeRoot, "plans", "introduction.json")
	raw, err := byteio.ReadFile(planPath, 64<<10)
	if err != nil {
		return err
	}
	var plan map[string]any
	if err := json.Unmarshal(raw, &plan); err != nil {
		return err
	}
	plan["AcknowledgementSocket"] = "/run/ardents/introduction-ack/ack.sock"
	plan["AcknowledgementKey"] = "/run/ardents/secrets/ack.hex"
	if err := byteio.WriteJSON(planPath, plan, 64<<10); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(routeRoot, "secrets", "introduction", "ack.hex"),
		[]byte(hex.EncodeToString(private)), 0o600)
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
