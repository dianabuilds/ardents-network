//go:build linux

package enrollment

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestForwardingInventoryIndependentCanonicalOracle(t *testing.T) {
	files := map[string][]byte{}
	var pairs []string
	for _, name := range []string{"ardents-forwarding.service", "ardents-node-linux-amd64", "infrastructure-forwarding.json", "root.json", "snapshot.json", "targets.json", "timestamp.json"} {
		files[name] = []byte("forwarding-public:" + name)
		sum := sha256.Sum256(files[name])
		pairs = append(pairs, fmt.Sprintf("%q:%q", name, hex.EncodeToString(sum[:])))
	}
	want := []byte("{\"schema\":\"ardents-infrastructure-enrollment-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"one\",\"release_version\":2,\"invocation\":\"forwarding\",\"files\":{" + strings.Join(pairs, ",") + "}}\n")
	facts := InfrastructureFacts{"linux-amd64", "one", 2, "forwarding"}
	got, err := PrepareInfrastructureInventory(facts, files)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("independent forwarding bytes %q: %v", got, err)
	}
	if _, err := parseInfrastructureInventory(want); err != nil {
		t.Fatal(err)
	}
	observed, hashes, err := ValidateInfrastructureInventory(want, files)
	if err != nil || observed != facts || len(hashes) != 7 {
		t.Fatal("bounded public byte inventory", observed, hashes, err)
	}
	for name, body := range files {
		sum := sha256.Sum256(body)
		if hashes[name] != hex.EncodeToString(sum[:]) {
			t.Fatal("returned digest did not match original bytes", name)
		}
	}
	hashes["ardents-node-linux-amd64"] = strings.Repeat("0", 64)
	_, fresh, err := ValidateInfrastructureInventory(want, files)
	if err != nil || fresh["ardents-node-linux-amd64"] == hashes["ardents-node-linux-amd64"] {
		t.Fatal("caller digest mutation changed a later validation", err)
	}
	originalProgram := files["ardents-node-linux-amd64"]
	files["ardents-node-linux-amd64"] = []byte("substituted original image")
	if got, sums, err := ValidateInfrastructureInventory(want, files); !errors.Is(err, ErrBinding) || got != (InfrastructureFacts{}) || sums != nil {
		t.Fatal("changed actual bytes supplied declarations", got, sums, err)
	}
	files["ardents-node-linux-amd64"] = originalProgram
	for _, mutation := range []struct{ before, after string }{
		{"\"forwarding\"", "\"source\""}, {"\"forwarding\"", "\"issuer\""},
		{"ardents-forwarding.service", "ardents-source.service"},
		{"infrastructure-forwarding.json", "infrastructure-source.json"},
		{"\"files\":", "\"extra\":0,\"files\":"},
	} {
		if _, err := parseInfrastructureInventory([]byte(strings.Replace(string(want), mutation.before, mutation.after, 1))); err == nil {
			t.Fatal("foreign inventory accepted", mutation)
		}
		if got, sums, err := ValidateInfrastructureInventory([]byte(strings.Replace(string(want), mutation.before, mutation.after, 1)), files); err == nil || got != (InfrastructureFacts{}) || sums != nil {
			t.Fatal("foreign inventory supplied declarations", mutation, err)
		}
	}
	delete(files, "ardents-forwarding.service")
	files["ardents-source.service"] = []byte("source")
	if _, err := PrepareInfrastructureInventory(facts, files); err == nil {
		t.Fatal("Source resource substituted in Forwarding inventory")
	}
	files["ardents-forwarding.service"] = []byte("forwarding")
	if _, err := PrepareInfrastructureInventory(facts, files); err == nil {
		t.Fatal("extra inventory accepted")
	}
	delete(files, "ardents-source.service")
	files["infrastructure-forwarding.json"] = bytes.Repeat([]byte("x"), (16<<10)+1)
	if _, err := PrepareInfrastructureInventory(facts, files); err == nil {
		t.Fatal("descriptor bound widened")
	}
	if got, sums, err := ValidateInfrastructureInventory(want, files); !errors.Is(err, ErrInventory) || got != (InfrastructureFacts{}) || sums != nil {
		t.Fatal("oversized descriptor supplied declarations", got, sums, err)
	}
}

func TestInfrastructureInventoryIndependentCanonicalOracle(t *testing.T) {
	files := map[string][]byte{}
	var pairs []string
	for _, name := range []string{"ardents-node-linux-amd64", "ardents-source.service", "infrastructure-source.json", "root.json", "snapshot.json", "targets.json", "timestamp.json"} {
		files[name] = []byte("independent:" + name)
		sum := sha256.Sum256(files[name])
		pairs = append(pairs, fmt.Sprintf("%q:%q", name, hex.EncodeToString(sum[:])))
	}
	want := []byte("{\"schema\":\"ardents-infrastructure-enrollment-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"one\",\"release_version\":1,\"invocation\":\"source\",\"files\":{" + strings.Join(pairs, ",") + "}}\n")
	got, err := PrepareInfrastructureInventory(InfrastructureFacts{"linux-amd64", "one", 1, "source"}, files)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("canonical %s %v", got, err)
	}
	if _, err := parseInfrastructureInventory(want); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{string(want) + "x", strings.Replace(string(want), "source\"", "join\"", 1), strings.Replace(string(want), "\"release_version\":1", "\"release_version\":1,\"release_version\":1", 1), strings.Replace(string(want), "enrollment-v1", "enrollment-v3", 1), strings.Replace(string(want), "\"files\":", "\"foreign\":0,\"files\":", 1)} {
		if _, err := parseInfrastructureInventory([]byte(raw)); err == nil {
			t.Fatal("accepted noncanonical inventory")
		}
	}
	files["foreign"] = []byte("x")
	if _, err := PrepareInfrastructureInventory(InfrastructureFacts{"linux-amd64", "one", 1, "source"}, files); err == nil {
		t.Fatal("extra inventory")
	}
	delete(files, "foreign")
	files["targets.json"] = bytes.Repeat([]byte("x"), (1<<20)+1)
	if _, err := PrepareInfrastructureInventory(InfrastructureFacts{"linux-amd64", "one", 1, "source"}, files); err == nil {
		t.Fatal("widened signed metadata bound")
	}
}

