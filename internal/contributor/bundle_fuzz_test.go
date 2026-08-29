package contributor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func FuzzContributorJSONDecoders(f *testing.F) {
	for _, seed := range [][]byte{
		{}, []byte(`{}`), []byte(`{"schema":"unknown-v9"}`),
		[]byte(`{"schema":"ardents-node-plan-v1","sources":[]}`),
		[]byte(`{"schema":"ardents-contributor-installation-v1","generation":1}`),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		_ = validateProfilePlan(raw)
		for _, target := range []any{&bundleManifest{}, &profileNodePlan{}, &installationRecord{}, &installationMarker{}, &lifecycleEvent{}} {
			if decodeStrict(raw, target) != nil {
				continue
			}
			encoded, err := json.Marshal(target)
			if err != nil {
				t.Fatal(err)
			}
			clone := reflect.New(reflect.TypeOf(target).Elem()).Interface()
			if err := decodeStrict(encoded, clone); err != nil || !reflect.DeepEqual(target, clone) {
				t.Fatalf("strict JSON round trip failed for %T: %v", target, err)
			}
		}
	})
}

func TestContributorPersistedAndBundleDecodersRejectUnknownVersions(t *testing.T) {
	directory := t.TempDir()
	manifest := []byte(`{"schema":"ardents-contributor-bundle-v9","profile":"h4-5-rendezvous-alpha-v1","deployment_id":"` +
		"1111111111111111111111111111111111111111111111111111111111111111" + `","generation":1,"files":{}}`)
	if err := os.WriteFile(filepath.Join(directory, "manifest.json"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	pin := sha256.Sum256(manifest)
	if _, err := openBundle(directory, hex.EncodeToString(pin[:])); err == nil {
		t.Fatal("unknown bundle version was accepted")
	}
	for name, read := range map[string]func(string) error{
		"installation": func(path string) error { _, err := readInstallation(path); return err },
		"marker":       func(path string) error { _, err := readInstallationMarker(path); return err },
		"lifecycle":    func(path string) error { _, err := readLifecycle(path); return err },
	} {
		path := filepath.Join(directory, name+".json")
		if err := os.WriteFile(path, []byte(`{"schema":"unknown-v9","kind":"lifecycle","state":"READY"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := read(path); err == nil {
			t.Fatalf("unknown %s version was accepted", name)
		}
	}
}
