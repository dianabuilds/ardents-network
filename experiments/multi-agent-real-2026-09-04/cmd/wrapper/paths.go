//go:build ignore

package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var runIDPattern = regexp.MustCompile(`^[0-9]{8}T[0-9]{6}Z-[0-9a-f]{12}$`)

func prepareRun(evidenceRoot string, now func() time.Time, entropy io.Reader) (string, runManifest, error) {
	root, err := filepath.Abs(evidenceRoot)
	if err != nil {
		return "", runManifest{}, err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", runManifest{}, errors.New("evidence root must be an existing directory")
	}
	if insideGitWorktree(root) {
		return "", runManifest{}, errors.New("evidence root must be outside a Git worktree")
	}
	for _, definition := range personaDefinitions {
		if err := requireRegularFile(filepath.Join(root, "fixtures", definition.fixture)); err != nil {
			return "", runManifest{}, err
		}
	}
	runID, err := newRunID(now().UTC(), entropy)
	if err != nil {
		return "", runManifest{}, err
	}
	runsRoot := filepath.Join(root, "runs")
	if err := os.MkdirAll(runsRoot, 0o700); err != nil {
		return "", runManifest{}, err
	}
	runRoot := filepath.Join(runsRoot, runID)
	if err := os.Mkdir(runRoot, 0o700); err != nil {
		return "", runManifest{}, err
	}
	succeeded := false
	defer func() {
		if !succeeded {
			_ = os.RemoveAll(runRoot)
		}
	}()

	manifest := runManifest{Schema: manifestSchema, RunID: runID, CreatedAt: now().UTC().Format(time.RFC3339),
		HostRunRoot: runRoot, Container: defaultContainer, ContainerEvidenceRoot: defaultContainerEvidence,
		Personas: make(map[string]personaConfig, len(personaDefinitions))}
	containerRunRoot := path.Join(defaultContainerEvidence, "runs", runID)
	if err := os.Mkdir(filepath.Join(runRoot, "plans"), 0o700); err != nil {
		return "", runManifest{}, err
	}
	if err := os.Mkdir(filepath.Join(runRoot, "prompts"), 0o700); err != nil {
		return "", runManifest{}, err
	}
	for _, definition := range personaDefinitions {
		personaRoot := filepath.Join(runRoot, definition.name)
		if err := os.MkdirAll(filepath.Join(personaRoot, "events"), 0o700); err != nil {
			return "", runManifest{}, err
		}
		if err := os.Mkdir(filepath.Join(personaRoot, "_meta"), 0o700); err != nil {
			return "", runManifest{}, err
		}
		config := personaConfig{Name: definition.name,
			StateRoot:          path.Join(containerRunRoot, definition.name, "state"),
			LocalRoleStateRoot: path.Join(containerRunRoot, definition.name, "local-roles"),
			SourcePlan:         path.Join(containerRunRoot, "plans", definition.name+".json"),
			ExpectedKind:       definition.expectedKind, ExpectedOutcomes: definition.expectedOutcomes,
			MinimumEvents: definition.minimumEvents, MinimumSpanSeconds: definition.minimumSpan, AllowNoop: definition.allowNoop}
		hostPlan := filepath.Join(runRoot, "plans", definition.name+".json")
		if err := rewriteSourcePlan(filepath.Join(root, "fixtures", definition.fixture), hostPlan, config.LocalRoleStateRoot); err != nil {
			return "", runManifest{}, err
		}
		config.SourcePlanHash, err = fileHash(hostPlan)
		if err != nil {
			return "", runManifest{}, err
		}
		config.ConfigurationHash = configurationHash(config)
		manifest.Personas[definition.name] = config
		if err := writeNewFile(filepath.Join(runRoot, "prompts", definition.name+".txt"), []byte(personaPrompt(definition.name))); err != nil {
			return "", runManifest{}, err
		}
	}
	manifestPath := filepath.Join(runRoot, "manifest.json")
	if err := writeNewJSON(manifestPath, manifest); err != nil {
		return "", runManifest{}, err
	}
	succeeded = true
	return manifestPath, manifest, nil
}

