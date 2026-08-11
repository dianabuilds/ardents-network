package preflight

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

type fixture struct {
	input  input
	layout RunLayout
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	return newFixtureAt(t, t.TempDir(), "20260808T120000Z-42")
}

func newFixtureAt(t *testing.T, tempRoot, runID string) fixture {
	t.Helper()
	sessionRoot := filepath.Join(tempRoot, sessionDirectoryPrefix+runID)
	repositoryRoot := filepath.Join(tempRoot, "repository-"+runID)
	for _, directory := range []string{sessionRoot, repositoryRoot} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	layout, err := NewRunLayout(sessionRoot, repositoryRoot, tempRoot, runID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(layout.runDir, 0o700); err != nil {
		t.Fatal(err)
	}
	value := validInput()
	value.RunID = runID
	value.GoCache = filepath.Join(layout.runDir, "cache", "go-build")
	value.GoModCache = filepath.Join(layout.runDir, "cache", "go-mod")
	return fixture{input: value, layout: layout}
}

func allResourcesAbsent() OwnedResources {
	return OwnedResources{ContainersAbsent: true, NetworksAbsent: true, VolumesAbsent: true}
}

func validInput() input {
	return input{
		SchemaVersion: inputSchemaVersion, RunID: "20260808T120000Z-42", Seed: "20260808",
		GitRevision: "0123456789abcdef0123456789abcdef01234567", GitDirty: true,
		HostOS: "linux", HostArch: "amd64", HostUbuntuVersion: expectedUbuntuVersion,
		ImageReference: expectedImageReference, ExpectedImageManifestDigest: expectedImageManifestDigest,
		ObservedImageManifestDigest: expectedImageManifestDigest,
		ImageID:                     "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		CarrierLabImageID:           "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		BinarySHA256:                "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		GoArchiveName:               expectedGoArchiveName,
		ExpectedGoArchiveSHA256:     expectedGoArchiveSHA256,
		ObservedGoArchiveSHA256:     expectedGoArchiveSHA256,
		RepositoryMount:             "read-only",
		ContainerNetwork:            "none",
		GoProxy:                     "off",
		GoCache:                     "/run/cache/go-build",
		GoModCache:                  "/run/cache/go-mod",
		Tools: toolVersions{Bash: "GNU bash 5.2.21", Git: "git version 2.43.0", DockerClient: "29.1.3",
			DockerServer: "29.1.3", SHA256Sum: "sha256sum (GNU coreutils) 9.4", Tar: "tar (GNU tar) 1.35"},
	}
}

func testRuntime() runtimeOptions {
	return runtimeOptions{
		ExecutionOS: "linux", ExecutionArch: "amd64", UbuntuID: "ubuntu",
		UbuntuVersion: expectedUbuntuVersion, RuntimeGoVersion: expectedGoVersion,
		Now: func() time.Time { return time.Unix(1_800_000_000, 0) },
	}
}

func loadManifest(t *testing.T, evidenceDir string) manifest {
	t.Helper()
	var record manifest
	loadJSON(t, filepath.Join(evidenceDir, manifestFilename), &record)
	return record
}

func loadJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("%s is not valid JSON: %v", path, err)
	}
}

func assertReasonCode(t *testing.T, reasons []failureReason, code string) {
	t.Helper()
	if !slices.ContainsFunc(reasons, func(reason failureReason) bool { return reason.Code == code }) {
		t.Fatalf("failure reasons %v do not contain code %q", reasons, code)
	}
}

func tree(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	slices.Sort(paths)
	return paths
}
