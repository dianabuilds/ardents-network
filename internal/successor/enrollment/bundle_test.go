package enrollment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Independently assembled current wire fields, not production encoders or
// predecessor imports. Program bytes are inert for these module tests.
func fixture(t *testing.T) (Request, map[string][]byte) {
	t.Helper()
	files := map[string][]byte{
		"ardents-linux-amd64": []byte("original program"), "ardents-control-linux-amd64": []byte("control program"),
		"1.root.json": []byte("root"), "timestamp.json": []byte("metadata"),
		"catalog.ac1": []byte("catalog"), "catalog.pub": []byte("disclosure"),
		"release.ac1": []byte("release"), "network.ac1": []byte("network"), "compatibility.ac1": []byte("compatibility"),
		"release.pub": []byte("release root"), "network.pub": []byte("network root"), "compatibility.pub": []byte("compatibility root"), "corpus.pub": []byte("corpus"),
	}
	files["RELEASE"] = []byte("schema=ardents-closed-alpha-enrollment-v3\ncohort=cohort\nrelease=release\nplatform=linux-amd64\nenvironment=alpha\nnetwork=network\ntarget_path=ardents/linux-amd64/endpoint\nartifact=ardents-linux-amd64\ntrusted_root=1.root.json\ncontrol_catalog=catalog.ac1\ndisclosure_root=catalog.pub\ncontrol_release=release.ac1\ncontrol_network=network.ac1\ncontrol_compatibility=compatibility.ac1\ncontrol_release_root=release.pub\ncontrol_network_root=network.pub\ncontrol_compatibility_root=compatibility.pub\ncorpus_authority=corpus.pub\ncontrol_artifact=ardents-control-linux-amd64\n")
	root := t.TempDir()
	return Request{BundleRoot: root, ExecutablePath: filepath.Join(root, "ardents-linux-amd64"), Scope: General}, files
}

