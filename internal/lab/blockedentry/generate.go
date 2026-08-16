package blockedentry

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"time"
)

var boundaries = []string{
	"endpoint-adapter", "tls-front", "webtunnel-server", "bridge-next-leg", "publisher-endpoint",
	"ordinary-initiator", "ordinary-introduction", "ordinary-rendezvous", "ordinary-responder",
}
var residualKinds = []string{"process", "listener", "socket", "namespace", "mount", "file", "queue", "timer", "cgroup", "publishable-secret"}
var topology = []topologyRole{
	{"endpoint-application", "application", "app-e", "none"},
	{"endpoint", "endpoint", "endpoint-e", "entry"},
	{"bridge-adapter", "adapter", "adapter-e", "entry"},
	{"bridge-front", "front", "front-b", "bridge"},
	{"bridge-server", "server", "server-b", "bridge-route"},
	{"initiator", "initiator", "route-i", "route"},
	{"introduction", "introduction", "route-x", "route"},
	{"rendezvous", "rendezvous", "route-v", "route"},
	{"responder", "responder", "route-r", "route"},
	{"publisher", "publisher", "endpoint-p", "route"},
	{"publisher-application", "application", "app-p", "none"},
}

func buildManifest(config Config, canaryHash, nonceHash string,
	artifacts, supplemental []artifactCommitment,
) (manifest, error) {
	harnessHash, _, err := hashFile(configExecutable())
	if err != nil {
		return manifest{}, err
	}
	verifierHash, _, err := hashFile(config.VerifierPath)
	if err != nil {
		return manifest{}, err
	}
	runnerHash, _, err := hashFile(config.RunnerPath)
	if err != nil {
		return manifest{}, err
	}
	clientHash, _, err := hashFile(config.ClientPath)
	if err != nil {
		return manifest{}, err
	}
	serverHash, _, err := hashFile(config.ServerPath)
	if err != nil {
		return manifest{}, err
	}
	evidenceRoot, err := filepath.Abs(config.EvidenceRoot)
	if err != nil {
		return manifest{}, err
	}
	registryRoot, err := filepath.Abs(config.RegistryRoot)
	if err != nil {
		return manifest{}, err
	}
	rootHash := sha256.Sum256([]byte(filepath.Clean(evidenceRoot)))
	registryHash := sha256.Sum256([]byte(filepath.Clean(registryRoot)))
	groups := make([]manifestGroup, 0, 9)
	for _, group := range hostileMatrix() {
		groups = append(groups, manifestGroup{ID: group.ID, Variants: group.Variants, Episodes: 5})
	}
	return manifest{Schema: "ardents-h3-blocked-entry-manifest-v1", CampaignKind: "development-fixture",
		Profile: developmentFixtureProfile, RunID: config.RunID, FixtureMode: config.Mode,
		SourceIdentity: "development-fixture:" + harnessHash + ":" + runnerHash,
		SupplyClass:    "unrestricted-schema-fixture",
		HarnessSHA256:  harnessHash, RunnerSHA256: runnerHash, VerifierSHA256: verifierHash,
		ClientSHA256: clientHash, ServerSHA256: serverHash,
		CanarySHA256: canaryHash, Groups: groups, Topology: topology, Boundaries: boundaries, ResidualKinds: residualKinds,
		SecretArtifacts: artifacts, SupplementalArtifacts: supplemental,
		CreatedUnixNano: time.Now().UnixNano(), ManifestNonceHash: nonceHash,
		EvidenceRootHash: hex.EncodeToString(rootHash[:]), RegistryRootHash: hex.EncodeToString(registryHash[:]),
		AttributionSources: attributionSources}, nil
}

func createNonceHash() (string, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	value := sha256.Sum256(nonce)
	return hex.EncodeToString(value[:]), nil
}

func configExecutable() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}
	return path
}

