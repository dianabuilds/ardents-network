package nativecircuit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRoleRejectsKnowledgeOutsideItsFixedView(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	configPath := filepath.Join(root, "role.json")
	evidenceDir := filepath.Join(root, "evidence")
	if err := os.Mkdir(evidenceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	config := `{"schema_version":"carrier-lab-native-role/v1","run_id":"20260810T140000Z-native","role":"user-entry","listen_address":"127.0.0.1:1","certificate_path":"node.pem","private_key_path":"node.key","allowed_next":["user-interior:37001"],"target_root_path":"forbidden.pem"}`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RunRole(context.Background(), configPath, evidenceDir); err == nil {
		t.Fatal("User Entry accepted Target knowledge")
	}
}

func TestRoleRejectsUnknownConfigurationField(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	configPath := filepath.Join(root, "role.json")
	evidenceDir := filepath.Join(root, "evidence")
	if err := os.Mkdir(evidenceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	config := `{"schema_version":"carrier-lab-native-role/v1","run_id":"20260810T140100Z-native","role":"rendezvous","listen_address":"127.0.0.1:1","certificate_path":"node.pem","private_key_path":"node.key","service_target":"forbidden"}`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RunRole(context.Background(), configPath, evidenceDir); err == nil {
		t.Fatal("Rendezvous accepted an undeclared configuration field")
	}
}