func writeFixture(t *testing.T, request *Request, files map[string][]byte) {
	t.Helper()
	var names []string
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var manifest strings.Builder
	for _, name := range names {
		sum := sha256.Sum256(files[name])
		manifest.WriteString(hex.EncodeToString(sum[:]) + "  " + name + "\n")
		if err := os.WriteFile(filepath.Join(request.BundleRoot, name), files[name], 0o600); err != nil {
			t.Fatal(err)
		}
	}
	raw := []byte(manifest.String())
	sum := sha256.Sum256(raw)
	request.ManifestSHA256 = hex.EncodeToString(sum[:])
	if err := os.WriteFile(filepath.Join(request.BundleRoot, "SHA256SUMS"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestBundleRetainsOnlyCompleteImmutableInitialProvenance(t *testing.T) {
	r, files := fixture(t)
	writeFixture(t, &r, files)
	bundle, err := Verify(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	facts, ok := bundle.Facts()
	if !ok || facts.ManifestSHA256 != r.ManifestSHA256 || facts.Artifact != "ardents-linux-amd64" || facts.Protected || facts.Headless {
		t.Fatalf("facts: %+v %v", facts, ok)
	}
	data, ok := bundle.File("timestamp.json")
	if !ok || !bytes.Equal(data, files["timestamp.json"]) {
		t.Fatal("missing original bytes")
	}
	data[0] ^= 1
	if err := os.WriteFile(filepath.Join(r.BundleRoot, "timestamp.json"), []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	retained, _ := bundle.File("timestamp.json")
	if !bytes.Equal(retained, files["timestamp.json"]) {
		t.Fatal("snapshot aliased returned or filesystem bytes")
	}
	names := bundle.Names()
	names[0] = "forged"
	if bundle.Names()[0] == "forged" {
		t.Fatal("names alias snapshot")
	}
	if _, ok := (Bundle{}).Facts(); ok {
		t.Fatal("zero value attests provenance")
	}
}

func TestBundleMetadataProjectionKeepsStaticInventoryWithItsOwner(t *testing.T) {
	for _, profile := range []string{"general", "headless-linux", "headless-windows", "protected"} {
		t.Run(profile, func(t *testing.T) {
			r, files := fixture(t)
			if profile == "protected" {
				r, files = protectedFixture(t)
			} else if profile != "general" {
				files["ardents-node-linux-amd64"] = []byte("node")
				files["ardents-custody-linux-amd64"] = []byte("custody")
				r.Scope = Headless
			}
			if profile == "headless-windows" {
				files["RELEASE"] = []byte(strings.ReplaceAll(string(files["RELEASE"]), "linux-amd64", "windows-amd64"))
				for _, command := range []string{"ardents", "ardents-control", "ardents-node", "ardents-custody"} {
					old := command + "-linux-amd64"
					name := command + "-windows-amd64.exe"
					files[name] = files[old]
					delete(files, old)
					files["RELEASE"] = bytes.ReplaceAll(files["RELEASE"], []byte(command+"-windows-amd64\n"), []byte(name+"\n"))
				}
				r.ExecutablePath = filepath.Join(r.BundleRoot, "ardents-windows-amd64.exe")
			}
			files["2.snapshot.json"], files["3.targets.json"] = []byte("snapshot"), []byte("targets")
			writeFixture(t, &r, files)
			bundle, err := Verify(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"2.snapshot.json", "3.targets.json", "timestamp.json"}
			names := bundle.MetadataNames()
			if !reflect.DeepEqual(names, want) {
				t.Fatalf("metadata projection: %v, want %v", names, want)
			}
			names[0] = "forged"
			if !reflect.DeepEqual(bundle.MetadataNames(), want) {
				t.Fatal("projection aliases returned names")
			}
			// Projection must not discard the actual companion bytes needed by
			// another consumer's independent authorization.
			for name, body := range files {
				retained, ok := bundle.File(name)
				if !ok || !bytes.Equal(retained, body) {
					t.Fatalf("projection discarded inventory member %s", name)
				}
			}
		})
	}
	if names := (Bundle{}).MetadataNames(); len(names) != 0 {
		t.Fatal("zero bundle exposes metadata")
	}
}

func TestBundleRefusalStagesReturnNoProvenance(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*testing.T, *Request, map[string][]byte)
		want   error
	}{
		{"retired", func(_ *testing.T, _ *Request, f map[string][]byte) {
			f["RELEASE"] = bytes.Replace(f["RELEASE"], []byte("enrollment-v3"), []byte("enrollment-v2"), 1)
		}, ErrLegacyEnrollmentDescriptor},
		{"unknown-version", func(_ *testing.T, _ *Request, f map[string][]byte) {
			f["RELEASE"] = bytes.Replace(f["RELEASE"], []byte("enrollment-v3"), []byte("enrollment-v4"), 1)
		}, ErrInventory},
		{"duplicate-field", func(_ *testing.T, _ *Request, f map[string][]byte) {
			f["RELEASE"] = append(f["RELEASE"], []byte("cohort=other\n")...)
		}, ErrInventory},
		{"missing-companion", func(_ *testing.T, _ *Request, f map[string][]byte) { delete(f, "corpus.pub") }, ErrInventory},
		{"partial-headless", func(_ *testing.T, _ *Request, f map[string][]byte) { f["ardents-node-linux-amd64"] = []byte("node") }, ErrInventory},
		{"headless-required", func(_ *testing.T, r *Request, _ map[string][]byte) { r.Scope = Headless }, ErrInventory},
		{"partial-protected", func(_ *testing.T, _ *Request, f map[string][]byte) { f["ardents-text.conf"] = []byte("config") }, ErrInventory},
		{"substituted-program", func(_ *testing.T, r *Request, _ map[string][]byte) {
			r.ExecutablePath = filepath.Join(r.BundleRoot, "1.root.json")
		}, ErrBinding},
	} {
		t.Run(test.name, func(t *testing.T) {
			r, files := fixture(t)
			test.mutate(t, &r, files)
			writeFixture(t, &r, files)
			bundle, err := Verify(context.Background(), r)
			if !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
			if _, ok := bundle.Facts(); ok {
				t.Fatal("refusal returned trusted result")
			}
		})
	}
}

func TestBundlePinPrecedesAnyManifestOrDescriptorInterpretation(t *testing.T) {
	r, files := fixture(t)
	files["RELEASE"] = []byte("schema=ardents-closed-alpha-enrollment-v2\n")
	writeFixture(t, &r, files)
	if err := os.WriteFile(filepath.Join(r.BundleRoot, "SHA256SUMS"), []byte("malformed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(context.Background(), r); !errors.Is(err, ErrPin) {
		t.Fatalf("pin ordering: %v", err)
	}
}

func TestRetiredDescriptorRefusesBeforeCompanionInventoryWork(t *testing.T) {
	r, files := fixture(t)
	files["RELEASE"] = bytes.Replace(files["RELEASE"], []byte("enrollment-v3"), []byte("enrollment-v1"), 1)
	writeFixture(t, &r, files)
	if err := os.WriteFile(filepath.Join(r.BundleRoot, "unknown-companion"), []byte("unlisted"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(context.Background(), r); !errors.Is(err, ErrLegacyEnrollmentDescriptor) {
		t.Fatalf("retirement must precede companion inventory: %v", err)
	}
}

func TestBundleActualInventoryAndByteChangesRefuse(t *testing.T) {
	for _, name := range []string{"extra-file", "changed-bytes", "nonregular"} {
		t.Run(name, func(t *testing.T) {
			r, files := fixture(t)
			writeFixture(t, &r, files)
			var err error
			switch name {
			case "extra-file":
				err = os.WriteFile(filepath.Join(r.BundleRoot, "unknown"), []byte("extra"), 0o600)
			case "changed-bytes":
				err = os.WriteFile(filepath.Join(r.BundleRoot, "timestamp.json"), []byte("forged"), 0o600)
			case "nonregular":
				err = os.Remove(filepath.Join(r.BundleRoot, "timestamp.json"))
				if err == nil {
					err = os.Mkdir(filepath.Join(r.BundleRoot, "timestamp.json"), 0o700)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Verify(context.Background(), r); err == nil {
				t.Fatal("accepted changed inventory")
			}
		})
	}
}

type observingContext struct {
	context.Context
	calls, cancelAt int
	action          func(int)
}

func (c *observingContext) Err() error {
	c.calls++
	if c.action != nil {
		c.action(c.calls)
	}
	if c.cancelAt > 0 && c.calls >= c.cancelAt {
		return context.Canceled
	}
	return c.Context.Err()
}

func TestBundleOriginalCancellationAtFinalHandoffRefuses(t *testing.T) {
	r, files := fixture(t)
	writeFixture(t, &r, files)
	observed := &observingContext{Context: context.Background()}
	if _, err := Verify(observed, r); err != nil {
		t.Fatal(err)
	}
	last := observed.calls
	if last < 3 {
		t.Fatal("did not exercise actual I/O")
	}
	canceled := &observingContext{Context: context.Background(), cancelAt: last}
	bundle, err := Verify(canceled, r)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("late original cancel: %v", err)
	}
	if _, ok := bundle.Facts(); ok {
		t.Fatal("late cancellation produced provenance")
	}
}

func TestActualReadRejectsGrowthBeyondEarlierStatBound(t *testing.T) {
	rootPath := t.TempDir()
	path := filepath.Join(rootPath, "file")
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx := &observingContext{Context: context.Background(), action: func(call int) {
		if call == 2 {
			if err := os.WriteFile(path, []byte("abcd"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}}
	data, _, err := readBundleFile(ctx, root, "file", 3)
	if !errors.Is(err, ErrInventory) || data != nil {
		t.Fatalf("actual read bound: %q %v", data, err)
	}
}

func TestActualReadRefusesSameBytesAtReplacementPath(t *testing.T) {
	rootPath := t.TempDir()
	path := filepath.Join(rootPath, "file")
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx := &observingContext{Context: context.Background(), action: func(call int) {
		if call != 2 {
			return
		}
		if err := os.Rename(path, filepath.Join(rootPath, "original")); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
			t.Fatal(err)
		}
	}}
	data, _, err := readBundleFile(ctx, root, "file", 3)
	if !errors.Is(err, ErrBinding) || data != nil {
		t.Fatalf("replacement identity: %q %v", data, err)
	}
}

func TestAuthenticatedNoncanonicalManifestRefuses(t *testing.T) {
	for _, scenario := range []string{"duplicate", "traversal", "uppercase-digest", "missing-newline", "unknown-unlisted"} {
		t.Run(scenario, func(t *testing.T) {
			r, files := fixture(t)
			writeFixture(t, &r, files)
			path := filepath.Join(r.BundleRoot, "SHA256SUMS")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "duplicate":
				first := bytes.IndexByte(raw, '\n')
				raw = append(append(append([]byte(nil), raw[:first+1]...), raw[:first+1]...), raw[first+1:]...)
			case "traversal":
				raw = bytes.Replace(raw, []byte("  1.root.json"), []byte("  ../root.json"), 1)
			case "uppercase-digest":
				raw = append([]byte(strings.ToUpper(string(raw[:64]))), raw[64:]...)
			case "missing-newline":
				raw = raw[:len(raw)-1]
			case "unknown-unlisted":
				if err := os.WriteFile(filepath.Join(r.BundleRoot, "undeclared"), []byte("unknown"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			pin := sha256.Sum256(raw)
			r.ManifestSHA256 = hex.EncodeToString(pin[:])
			if err := os.WriteFile(path, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Verify(context.Background(), r); !errors.Is(err, ErrInventory) {
				t.Fatalf("authenticated %s: %v", scenario, err)
			}
		})
	}
}
