package tooling

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseToolLockAndVerifyBundle(t *testing.T) {
	t.Parallel()
	fixture := newToolBundleFixture(t)

	verified, err := verifyToolBundle(fixture.lockPath, fixture.bundleDir, fixture.repositoryRoot)
	if err != nil {
		t.Fatalf("VerifyToolBundle() error = %v", err)
	}
	if verified.Schema != toolBundleSchema || verified.Platform != "linux/amd64" {
		t.Fatalf("verified identity = %#v", verified)
	}
	if got := verified.Tools["tc"].Version; got != "6.19.0" {
		t.Fatalf("tc version = %q", got)
	}
	if len(verified.Packages) != 1 || verified.Packages[0].Name != "iproute2" {
		t.Fatalf("packages = %#v", verified.Packages)
	}
}

func TestVerifyToolObservationRejectsVersionOrDigestMismatch(t *testing.T) {
	t.Parallel()
	fixture := newToolBundleFixture(t)
	verified, err := verifyToolBundle(fixture.lockPath, fixture.bundleDir, fixture.repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name        string
		observation toolObservation
	}{
		{name: "version", observation: toolObservation{Name: "tc", Version: "6.18.0", Path: "/usr/sbin/tc", SHA256: strings.Repeat("a", 64)}},
		{name: "digest", observation: toolObservation{Name: "tc", Version: "6.19.0", Path: "/usr/sbin/tc", SHA256: strings.Repeat("b", 64)}},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := verified.verifyObservation(test.observation); err == nil {
				t.Fatal("mismatched tool observation was accepted")
			}
		})
	}
}

func TestVerifyToolBundleRejectsMissingExtraOrDigestMismatch(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		edit func(*testing.T, toolBundleFixture)
	}{
		{name: "missing", edit: func(t *testing.T, fixture toolBundleFixture) {
			if err := os.Remove(filepath.Join(fixture.bundleDir, fixture.filename)); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "extra", edit: func(t *testing.T, fixture toolBundleFixture) {
			if err := os.WriteFile(filepath.Join(fixture.bundleDir, "unexpected.deb"), []byte("extra"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "digest", edit: func(t *testing.T, fixture toolBundleFixture) {
			if err := os.WriteFile(filepath.Join(fixture.bundleDir, fixture.filename), []byte("tampered"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newToolBundleFixture(t)
			test.edit(t, fixture)
			if _, err := verifyToolBundle(fixture.lockPath, fixture.bundleDir, fixture.repositoryRoot); err == nil {
				t.Fatal("invalid tool bundle was accepted")
			}
		})
	}
}

func TestVerifyToolBundleRejectsRepositoryPath(t *testing.T) {
	t.Parallel()
	fixture := newToolBundleFixture(t)
	bundle := filepath.Join(fixture.repositoryRoot, "bundle")
	if err := os.Mkdir(bundle, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, fixture.filename), fixture.payload, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := verifyToolBundle(fixture.lockPath, bundle, fixture.repositoryRoot); err == nil {
		t.Fatal("tool bundle inside repository was accepted")
	}
}

func TestVerifyToolBundleRejectsAlternateLock(t *testing.T) {
	t.Parallel()
	fixture := newToolBundleFixture(t)
	alternate := filepath.Join(filepath.Dir(fixture.repositoryRoot), "alternate.lock")
	data, err := os.ReadFile(fixture.lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(alternate, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyToolBundle(alternate, fixture.bundleDir, fixture.repositoryRoot); err == nil {
		t.Fatal("alternate caller-selected lock was accepted")
	}
}

func TestFilesystemPathComparisonUsesPlatformSemantics(t *testing.T) {
	t.Parallel()
	left := filepath.Join("root", "carrier-lab", "tools.lock")
	right := filepath.Join("root", "Carrier-Lab", "tools.lock")
	wantEqual := runtime.GOOS == "windows"
	if got := sameFilesystemPath(left, right); got != wantEqual {
		t.Fatalf("case-variant paths equal = %t, want %t", got, wantEqual)
	}
}

type toolBundleFixture struct {
	lockPath       string
	bundleDir      string
	repositoryRoot string
	filename       string
	payload        []byte
}

func newToolBundleFixture(t *testing.T) toolBundleFixture {
	t.Helper()
	root := t.TempDir()
	repositoryRoot := filepath.Join(root, "repository")
	bundleDir := filepath.Join(root, "bundle")
	for _, directory := range []string{repositoryRoot, bundleDir} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	payload := []byte("official-package-fixture")
	digest := sha256.Sum256(payload)
	packageDigest := hex.EncodeToString(digest[:])
	filename := "iproute2_6.19.0-1ubuntu1.1_amd64.deb"
	lock := strings.Join([]string{
		"meta\tschema\tcarrier-lab-tool-bundle/v1",
		"meta\tplatform\tlinux/amd64",
		"meta\tbase_image\tubuntu@sha256:" + strings.Repeat("1", 64),
		"tool\ttc\t6.19.0\t/usr/sbin/tc\t" + strings.Repeat("a", 64),
		"package\tiproute2\t6.19.0-1ubuntu1.1\t" + filename + "\t" + packageDigest + "\thttps://archive.ubuntu.com/ubuntu/pool/main/i/iproute2/" + filename + "\tGPL-2.0-only",
	}, "\n") + "\n"
	carrierLabDirectory := filepath.Join(repositoryRoot, "carrier-lab")
	if err := os.Mkdir(carrierLabDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(carrierLabDirectory, "tools.lock")
	if err := os.WriteFile(lockPath, []byte(lock), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, filename), payload, 0o600); err != nil {
		t.Fatal(err)
	}
	return toolBundleFixture{lockPath: lockPath, bundleDir: bundleDir, repositoryRoot: repositoryRoot, filename: filename, payload: payload}
}