func newRunID(now time.Time, entropy io.Reader) (string, error) {
	if entropy == nil {
		entropy = rand.Reader
	}
	random := make([]byte, 6)
	if _, err := io.ReadFull(entropy, random); err != nil {
		return "", fmt.Errorf("read run entropy: %w", err)
	}
	return now.Format("20060102T150405Z") + "-" + hex.EncodeToString(random), nil
}

func rewriteSourcePlan(source, destination, localRoleRoot string) error {
	raw, err := os.ReadFile(source)
	if err != nil || len(raw) == 0 || len(raw) > 32<<10 {
		return errors.New("source plan fixture is unavailable or unbounded")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var plan map[string]json.RawMessage
	if err := decoder.Decode(&plan); err != nil {
		return errors.New("source plan fixture is invalid")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("source plan fixture has trailing content")
	}
	var schema string
	if err := json.Unmarshal(plan["schema"], &schema); err != nil || schema != "ardents-source-plan-v1" {
		return errors.New("source plan fixture has the wrong schema")
	}
	encodedRoot, _ := json.Marshal(localRoleRoot)
	plan["local_role_state_root"] = encodedRoot
	rendered, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	return writeNewFile(destination, append(rendered, '\n'))
}

func configurationHash(config personaConfig) string {
	copy := config
	copy.ConfigurationHash = ""
	raw, _ := json.Marshal(copy)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func fileHash(name string) (string, error) {
	raw, err := os.ReadFile(name)
	if err != nil || len(raw) == 0 || len(raw) > 32<<10 {
		return "", errors.New("source plan is unavailable or unbounded")
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func loadManifest(name string) (runManifest, error) {
	raw, err := readBounded(name, 64<<10)
	if err != nil {
		return runManifest{}, errors.New("run manifest is unavailable or unbounded")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var manifest runManifest
	if err := decoder.Decode(&manifest); err != nil {
		return runManifest{}, errors.New("run manifest is invalid")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return runManifest{}, errors.New("run manifest has trailing content")
	}
	if err := validateManifest(name, manifest); err != nil {
		return runManifest{}, err
	}
	return manifest, nil
}

func validateManifest(name string, manifest runManifest) error {
	manifestPath, err := filepath.Abs(name)
	if err != nil {
		return err
	}
	if manifest.Schema != manifestSchema || !runIDPattern.MatchString(manifest.RunID) ||
		manifest.Container != defaultContainer || manifest.ContainerEvidenceRoot != defaultContainerEvidence ||
		filepath.Clean(manifest.HostRunRoot) != filepath.Dir(manifestPath) || len(manifest.Personas) != len(personaDefinitions) {
		return errors.New("run manifest identity is invalid")
	}
	containerRunRoot := path.Join(defaultContainerEvidence, "runs", manifest.RunID)
	for _, definition := range personaDefinitions {
		config, ok := manifest.Personas[definition.name]
		hostPlan := filepath.Join(manifest.HostRunRoot, "plans", definition.name+".json")
		actualPlanHash, hashErr := fileHash(hostPlan)
		if !ok || config.Name != definition.name || config.StateRoot != path.Join(containerRunRoot, definition.name, "state") ||
			config.LocalRoleStateRoot != path.Join(containerRunRoot, definition.name, "local-roles") ||
			config.SourcePlan != path.Join(containerRunRoot, "plans", definition.name+".json") ||
			config.ExpectedKind != definition.expectedKind || config.ExpectedOutcomes != definition.expectedOutcomes ||
			config.MinimumEvents != definition.minimumEvents || config.MinimumSpanSeconds != definition.minimumSpan ||
			config.AllowNoop != definition.allowNoop ||
			hashErr != nil || config.SourcePlanHash != actualPlanHash || config.ConfigurationHash != configurationHash(config) ||
			strings.Contains(strings.ToLower(config.StateRoot+config.LocalRoleStateRoot), "tick") {
			return fmt.Errorf("run manifest persona %s is invalid", definition.name)
		}
	}
	return nil
}

func requireRegularFile(name string) error {
	info, err := os.Lstat(name)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("required regular file is unavailable: %s", name)
	}
	return nil
}

func writeNewJSON(name string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeNewFile(name, append(raw, '\n'))
}

func writeNewFile(name string, raw []byte) error {
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	return errors.Join(err, file.Close())
}

func insideGitWorktree(root string) bool {
	for current := root; ; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false
		}
	}
}
