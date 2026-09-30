package enrollment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/release"
)

func TestEnrollmentRefusesEachIncompleteProtectedInventory(t *testing.T) {
	names := []string{"protected-endpoint.json", "ardents-text-linux-amd64", "ardents-text-reader@.service", "ardents-text-publisher@.service", "ardents-text-reader.socket", "ardents-text-publisher.socket", "50-ardents-text.rules", "ardents-text.conf", "ardents-endpoint.service"}
	for _, missing := range names {
		t.Run(missing, func(t *testing.T) {
			root, request := enrolledFixture(t)
			files := make(map[string][]byte)
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if entry.Name() == manifestName {
					continue
				}
				files[entry.Name()], err = os.ReadFile(filepath.Join(root, entry.Name()))
				if err != nil {
					t.Fatal(err)
				}
			}
			// Include the old headless companions so that its missing-pair
			// refusal cannot mask the protected inventory defect.
			files[ExecutableArtifactName("ardents-node", request.Pin.Platform)] = []byte("node")
			files[ExecutableArtifactName("ardents-custody", request.Pin.Platform)] = []byte("custody")
			for _, name := range names {
				if name != missing {
					files[name] = []byte("manifest-authenticated resource")
				}
			}
			for name, contents := range files {
				if err := os.WriteFile(filepath.Join(root, name), contents, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			manifest := makeManifest(t, files)
			if err := os.WriteFile(filepath.Join(root, manifestName), manifest, 0o600); err != nil {
				t.Fatal(err)
			}
			pin := sha256.Sum256(manifest)
			request.Pin.ManifestSHA256 = hex.EncodeToString(pin[:])
			for _, verify := range []func(Request) (Verified, error){Verify, VerifyHeadless} {
				if _, err := verify(request); err == nil || !strings.Contains(err.Error(), "partial protected") {
					t.Errorf("incomplete protected inventory crossed enrollment: %v", err)
				}
			}
		})
	}
}

func TestProtectedGenerationProjectionAndCausalSubstitution(t *testing.T) {
	root, request := enrolledFixture(t)
	files := make(map[string][]byte)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == manifestName {
			continue
		}
		files[entry.Name()], err = os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
	}
	names := []string{"ardents-linux-amd64", "ardents-text-linux-amd64", "ardents-text-reader@.service", "ardents-text-publisher@.service", "ardents-text-reader.socket", "ardents-text-publisher.socket", "50-ardents-text.rules", "ardents-text.conf", "ardents-endpoint.service"}
	digests := make(map[string]string)
	for _, name := range names {
		if name != "ardents-linux-amd64" {
			files[name] = []byte("resource: " + name)
		}
		hash := sha256.Sum256(files[name])
		digests[name] = hex.EncodeToString(hash[:])
	}
	inventory, err := json.Marshal(digests)
	if err != nil {
		t.Fatal(err)
	}
	canonical := []byte(fmt.Sprintf("{\"schema\":\"ardents-protected-endpoint-artifact-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"alpha-1\",\"release_version\":1,\"files\":%s}\n", inventory))
	verify := func(raw []byte) (Verified, error) {
		t.Helper()
		files["protected-endpoint.json"] = raw
		for name, contents := range files {
			if err := os.WriteFile(filepath.Join(root, name), contents, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		manifest := makeManifest(t, files)
		if err := os.WriteFile(filepath.Join(root, manifestName), manifest, 0o600); err != nil {
			t.Fatal(err)
		}
		pin := sha256.Sum256(manifest)
		request.Pin.ManifestSHA256 = hex.EncodeToString(pin[:])
		return Verify(request)
	}
	verified, err := verify(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if string(verified.ProtectedDescriptor) != string(canonical) || len(verified.ProtectedFiles) != 9 {
		t.Fatal("protected projection is incomplete")
	}
	for _, name := range append(names, "protected-endpoint.json") {
		if verified.Inputs.Files[release.MetadataURL(name)] != nil {
			t.Fatalf("resource %s crossed into Release metadata", name)
		}
	}
	for label, raw := range map[string][]byte{
		"whitespace": append([]byte(" "), canonical...),
		"duplicate":  []byte(strings.Replace(string(canonical), "\"release_version\":1", "\"release_version\":1,\"release_version\":1", 1)),
		"unknown":    []byte(strings.Replace(string(canonical), "\"release_version\":1", "\"unknown\":0,\"release_version\":1", 1)),
		"identity":   []byte(strings.Replace(string(canonical), "alpha-1", "alpha-2", 1)),
		"version":    []byte(strings.Replace(string(canonical), "\"release_version\":1", "\"release_version\":0", 1)),
		"oversized":  []byte(strings.Repeat(" ", 16<<10+1)),
	} {
		t.Run(label, func(t *testing.T) {
			if _, err := verify(raw); err == nil {
				t.Fatal("invalid generation accepted despite a matching new enrollment pin")
			}
		})
	}
	files["ardents-text-linux-amd64"] = []byte("substituted worker")
	if _, err := verify(canonical); err == nil {
		t.Fatal("repinned worker substitution accepted against unchanged generation")
	}
}