func buildEvidence(config Config, manifestHash string, artifacts, supplemental []artifactCommitment,
	canaries canaryCorpus,
) (evidence, []byte, error) {
	mode := config.Mode
	if mode == "" {
		mode = "pass"
		config.Mode = mode
	}
	allowed := map[string]bool{"pass": true, "candidate-fail": true, "harness-invalid": true,
		"candidate-canary": true, "pipeline-canary": true, "candidate-residual": true, "inventory-missing": true}
	allowed["candidate-forbidden"] = true
	allowed["cell-inventory-missing"] = true
	allowed["collector-loss"] = true
	allowed["blocker-loss"] = true
	allowed["forbidden-owner-mismatch"] = true
	allowed["candidate-fail-harness-invalid"] = true
	for _, field := range []string{"invite", "address", "path", "certificate"} {
		allowed["candidate-canary-"+field] = true
		allowed["pipeline-canary-"+field] = true
	}
	if !allowed[mode] {
		return evidence{}, nil, errors.New("blocked-entry harness mode is unsupported")
	}
	events, observers, cleanup, err := collectEvents(config, canaries)
	if err != nil {
		return evidence{}, nil, err
	}
	attributions, err := attributionCommitments(config.EvidenceRoot+".partial/secret", events)
	if err != nil {
		return evidence{}, nil, err
	}
	result := evidence{Schema: "ardents-h3-blocked-entry-evidence-v1", CampaignKind: "development-fixture",
		Profile: developmentFixtureProfile,
		RunID:   config.RunID, ManifestSHA256: manifestHash, Events: events, Observers: observers, Cleanup: cleanup,
		SecretArtifacts: artifacts, SupplementalArtifacts: supplemental,
		AttributionArtifacts: attributions, CollectionClosed: true}
	return injectMode(result, mode, canaries)
}

func expectedTerminal(group, variant string) string {
	switch group {
	case "G1-invite":
		if variantIn(variant, "wrong-network", "wrong-epoch", "wrong-profile", "not-yet-valid",
			"insufficient-time-confidence") {
			return "incompatible"
		}
		if variant == "expired" {
			return "expired"
		}
		return "invalid"
	case "G2-domain-collision":
		if variantIn(variant, "responder", "introduction", "rendezvous", "resolution", "unknown-domain") {
			return "wrong-domain"
		}
		return "conflicting-role"
	case "G3-replay-replacement":
		switch variant {
		case "active-reimport":
			return "already-present"
		case "retired-replay", "same-generation-different-bytes":
			return "replay"
		case "full-set":
			return "set-full"
		default:
			return "replacement-rejected"
		}
	case "G4-restart":
		return "bridge-interrupted"
	case "G5-adapter-fault":
		if variant == "evidence-write-exhaustion" {
			return "bridge-local-denial"
		}
		return "bridge-attempt-exhausted"
	case "G6-substitution":
		if variantIn(variant, "network", "route-profile") {
			return "incompatible"
		}
		return "bridge-local-denial"
	case "G7-forbidden-path":
		if variant == "deadline-exposure-reset" {
			return "bridge-deadline-exceeded"
		}
		return "bridge-attempt-exhausted"
	case "G8-lifecycle":
		switch variant {
		case "collector-loss", "blocker-loss":
			return ""
		case "cancellation":
			return "bridge-deadline-exceeded"
		case "expiry-revocation", "clock-discontinuity":
			return "bridge-ineligible"
		case "endpoint-restart", "bridge-restart":
			return "bridge-interrupted"
		default:
			return "bridge-local-denial"
		}
	case "G9-ledger-leakage":
		if variant == "unknown-invite-field" {
			return "invalid"
		}
		if variantIn(variant, "pipeline-contamination-invite", "pipeline-contamination-address",
			"pipeline-contamination-path", "pipeline-contamination-certificate") {
			return ""
		}
		return "bridge-local-denial"
	default:
		return ""
	}
}

func variantIn(value string, wanted ...string) bool {
	for _, candidate := range wanted {
		if value == candidate {
			return true
		}
	}
	return false
}

func secretArtifacts(secretRoot string, config Config) ([]artifactCommitment, error) {
	generated := []string{"candidate/client.stderr", "candidate/server.stderr", "capture/packets.bin"}
	for _, path := range generated {
		absolute := filepath.Join(secretRoot, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
			return nil, err
		}
		if err := os.WriteFile(absolute, []byte("secret-only fixture\n"), 0o600); err != nil {
			return nil, err
		}
	}
	paths := []string{"canaries.json", generated[0], generated[1], generated[2],
		filepath.ToSlash(filepath.Join("supply", filepath.Base(config.RunnerPath))),
		filepath.ToSlash(filepath.Join("supply", filepath.Base(config.ClientPath))),
		filepath.ToSlash(filepath.Join("supply", filepath.Base(config.ServerPath)))}
	artifacts := make([]artifactCommitment, 0, len(paths))
	for _, path := range paths {
		value, err := commitment(secretRoot, path)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, value)
	}
	return artifacts, nil
}
