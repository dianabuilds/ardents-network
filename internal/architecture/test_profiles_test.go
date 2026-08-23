package architecture

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type testProfileRegistry struct {
	Schema     string          `json:"schema"`
	Profiles   []testProfile   `json:"profiles"`
	SuiteRoots []testSuiteRoot `json:"suite_roots"`
}

type testSuiteRoot struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Profile string `json:"profile"`
	Owner   string `json:"owner"`
}

func TestPackageProfileMembershipIsComplete(t *testing.T) {
	root := repositoryRoot(t)
	actual := listedPackages(t, root, "./cmd/...", "./internal/...")
	deterministic := listedProfilePackages(t, root, "tests/profiles/deterministic-packages.txt")
	for packagePath := range actual {
		_, inDeterministic := deterministic[packagePath]
		if !inDeterministic {
			t.Errorf("package %s must belong to the deterministic profile", packagePath)
		}
	}
}

func TestEndToEndPackageProfileMembershipIsComplete(t *testing.T) {
	root := repositoryRoot(t)
	actual := listedPackages(t, root, "./tests/e2e/...")
	process := listedProfilePackages(t, root, "tests/profiles/process-packages.txt")
	for packagePath := range actual {
		_, inProcess := process[packagePath]
		if !inProcess {
			t.Errorf("e2e package %s must belong to the process profile", packagePath)
		}
	}
}

func TestProfilePackageEntriesAreCurrent(t *testing.T) {
	root := repositoryRoot(t)
	actual := listedPackages(t, root, "./cmd/...", "./internal/...", "./tests/e2e/...")
	for _, path := range []string{
		"tests/profiles/deterministic-packages.txt",
		"tests/profiles/process-packages.txt",
	} {
		for packagePath := range listedProfilePackages(t, root, path) {
			if !actual[packagePath] {
				t.Errorf("profile %s contains non-current package %s", path, packagePath)
			}
		}
	}
}

func listedPackages(t *testing.T, root string, patterns ...string) map[string]bool {
	t.Helper()
	command := exec.Command("go", append([]string{"list"}, patterns...)...)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatalf("list maintained packages: %v", err)
	}
	return packageSet(t, string(output))
}

func listedProfilePackages(t *testing.T, root, path string) map[string]bool {
	t.Helper()
	return packageSet(t, string(readProjectFile(t, root, path)))
}

func packageSet(t *testing.T, contents string) map[string]bool {
	t.Helper()
	set := make(map[string]bool)
	for _, line := range strings.Fields(contents) {
		if set[line] {
			t.Errorf("duplicate package profile entry %s", line)
		}
		set[line] = true
	}
	return set
}

type testProfile struct {
	ID                 string   `json:"id"`
	State              string   `json:"state"`
	MakeTarget         string   `json:"make_target"`
	Surface            string   `json:"surface"`
	Prerequisites      []string `json:"prerequisites"`
	InvalidEnvironment string   `json:"invalid_environment"`
	Timeout            string   `json:"timeout"`
	Activation         string   `json:"activation"`
}

func TestTestProfileRegistryIsFactualAndWired(t *testing.T) {
	root := repositoryRoot(t)
	registry := readTestProfileRegistry(t, root)
	if registry.Schema != "ardents-test-profiles-v1" {
		t.Fatalf("profile registry schema = %q", registry.Schema)
	}
	makefile := string(readProjectFile(t, root, "Makefile"))
	required := map[string]bool{
		"affected-platform": false,
		"developer":         false,
		"deterministic":     false,
		"fuzz":              false,
		"process":           false,
		"qualification":     false,
		"race":              false,
		"soak":              false,
		"live":              false,
	}
	for _, profile := range registry.Profiles {
		if _, known := required[profile.ID]; !known {
			t.Errorf("unknown test profile %q", profile.ID)
			continue
		}
		if required[profile.ID] {
			t.Errorf("duplicate test profile %q", profile.ID)
		}
		required[profile.ID] = true
		if profile.State != "active" && profile.State != "inactive" {
			t.Errorf("profile %q has invalid state %q", profile.ID, profile.State)
		}
		if profile.State == "active" && (profile.MakeTarget == "" || !strings.Contains(makefile, "\n"+profile.MakeTarget+":")) {
			t.Errorf("active profile %q names absent Make target %q", profile.ID, profile.MakeTarget)
		}
		if profile.State == "inactive" && (profile.MakeTarget != "" || profile.Activation == "") {
			t.Errorf("inactive profile %q must omit its Make target and name its activation condition", profile.ID)
		}
		if profile.Surface == "" || profile.InvalidEnvironment == "" {
			t.Errorf("profile %q lacks surface or invalid-environment result", profile.ID)
		}
		if profile.InvalidEnvironment == "not applicable" && len(profile.Prerequisites) != 0 {
			t.Errorf("profile %q cannot have prerequisites when invalid environment is not applicable", profile.ID)
		}
		if profile.Timeout != "" && !strings.Contains(makefile, "-timeout="+profile.Timeout) {
			t.Errorf("profile %q timeout %q is absent from its Make entrypoint", profile.ID, profile.Timeout)
		}
	}
	for id, found := range required {
		if !found {
			t.Errorf("required test profile %q is missing", id)
		}
	}
}

func TestSuiteRootsBelongToOneExecutionProfile(t *testing.T) {
	root := repositoryRoot(t)
	registry := readTestProfileRegistry(t, root)
	profiles := make(map[string]bool)
	for _, profile := range registry.Profiles {
		profiles[profile.ID] = true
	}
	actual := suiteRootSet(t, root)
	registered := make(map[string]bool)
	identifiers := make(map[string]bool)
	for _, suite := range registry.SuiteRoots {
		if suite.ID == "" || suite.Path == "" || suite.Owner == "" {
			t.Errorf("suite root must name an id, path, and owner: %+v", suite)
		}
		if identifiers[suite.ID] {
			t.Errorf("duplicate suite-root id %q", suite.ID)
		}
		identifiers[suite.ID] = true
		if registered[suite.Path] {
			t.Errorf("suite root %q belongs to more than one execution profile", suite.Path)
		}
		registered[suite.Path] = true
		if !profiles[suite.Profile] {
			t.Errorf("suite root %q names unknown execution profile %q", suite.Path, suite.Profile)
		}
	}
	for path := range actual {
		if !registered[path] {
			t.Errorf("current suite root %q has no execution profile", path)
		}
	}
	for path := range registered {
		if !actual[path] {
			t.Errorf("registered suite root %q does not exist", path)
		}
	}
}

func readTestProfileRegistry(t *testing.T, root string) testProfileRegistry {
	t.Helper()
	data := readProjectFile(t, root, "tests/profiles/profiles.json")
	var registry testProfileRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		t.Fatalf("decode test profile registry: %v", err)
	}
	return registry
}

func suiteRootSet(t *testing.T, root string) map[string]bool {
	t.Helper()
	roots := make(map[string]bool)
	for _, parent := range []string{"tests/e2e"} {
		entries, err := os.ReadDir(filepath.Join(root, parent))
		if err != nil {
			t.Fatalf("read suite parent %s: %v", parent, err)
		}
		for _, entry := range entries {
			if entry.IsDir() && suiteHasGoFiles(t, filepath.Join(root, parent, entry.Name())) {
				roots[parent+"/"+entry.Name()] = true
			}
		}
	}
	return roots
}

func suiteHasGoFiles(t *testing.T, path string) bool {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatalf("read suite root %s: %v", path, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			return true
		}
	}
	return false
}
