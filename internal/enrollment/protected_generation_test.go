package enrollment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func protectedFixture(t *testing.T) (Request, map[string][]byte) {
	t.Helper()
	r, files := fixture(t)
	// Independent ADR-0119 inventory and ordered outer fields.
	names := []string{"ardents-linux-amd64", "ardents-text-linux-amd64", "ardents-text-reader@.service",
		"ardents-text-publisher@.service", "ardents-text-reader.socket", "ardents-text-publisher.socket",
		"50-ardents-text.rules", "ardents-text.conf", "ardents-endpoint.service"}
	digests := make(map[string]string)
	for _, name := range names {
		if name != "ardents-linux-amd64" {
			files[name] = []byte("resource:" + name)
		}
		sum := sha256.Sum256(files[name])
		digests[name] = hex.EncodeToString(sum[:])
	}
	raw, err := json.Marshal(digests)
	if err != nil {
		t.Fatal(err)
	}
	files["protected-endpoint.json"] = []byte("{\"schema\":\"ardents-protected-endpoint-artifact-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"release\",\"release_version\":1,\"files\":" + string(raw) + "}\n")
	files["ardents-node-linux-amd64"], files["ardents-custody-linux-amd64"] = []byte("node"), []byte("custody")
	r.Scope = Headless
	return r, files
}

func TestProtectedCompleteGroupIsInitialProvenanceOnly(t *testing.T) {
	r, files := protectedFixture(t)
	writeFixture(t, &r, files)
	bundle, err := Verify(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	facts, ok := bundle.Facts()
	if !ok || !facts.Protected || !facts.Headless {
		t.Fatalf("facts: %+v", facts)
	}
	for name, original := range files {
		data, ok := bundle.File(name)
		if !ok || string(data) != string(original) {
			t.Fatalf("missing verified resource %s", name)
		}
	}
}

func TestValidateProtectedGenerationKeepsStaticGrammarWithoutPinAuthority(t *testing.T) {
	_, files := protectedFixture(t)
	resources := make(map[string][]byte)
	for _, name := range protectedNames() {
		resources[name] = files[name]
	}
	if err := ValidateProtectedGeneration(files["protected-endpoint.json"], resources, "release"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProtectedGeneration(files["protected-endpoint.json"], resources, "foreign"); !errors.Is(err, ErrBinding) {
		t.Fatal("foreign release accepted", err)
	}
	resources["foreign"] = []byte("extra")
	if err := ValidateProtectedGeneration(files["protected-endpoint.json"], resources, "release"); !errors.Is(err, ErrInventory) {
		t.Fatal("extra static file accepted", err)
	}
	delete(resources, "foreign")
	resources["ardents-text-linux-amd64"] = []byte("foreign worker")
	if err := ValidateProtectedGeneration(files["protected-endpoint.json"], resources, "release"); !errors.Is(err, ErrBinding) {
		t.Fatal("foreign worker accepted", err)
	}
}

func TestProtectedCanonicalIdentityAndResourceRefusals(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(map[string][]byte)
		want   error
	}{
		{"digest", func(f map[string][]byte) { f["ardents-text.conf"] = []byte("different config") }, ErrBinding},
		{"release", func(f map[string][]byte) {
			f["protected-endpoint.json"] = []byte(strings.ReplaceAll(string(f["protected-endpoint.json"]), "\"release_identity\":\"release\"", "\"release_identity\":\"other\""))
		}, ErrBinding},
		{"platform", func(f map[string][]byte) {
			f["protected-endpoint.json"] = []byte(strings.ReplaceAll(string(f["protected-endpoint.json"]), "\"platform\":\"linux-amd64\"", "\"platform\":\"windows-amd64\""))
		}, ErrBinding},
		{"zero-version", func(f map[string][]byte) {
			f["protected-endpoint.json"] = []byte(strings.ReplaceAll(string(f["protected-endpoint.json"]), "\"release_version\":1", "\"release_version\":0"))
		}, ErrBinding},
		{"duplicate-field", func(f map[string][]byte) {
			f["protected-endpoint.json"] = []byte(strings.ReplaceAll(string(f["protected-endpoint.json"]), "\"release_version\":1", "\"release_version\":1,\"release_version\":1"))
		}, ErrInventory},
		{"unknown-field", func(f map[string][]byte) {
			f["protected-endpoint.json"] = []byte(strings.ReplaceAll(string(f["protected-endpoint.json"]), "\"release_version\":1", "\"release_version\":1,\"unknown\":true"))
		}, ErrInventory},
		{"trailing", func(f map[string][]byte) { f["protected-endpoint.json"] = append(f["protected-endpoint.json"], '\n') }, ErrInventory},
		{"partial", func(f map[string][]byte) { delete(f, "ardents-text-publisher.socket") }, ErrInventory},
		{"oversize", func(f map[string][]byte) { f["protected-endpoint.json"] = []byte(strings.Repeat(" ", (16<<10)+1)) }, ErrInventory},
	} {
		t.Run(test.name, func(t *testing.T) {
			r, files := protectedFixture(t)
			test.change(files)
			writeFixture(t, &r, files)
			if _, err := Verify(context.Background(), r); !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
}
