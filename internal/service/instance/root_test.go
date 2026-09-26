package instance

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInitializedHostRootReopensTheSamePublicRequest(t *testing.T) {
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	config := InitializeConfig{Root: instanceFixtureRoot(t), NetworkID: [32]byte{1}, NotBefore: now, NotAfter: now.Add(time.Hour)}
	root, err := Initialize(config)
	if err != nil {
		t.Fatalf("initialize host Instance root: %v", err)
	}
	request, err := root.Request()
	if err != nil {
		t.Fatalf("read host request: %v", err)
	}
	if err := root.Close(); err != nil {
		t.Fatalf("close initialized root: %v", err)
	}
	view, err := ParseRequest(request)
	if err != nil {
		t.Fatalf("parse public request: %v", err)
	}
	if view.NetworkID != config.NetworkID || view.InstancePublic == [32]byte{} ||
		view.NotBefore != now.Unix() || view.NotAfter != now.Add(time.Hour).Unix() || view.Commitment == [32]byte{} {
		t.Fatalf("public request = %+v", view)
	}
	reopened, err := Open(config.Root)
	if err != nil {
		t.Fatalf("reopen host Instance root: %v", err)
	}
	defer reopened.Close()
	again, err := reopened.Request()
	if err != nil {
		t.Fatalf("read reopened request: %v", err)
	}
	if !bytes.Equal(again, request) {
		t.Fatal("reopened host root changed its public request")
	}
}

func TestLegacyRootBytesAreTypedRefusals(t *testing.T) {
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	config := InitializeConfig{NetworkID: [32]byte{1}, NotBefore: now, NotAfter: now.Add(time.Hour)}

	// A root persisted under the pre-v3 marker refuses both entry paths with
	// the typed sentinel; its bytes stay on disk as refused evidence (ADR-0102).
	legacy := instanceFixtureRoot(t)
	if err := os.WriteFile(filepath.Join(legacy, markerName), []byte(legacyMarker), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(legacy); !errors.Is(err, ErrLegacyRoot) {
		t.Fatalf("Open(pre-v3 marker) = %v, want ErrLegacyRoot", err)
	}
	config.Root = legacy
	if _, err := Initialize(config); !errors.Is(err, ErrLegacyRoot) {
		t.Fatalf("Initialize(pre-v3 marker) = %v, want ErrLegacyRoot", err)
	}

	// A v2-marked root whose state file still carries the v1 schema meets the
	// same typed refusal from the lenient probe before any field decode.
	migrated := instanceFixtureRoot(t)
	if err := os.WriteFile(filepath.Join(migrated, markerName), []byte(marker), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrated, lockName), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrated, stateName), []byte(`{"schema":"ardents-service-instance-root-v1"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(migrated); !errors.Is(err, ErrLegacyRoot) {
		t.Fatalf("Open(v1 state schema) = %v, want ErrLegacyRoot", err)
	}
}

func instanceFixtureRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "instance-root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}
