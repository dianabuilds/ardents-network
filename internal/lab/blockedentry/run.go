package blockedentry

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Run creates one immutable external evidence bundle without computing its verdict.
func Run(config Config) (Result, error) {
	if config.PreparationRoot != "" {
		return prepareFinal(config)
	}
	if config.Mode == "" {
		config.Mode = "pass"
	}
	workspace, workspaceErr := filepath.Abs(config.WorkspaceRoot)
	evidenceRoot, evidenceErr := filepath.Abs(config.EvidenceRoot)
	registryRoot, registryErr := filepath.Abs(config.RegistryRoot)
	if workspaceErr != nil || evidenceErr != nil || registryErr != nil {
		return Result{}, errors.Join(workspaceErr, evidenceErr, registryErr)
	}
	workspaceAliased, workspaceAliasErr := pathHasSymlink(workspace)
	evidenceAliased, evidenceAliasErr := pathHasSymlink(filepath.Dir(evidenceRoot))
	registryAliased, registryAliasErr := pathHasSymlink(filepath.Dir(registryRoot))
	if workspaceAliasErr != nil || evidenceAliasErr != nil || registryAliasErr != nil {
		return Result{}, errors.Join(workspaceAliasErr, evidenceAliasErr, registryAliasErr)
	}
	if config.RunID == "" || len(config.RunID) > 80 || strings.ContainsAny(config.RunID, `/\\`) {
		return Result{}, errors.New("blocked-entry run identity is invalid")
	}
	if workspaceAliased || evidenceAliased || registryAliased || within(workspace, evidenceRoot) ||
		within(workspace, registryRoot) || workspace == evidenceRoot || workspace == registryRoot ||
		evidenceRoot == registryRoot || within(evidenceRoot, registryRoot) || within(registryRoot, evidenceRoot) {
		return Result{}, errors.New("blocked-entry run identity or external evidence root is invalid")
	}
	if err := validateWorkspace(workspace); err != nil {
		return Result{}, err
	}
	if _, err := os.Lstat(evidenceRoot); !os.IsNotExist(err) {
		return Result{}, errors.New("blocked-entry evidence root must not already exist")
	}
	temporary := evidenceRoot + ".partial"
	if _, err := os.Lstat(temporary); !os.IsNotExist(err) {
		return Result{}, errors.New("blocked-entry construction root must not already exist")
	}
	if err := os.Mkdir(temporary, 0o700); err != nil {
		return Result{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(temporary)
		}
	}()
	if err := protectEvidenceTree(temporary); err != nil {
		return Result{}, err
	}
	publishable := filepath.Join(temporary, "publishable")
	secretRoot := filepath.Join(temporary, "secret")
	if err := os.Mkdir(publishable, 0o700); err != nil {
		return Result{}, err
	}
	if err := os.Mkdir(secretRoot, 0o700); err != nil {
		return Result{}, err
	}
	config, err := freezeSupply(config, secretRoot)
	if err != nil {
		return Result{}, err
	}
	config, finalSpecValue, err := freezeCampaignSpec(config, secretRoot)
	if err != nil {
		return Result{}, err
	}
	canaries, canaryHash, err := createCanaries(secretRoot)
	if err != nil {
		return Result{}, err
	}
	artifacts, err := secretArtifacts(secretRoot, config)
	if err != nil {
		return Result{}, err
	}
	nonceHash, err := createNonceHash()
	if err != nil {
		return Result{}, err
	}
	supplemental, plannedContamination := plannedSupplemental(config.Mode, canaries)
	manifestValue, err := buildManifest(config, finalSpecValue, canaryHash, nonceHash, artifacts, supplemental)
	if err != nil {
		return Result{}, err
	}
	manifestPath := filepath.Join(publishable, "manifest.json")
	if err := writeJSON(manifestPath, manifestValue); err != nil {
		return Result{}, err
	}
	if err := os.Chmod(manifestPath, 0o400); err != nil {
		return Result{}, err
	}
	manifestRaw, err := os.ReadFile(manifestPath)
	if err != nil {
		return Result{}, err
	}
	manifestDigest := sha256.Sum256(manifestRaw)
	manifestHash := hex.EncodeToString(manifestDigest[:])
	evidenceValue, contamination, err := buildEvidence(config, manifestHash, artifacts, supplemental, canaries,
		finalSpecValue)
	if err != nil {
		return Result{}, err
	}
	evidencePath := filepath.Join(publishable, "evidence.json")
	if err := writeJSON(evidencePath, evidenceValue); err != nil {
		return Result{}, err
	}
	if string(contamination) != string(plannedContamination) {
		return Result{}, errors.New("publishable contamination differs from its immutable commitment")
	}
	evidenceRaw, err := os.ReadFile(evidencePath)
	if err != nil {
		return Result{}, err
	}
	evidenceDigest := sha256.Sum256(evidenceRaw)
	closurePath := filepath.Join(publishable, "closure.json")
	closure := evidenceClosure{Schema: "ardents-h3-blocked-entry-closure-v1", RunID: config.RunID,
		ManifestSHA256: manifestHash, EvidenceSHA256: hex.EncodeToString(evidenceDigest[:]),
		ClosedUnixNano: time.Now().UnixNano()}
	if err := writeJSON(closurePath, closure); err != nil {
		return Result{}, err
	}
	if len(contamination) > 0 {
		if err := os.WriteFile(filepath.Join(publishable, "pipeline-note.bin"), contamination, 0o600); err != nil {
			return Result{}, err
		}
	}
	if err := protectEvidenceTree(temporary); err != nil {
		return Result{}, err
	}
	if err := publishDirectory(temporary, evidenceRoot); err != nil {
		return Result{}, err
	}
	committed = true
	return Result{ManifestPath: filepath.Join(evidenceRoot, "publishable", "manifest.json"),
		EvidencePath: filepath.Join(evidenceRoot, "publishable", "evidence.json"),
		ClosurePath:  filepath.Join(evidenceRoot, "publishable", "closure.json"),
		SecretRoot:   filepath.Join(evidenceRoot, "secret"), CanaryPath: filepath.Join(evidenceRoot, "secret", "canaries.json"),
		PublishableRoot: filepath.Join(evidenceRoot, "publishable"), ManifestSHA256: manifestHash}, nil
}

func validateWorkspace(root string) error {
	module, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || !strings.HasPrefix(string(module), "module github.com/dianabuilds/ardents-network\n") {
		return errors.New("workspace root is not the Ardents Network repository")
	}
	info, err := os.Stat(filepath.Join(root, ".git"))
	if err != nil || !info.IsDir() {
		return errors.New("workspace root has no repository identity")
	}
	return nil
}

func pathHasSymlink(path string) (bool, error) {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return true, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false, nil
		}
	}
}
