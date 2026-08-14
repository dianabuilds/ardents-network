package recoverysmoke

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
	"github.com/dianabuilds/ardents-network/internal/qualification/routesmoke"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

const fixtureMarker = "ardents-h3-recovery-smoke-fixture-v1\n"
const evidenceMarker = "ardents-h3-recovery-smoke-evidence-v1\n"

type prepared struct {
	network, authority, introduction, target, routeManifest, manifest [32]byte
	at                                                                time.Time
	credentials                                                       [2]serviceconn.Credential
	bindings                                                          [2][2]grantBinding
	candidates                                                        []route.Position
	publisherRoutePublic                                              [32]byte
	routeCase                                                         json.RawMessage
}

func prepare(input config) (prepared, error) {
	if input.Duration < 10*time.Minute || input.Duration > 30*time.Minute || !filepath.IsAbs(input.ComposeFile) ||
		!filepath.IsAbs(input.SourceRoot) || validateRecoveryRoots(input) != nil {
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
	if err := os.Mkdir(filepath.Join(input.FixtureRoot, "gate"), 0o777); err != nil {
		return prepared{}, err
	}
	at := time.Now().UTC().Truncate(time.Second)
	routeFixture, err := routesmoke.PrepareRecoveryStreamFixture(filepath.Join(input.FixtureRoot, "route"),
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
	routeCase, err := json.Marshal(routeFixture.RouteCase)
	if err != nil {
		return prepared{}, err
	}
	value := prepared{network: routeFixture.NetworkID, authority: authority, introduction: introduction,
		routeManifest: routeFixture.ManifestDigest, at: at,
		candidates: append([]route.Position(nil), routeFixture.Candidates...), routeCase: routeCase,
		publisherRoutePublic: routeFixture.PublisherPublic}
	for index := range 2 {
		generationRoot := filepath.Join(input.FixtureRoot, "generations", string(rune('1'+index)))
		if err := os.MkdirAll(generationRoot, 0o700); err != nil {
			return prepared{}, err
		}
		instance, keyErr := generateInstance(input.SourceRoot, filepath.Join(generationRoot, "instance.hex"))
		if keyErr != nil {
			return prepared{}, keyErr
		}
		credential, issueErr := (serviceconn.Credential{InstancePublic: instance,
			Generation: uint64(index + 1), NotBefore: at.Add(-time.Minute).Unix(), NotAfter: at.Add(60 * time.Minute).Unix(),
			NetworkID: routeFixture.NetworkID, Capabilities: 3}).Issue(private)
		if issueErr != nil {
			return prepared{}, issueErr
		}
		value.credentials[index] = credential
		if index == 0 {
			value.target = credential.Target
		}
		bindings, writeErr := writeGeneration(input.FixtureRoot, index+1, at, credential, authority, introduction)
		if writeErr != nil {
			return prepared{}, writeErr
		}
		value.bindings[index] = bindings
	}
	publicManifest := recovery.PublicManifest{RouteManifest: routeFixture.ManifestDigest, NetworkID: value.network,
		AuthorityPublic: value.authority, IntroductionPublic: value.introduction, Target: value.target,
		InstancePublic: value.credentials[0].InstancePublic, ClientPrincipal: value.bindings[0][0].Principal,
		PublisherPrincipal: value.bindings[0][1].Principal, CredentialSignature: value.credentials[0].Signature,
		CredentialGeneration: value.credentials[0].Generation, CredentialNotBefore: value.credentials[0].NotBefore,
		CredentialNotAfter: value.credentials[0].NotAfter, CredentialCapabilities: value.credentials[0].Capabilities,
		RouteProfile: "h3-route-tracer-v1", WorkSafetyNotAfter: value.credentials[0].NotAfter,
		WorkSafetyMaximum: value.credentials[0].NotAfter, NoNewRecoveryAfter: value.credentials[0].NotAfter}
	value.manifest = publicManifestDigest(publicManifest)
	if err := byteio.WriteJSON(filepath.Join(input.FixtureRoot, "manifest.json"), map[string]any{
		"schema": "ardents-h3-recovery-fixture-v1", "created_at": at, "network_id": hex32(value.network),
		"target": hex32(value.target), "authority_public": hex32(value.authority), "manifest_digest": hex32(value.manifest)}, 64<<10); err != nil {
		return prepared{}, err
	}
	if err := configureRecoveryFixture(input.FixtureRoot, value); err != nil {
		return prepared{}, err
	}
	return value, nil
}

func validateRecoveryRoots(input config) error {
	if !newAbsolute(input.FixtureRoot) || !newAbsolute(input.EvidenceRoot) {
		return errors.New("recovery roots must be new")
	}
	fixture, err := canonicalNewPath(input.FixtureRoot)
	if err != nil {
		return err
	}
	evidence, err := canonicalNewPath(input.EvidenceRoot)
	if err != nil {
		return err
	}
	source, err := filepath.EvalSymlinks(input.SourceRoot)
	if err != nil {
		return err
	}
	if overlaps(fixture, evidence) || overlaps(source, fixture) || overlaps(source, evidence) {
		return errors.New("recovery roots overlap each other or the source tree")
	}
	return nil
}

func canonicalNewPath(path string) (string, error) {
	parent, err := filepath.EvalSymlinks(filepath.Dir(filepath.Clean(path)))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(path)), nil
}

func overlaps(first, second string) bool {
	return containsPath(first, second) || containsPath(second, first)
}

func containsPath(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
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