// Inventory hashes alone do not admit a delivery with public native access.
// This reader control creates no Release proof or executable Node authority.
func TestForwardingCandidateRequiresPrivateExecutableDelivery(t *testing.T) {
	for _, scenario := range []struct {
		name string
		mode os.FileMode
		want error
	}{{"private executable", 0700, nil}, {"public executable", 0755, ErrInventory}} {
		t.Run(scenario.name, func(t *testing.T) {
			root := t.TempDir()
			files := make(map[string][]byte)
			for _, name := range []string{"ardents-node-linux-amd64", "ardents-forwarding.service", "infrastructure-forwarding.json", "root.json", "snapshot.json", "targets.json", "timestamp.json"} {
				files[name] = []byte("opaque public candidate input: " + name)
			}
			inventory, err := PrepareInfrastructureInventory(InfrastructureFacts{"linux-amd64", "candidate-access", 2, "forwarding"}, files)
			if err != nil {
				t.Fatal(err)
			}
			files[InfrastructureInventoryName] = inventory
			for name, raw := range files {
				mode := os.FileMode(0600)
				if name == "ardents-node-linux-amd64" {
					mode = scenario.mode
				}
				path := filepath.Join(root, name)
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, mode); err != nil {
					t.Fatal(err)
				}
				info, err := os.Lstat(path)
				if err != nil || info.Mode().Perm() != mode {
					t.Fatal("invalid environment: actual native access control differs", err)
				}
			}
			candidate, err := ReadInfrastructureCandidate(t.Context(), root)
			if !errors.Is(err, scenario.want) {
				t.Fatal("candidate access admission differs", err, scenario.want)
			}
			facts, accepted := candidate.Facts()
			if scenario.want != nil {
				_, exposed := candidate.File("ardents-node-linux-amd64")
				if accepted || exposed {
					t.Fatal("refused public delivery exposed a candidate")
				}
				return
			}
			if !accepted || facts.Invocation != "forwarding" || candidate.Check() != nil {
				t.Fatal("private executable delivery lacks a complete retained candidate")
			}
			for name, want := range files {
				got, found := candidate.File(name)
				if !found || !bytes.Equal(got, want) {
					t.Fatal("private candidate lost an original file", name)
				}
			}
		})
	}
}

func TestJoinInventoryIndependentCanonicalOracle(t *testing.T) {
	files := map[string][]byte{}
	var pairs []string
	for _, name := range []string{"ardents-join.service", "ardents-node-linux-amd64", "infrastructure-join.json", "root.json", "snapshot.json", "targets.json", "timestamp.json"} {
		files[name] = []byte("JOIN public:" + name)
		sum := sha256.Sum256(files[name])
		pairs = append(pairs, fmt.Sprintf("%q:%q", name, hex.EncodeToString(sum[:])))
	}
	want := []byte("{\"schema\":\"ardents-infrastructure-enrollment-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"one\",\"release_version\":2,\"invocation\":\"join\",\"files\":{" + strings.Join(pairs, ",") + "}}\n")
	facts := InfrastructureFacts{"linux-amd64", "one", 2, "join"}
	got, err := PrepareInfrastructureInventory(facts, files)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("independent JOIN inventory %q: %v", got, err)
	}
	observed, hashes, err := ValidateInfrastructureInventory(want, files)
	if err != nil || observed != facts || len(hashes) != 7 {
		t.Fatal("JOIN public inventory", observed, hashes, err)
	}
	for _, role := range []string{"source", "forwarding", "issuer", "descriptor", "introduction"} {
		if _, _, err := ValidateInfrastructureInventory([]byte(strings.Replace(string(want), "\"join\"", fmt.Sprintf("%q", role), 1)), files); err == nil {
			t.Fatal("foreign role inventory accepted", role)
		}
	}
	files["ardents-node-linux-amd64"] = []byte("substituted program")
	if _, _, err := ValidateInfrastructureInventory(want, files); err == nil {
		t.Fatal("substituted JOIN program accepted")
	}
	files["infrastructure-join.json"] = bytes.Repeat([]byte("x"), (16<<10)+1)
	if _, err := PrepareInfrastructureInventory(facts, files); err == nil {
		t.Fatal("JOIN descriptor bound widened")
	}
	files["infrastructure-join.json"] = []byte("JOIN descriptor")
	files["extra"] = []byte("extra")
	if _, err := PrepareInfrastructureInventory(facts, files); err == nil {
		t.Fatal("extra inventory accepted")
	}
	delete(files, "extra")
	delete(files, "ardents-join.service")
	files["ardents-forwarding.service"] = []byte("foreign unit")
	if _, err := PrepareInfrastructureInventory(facts, files); err == nil {
		t.Fatal("Forwarding unit substituted for JOIN")
	}
}
