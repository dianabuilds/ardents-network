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

func buildManifest(config Config, finalSpecValue *finalSpec, canaryHash, nonceHash string,
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
	if err := validateFinalRunnerBinding(runnerHash, finalSpecValue); err != nil {
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
	result := manifest{Schema: "ardents-h3-blocked-entry-manifest-v1", CampaignKind: "development-fixture",
		Profile: developmentFixtureProfile, RunID: config.RunID, FixtureMode: config.Mode,
		SourceIdentity: "development-fixture:" + harnessHash + ":" + runnerHash,
		SupplyClass:    "unrestricted-schema-fixture",
		HarnessSHA256:  harnessHash, RunnerSHA256: runnerHash, VerifierSHA256: verifierHash,
		ClientSHA256: clientHash, ServerSHA256: serverHash,
		CanarySHA256: canaryHash, Groups: groups, Topology: topology, Boundaries: boundaries, ResidualKinds: residualKinds,
		SecretArtifacts: artifacts, SupplementalArtifacts: supplemental,
		CreatedUnixNano: time.Now().UnixNano(), ManifestNonceHash: nonceHash,
		EvidenceRootHash: hex.EncodeToString(rootHash[:]), RegistryRootHash: hex.EncodeToString(registryHash[:]),
		AttributionSources: attributionSources}
	if finalSpecValue != nil {
		result.CampaignKind = "final-local"
		result.Profile = finalCampaignProfile
		result.FixtureMode = "final-campaign"
		result.SourceIdentity = "repository:" + finalSpecValue.RepositoryCommit + ":" + finalSpecValue.SourceSHA256
		result.SupplyClass = "pinned-offline-webtunnel"
		result.FinalSpec = finalSpecValue
	}
	return result, nil
}

func validateFinalRunnerBinding(runnerHash string, value *finalSpec) error {
	if value != nil && runnerHash != value.ProductReceipt.NetworkSHA256 {
		return errors.New("final runner differs from the archive-built product receipt")
	}
	return nil
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
	canaries canaryCorpus, finalSpecValue *finalSpec,
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
	allowed["final-campaign"] = true
	for _, field := range []string{"invite", "address", "path", "certificate"} {
		allowed["candidate-canary-"+field] = true
		allowed["pipeline-canary-"+field] = true
	}
	if !allowed[mode] {
		return evidence{}, nil, errors.New("blocked-entry harness mode is unsupported")
	}
	events, observers, cleanup, finalSummaryValue, err := collectEvents(config, canaries, finalSpecValue)
	if err != nil {
		return evidence{}, nil, err
	}
	if err := publishFinalMutationInputs(config.EvidenceRoot+".partial/secret", finalSpecValue,
		finalSummaryValue, canaries); err != nil {
		return evidence{}, nil, err
	}
	if err := publishFinalMeasurements(config.EvidenceRoot+".partial/secret", finalSummaryValue); err != nil {
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
		AttributionArtifacts: attributions, CollectionClosed: true, FinalSummary: finalSummaryValue}
	if finalSummaryValue != nil {
		result.CampaignKind = "final-local"
		result.Profile = finalCampaignProfile
	}
	return injectMode(result, mode, canaries)
}
