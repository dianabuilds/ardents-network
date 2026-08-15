package tooling

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStage4ImpairedShapingConfigurationIsFrozen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tool.json")
	raw := []byte(`{"schema_version":"carrier-lab-native-tool-role/v1","run_id":"s43-test-1","role":"shape-client","mode":"shape","profile":"h3-s43-impaired-v1","seed":17}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := readNativeToolConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"qdisc", "replace", "dev", "eth0", "root", "netem", "limit", "1000",
		"delay", "150ms", "60.8ms", "distribution", "normal", "loss", "random", "5%", "seed", "17",
		"rate", "25mbit"}
	if got := nativeShapingArguments(config, "eth0"); !reflect.DeepEqual(got, want) {
		t.Fatalf("S4.3 qdisc arguments=%v; want %v", got, want)
	}
}

func TestStage4ImpairedShapingRequiresSeedAndExactProfile(t *testing.T) {
	for name, raw := range map[string]string{
		"missing seed":    `{"schema_version":"carrier-lab-native-tool-role/v1","run_id":"s43-test-1","role":"shape-client","mode":"shape","profile":"h3-s43-impaired-v1"}`,
		"unknown profile": `{"schema_version":"carrier-lab-native-tool-role/v1","run_id":"s43-test-1","role":"shape-client","mode":"shape","profile":"other","seed":17}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tool.json")
			if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := readNativeToolConfig(path); err == nil {
				t.Fatal("invalid S4.3 shaping configuration was accepted")
			}
		})
	}
}
