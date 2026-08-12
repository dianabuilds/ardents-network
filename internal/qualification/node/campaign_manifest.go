package node

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

const nodeCampaignManifestSeal = "campaign-manifest.sha256"

type nodeCampaignManifest struct {
	Schema                string            `json:"schema"`
	Mode                  string            `json:"mode"`
	ExpectedResult        string            `json:"expected_result"`
	DurationSeconds       int64             `json:"duration_seconds"`
	FixtureManifestSHA256 string            `json:"fixture_manifest_sha256"`
	FixtureInputsSHA256   string            `json:"fixture_inputs_sha256"`
	InitialStateSHA256    string            `json:"initial_state_sha256"`
	SourceDigest          string            `json:"source_digest"`
	ComposeSHA256         string            `json:"compose_sha256"`
	SampleSchema          string            `json:"sample_schema"`
	Calculator            string            `json:"calculator"`
	SampleIntervalMS      int               `json:"sample_interval_ms"`
	SampleLimit           int               `json:"sample_limit"`
	SampleBudget          int64             `json:"sample_budget_bytes"`
	Clock                 string            `json:"clock"`
	Topology              []string          `json:"topology"`
	ResourceProfiles      map[string]string `json:"resource_profiles"`
	Workload              []string          `json:"workload"`
	FaultPlan             []string          `json:"fault_plan"`
}

func (observer *nodeObserver) freezeCampaignManifest(fixtureManifest, compose []byte) error {
	mode, found := selectNodeCampaignMode(observer.input.Mode)
	if !found {
		return errors.New("node campaign mode is invalid")
	}
	fixtureDigest, composeDigest := sha256.Sum256(fixtureManifest), sha256.Sum256(compose)
	inputDigest, err := immutableNodeFixtureDigest(observer.input.FixtureRoot)
	if err != nil {
		return err
	}
	stateDigest, err := nodeFixtureDirectoryDigest(observer.input.FixtureRoot, "state")
	if err != nil {
		return err
	}
	manifest := nodeCampaignManifest{Schema: "ardents-h3-node-campaign-v1", Mode: observer.input.Mode,
		ExpectedResult: "pass", DurationSeconds: int64(mode.duration.Seconds()), SourceDigest: observer.sourceDigest,
		FixtureManifestSHA256: hex.EncodeToString(fixtureDigest[:]), ComposeSHA256: hex.EncodeToString(composeDigest[:]),
		FixtureInputsSHA256: inputDigest, InitialStateSHA256: stateDigest,
		SampleSchema: "ardents-h3-node-sample-v1", Calculator: "ardents-h3-node-calculator-v1", SampleIntervalMS: 1000,
		SampleLimit: mode.sampleLimit, SampleBudget: mode.sampleBudget, Clock: "h-owned-monotonic-and-utc",
		Topology:         []string{"source1", "source2", "endpoint", "node1", "node2", "external-host-pid-collector"},
		ResourceProfiles: map[string]string{"source1": "h3-s-v1", "source2": "h3-s-v1", "endpoint": "h3-s-v1", "node1": "h3-np1-v1", "node2": "h3-np1-v1"},
		Workload:         mode.workload, FaultPlan: mode.faults}
	path := filepath.Join(observer.input.EvidenceRoot, "campaign-manifest.json")
	if err := byteio.WriteJSON(path, manifest, 64<<10); err != nil {
		return err
	}
	raw, err := byteio.ReadFile(path, 64<<10)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	return os.WriteFile(filepath.Join(observer.input.EvidenceRoot, nodeCampaignManifestSeal), []byte(hex.EncodeToString(digest[:])+"\n"), 0o600)
}

func nodeFixtureDirectoryDigest(root, name string) (string, error) {
	paths, err := collectNodeFixtureFiles(filepath.Join(root, name), 4096)
	if err != nil {
		return "", err
	}
	return digestNodeFixtureFiles(root, paths)
}

func immutableNodeFixtureDigest(root string) (string, error) {
	paths := []string{filepath.Join(root, "manifest.json"), filepath.Join(root, ".ardents-node-manifest.sha256")}
	for _, name := range []string{"artifacts", "plans", "secrets"} {
		files, err := collectNodeFixtureFiles(filepath.Join(root, name), 4096-len(paths))
		if err != nil {
			return "", err
		}
		paths = append(paths, files...)
	}
	return digestNodeFixtureFiles(root, paths)
}

func collectNodeFixtureFiles(root string, remaining int) ([]string, error) {
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if remaining < 1 || rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return nil, errors.New("node fixture input root is invalid")
	}
	paths, directories, entries := make([]string, 0, min(remaining, 64)), []string{root}, 0
	for len(directories) > 0 {
		directory := directories[0]
		directories = directories[1:]
		children, readErr := readNodeFixtureDirectory(directory, remaining-entries)
		if readErr != nil {
			return nil, readErr
		}
		entries += len(children)
		for _, child := range children {
			path := filepath.Join(directory, child.Name())
			if child.Type()&os.ModeSymlink != 0 {
				return nil, errors.New("node fixture input is a symlink")
			}
			if child.IsDir() {
				directories = append(directories, path)
				continue
			}
			info, infoErr := child.Info()
			if infoErr != nil {
				return nil, infoErr
			}
			if !info.Mode().IsRegular() {
				return nil, errors.New("node fixture input is not a regular owned file")
			}
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func readNodeFixtureDirectory(path string, remaining int) ([]os.DirEntry, error) {
	if remaining < 1 {
		return nil, errors.New("node fixture input count exceeds its bound")
	}
	directory, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	entries := make([]os.DirEntry, 0, min(remaining, 64))
	for {
		batch, readErr := directory.ReadDir(min(64, remaining-len(entries)+1))
		entries = append(entries, batch...)
		if len(entries) > remaining {
			_ = directory.Close()
			return nil, errors.New("node fixture input count exceeds its bound")
		}
		if errors.Is(readErr, io.EOF) {
			return entries, directory.Close()
		}
		if readErr != nil {
			_ = directory.Close()
			return nil, readErr
		}
	}
}

func validateNodeCampaignManifest(root, fixtureRoot string) error {
	raw, err := byteio.ReadFile(filepath.Join(root, "campaign-manifest.json"), 64<<10)
	if err != nil {
		return err
	}
	seal, err := byteio.ReadFile(filepath.Join(root, nodeCampaignManifestSeal), 128)
	if err != nil {
		return err
	}
	want, err := hex.DecodeString(string(bytes.TrimSpace(seal)))
	if err != nil {
		return err
	}
	actual := sha256.Sum256(raw)
	if len(want) != sha256.Size || !bytes.Equal(want, actual[:]) {
		return errors.New("node campaign manifest seal is invalid")
	}
	var manifest nodeCampaignManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return err
	}
	inputDigest, err := immutableNodeFixtureDigest(fixtureRoot)
	if err != nil {
		return err
	}
	if manifest.Schema != "ardents-h3-node-campaign-v1" || manifest.FixtureInputsSHA256 != inputDigest {
		return errors.New("node immutable fixture inputs changed after campaign freeze")
	}
	return nil
}
