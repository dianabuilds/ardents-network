package architecture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type ownershipRegistry struct {
	Schema             string              `json:"schema"`
	Owners             []string            `json:"owners"`
	Rules              []ownershipRule     `json:"rules"`
	QualificationLanes []qualificationLane `json:"qualification_lanes"`
	ArtifactLanes      []artifactLane      `json:"artifact_lanes"`
}

type ownershipRule struct {
	ID              string   `json:"id"`
	Owner           string   `json:"owner"`
	IncludePrefixes []string `json:"include_prefixes"`
	IncludePaths    []string `json:"include_paths"`
	ExcludePrefixes []string `json:"exclude_prefixes"`
	ExcludePaths    []string `json:"exclude_paths"`
}

type qualificationLane struct {
	ID, Owner, Path string
}

type artifactLane struct {
	ID               string `json:"id"`
	Owner            string `json:"owner"`
	Commands         string `json:"commands"`
	Packaging        string `json:"packaging"`
	EvidencePackages string `json:"evidence_packages"`
}

func TestMaintainedFilesHaveExactlyOneOwner(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	registry := readOwnershipRegistry(t, root)
	walk(t, root, func(path string, entry os.DirEntry) {
		if entry.IsDir() {
			return
		}
		relative := relativePath(t, root, path)
		if relative == ".git" {
			return
		}
		var matches []string
		for _, rule := range registry.Rules {
			if ruleMatches(rule, relative) {
				matches = append(matches, rule.ID)
			}
		}
		if len(matches) != 1 {
			t.Errorf("maintained file %s has %d ownership matches: %v", relative, len(matches), matches)
		}
	})
}

func TestOwnershipRegistryNamesCurrentQualificationAndArtifactLanes(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	registry := readOwnershipRegistry(t, root)
	wantQualification := make(map[string]bool)
	entries, err := os.ReadDir(filepath.Join(root, "tests", "qualification"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			wantQualification["tests/qualification/"+entry.Name()] = true
		}
	}
	seen := make(map[string]bool)
	for _, lane := range registry.QualificationLanes {
		if lane.ID == "" || !containsString(registry.Owners, lane.Owner) || !wantQualification[lane.Path] || seen[lane.Path] {
			t.Errorf("invalid or duplicate qualification lane: %+v", lane)
		}
		seen[lane.Path] = true
	}
	for path := range wantQualification {
		if !seen[path] {
			t.Errorf("qualification lane has no owner: %s", path)
		}
	}
	wantArtifactInventories := make(map[string]bool)
	profileEntries, err := os.ReadDir(filepath.Join(root, "tests", "profiles"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range profileEntries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), "-commands.txt") {
			wantArtifactInventories["tests/profiles/"+entry.Name()] = true
		}
	}
	commands := make(map[string]string)
	compositions := make(map[string]string)
	identifiers := make(map[string]bool)
	for _, lane := range registry.ArtifactLanes {
		if lane.ID == "" || identifiers[lane.ID] || !containsString(registry.Owners, lane.Owner) || !wantArtifactInventories[lane.Commands] {
			t.Errorf("invalid artifact lane: %+v", lane)
		}
		identifiers[lane.ID] = true
		if other := commands[lane.Commands]; other != "" {
			t.Errorf("artifact lane %s merges an inventory with another lane", lane.ID)
		}
		commands[lane.Commands] = lane.ID

		composition := lane.Packaging
		if (lane.Packaging == "") == (lane.EvidencePackages == "") {
			t.Errorf("artifact lane %s must name exactly one packaging or evidence-packages composition", lane.ID)
			continue
		}
		if lane.EvidencePackages != "" {
			composition = lane.EvidencePackages
			wantEvidence := strings.TrimSuffix(lane.Commands, "-commands.txt") + "-packages.txt"
			if lane.EvidencePackages != wantEvidence {
				t.Errorf("artifact lane %s evidence packages = %q, want %q", lane.ID, lane.EvidencePackages, wantEvidence)
			}
		}
		if other := compositions[composition]; other != "" {
			t.Errorf("artifact lanes %s and %s share composition %s", other, lane.ID, composition)
		}
		compositions[composition] = lane.ID
		assertArtifactLanePath(t, root, registry, lane.Owner, lane.Commands, false)
		assertArtifactLanePath(t, root, registry, lane.Owner, composition, lane.Packaging != "")
	}
	for inventory := range wantArtifactInventories {
		if commands[inventory] == "" {
			t.Errorf("artifact command inventory has no owner: %s", inventory)
		}
	}
}

func assertArtifactLanePath(t *testing.T, root string, registry ownershipRegistry, owner, path string, wantDirectory bool) {
	t.Helper()
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Errorf("artifact lane path %s: %v", path, err)
		return
	}
	if info.IsDir() != wantDirectory {
		t.Errorf("artifact lane path %s directory = %t, want %t", path, info.IsDir(), wantDirectory)
	}
	ownershipPath := path
	if wantDirectory {
		ownershipPath += "/"
	}
	var matches []ownershipRule
	for _, rule := range registry.Rules {
		if ruleMatches(rule, ownershipPath) {
			matches = append(matches, rule)
		}
	}
	if len(matches) != 1 || matches[0].Owner != owner {
		t.Errorf("artifact lane path %s owner = %v, want exactly %s", path, matches, owner)
	}
}

func TestApplicationCandidateImportsOnlyBrowserAndInterfaceOwners(t *testing.T) {
	root := repositoryRoot(t)
	for _, command := range []string{"./cmd/ardents-browser", "./cmd/ardents-browser-entry"} {
		for dependency := range listedDependencies(t, root, command) {
			if !strings.HasPrefix(dependency, modulePath+"/internal/") {
				continue
			}
			if strings.HasPrefix(dependency, modulePath+"/internal/browser/") ||
				strings.HasPrefix(dependency, modulePath+"/internal/application/interfacev1/") {
				continue
			}
			t.Errorf("Application command %s imports Network implementation %s", command, dependency)
		}
	}
}

func readOwnershipRegistry(t *testing.T, root string) ownershipRegistry {
	t.Helper()
	var registry ownershipRegistry
	if err := json.Unmarshal(readProjectFile(t, root, "docs/development/ownership.json"), &registry); err != nil {
		t.Fatalf("decode ownership registry: %v", err)
	}
	if registry.Schema != "ardents-source-ownership-v1" {
		t.Fatalf("ownership schema = %q", registry.Schema)
	}
	owners := make(map[string]bool)
	for _, owner := range registry.Owners {
		if owner == "" || owners[owner] {
			t.Fatalf("invalid ownership owner %q", owner)
		}
		owners[owner] = true
	}
	ids := make(map[string]bool)
	for _, rule := range registry.Rules {
		if rule.ID == "" || ids[rule.ID] || !owners[rule.Owner] {
			t.Fatalf("invalid ownership rule %+v", rule)
		}
		ids[rule.ID] = true
	}
	return registry
}

func ruleMatches(rule ownershipRule, path string) bool {
	included := containsString(rule.IncludePaths, path)
	for _, prefix := range rule.IncludePrefixes {
		included = included || strings.HasPrefix(path, prefix)
	}
	if !included || containsString(rule.ExcludePaths, path) {
		return false
	}
	for _, prefix := range rule.ExcludePrefixes {
		if strings.HasPrefix(path, prefix) {
			return false
		}
	}
	return true
}

func containsString(values []string, wanted string) bool {
	index := sort.SearchStrings(values, wanted)
	if index < len(values) && values[index] == wanted {
		return true
	}
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
