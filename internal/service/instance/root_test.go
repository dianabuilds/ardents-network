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

func TestOldRootBytesAreRefusedWithoutMutation(t *testing.T) {
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	config := InitializeConfig{NetworkID: [32]byte{1}, NotBefore: now, NotAfter: now.Add(time.Hour)}

	// A root with an unsupported marker refuses both entry paths without
	// changing the marker bytes.
	legacy := instanceFixtureRoot(t)
	oldMarker := []byte("ardents-service-instance-root-v2\n")
	if err := os.WriteFile(filepath.Join(legacy, markerName), oldMarker, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(legacy); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Open(old marker) = %v, want ErrInvalid", err)
	}
	config.Root = legacy
	if _, err := Initialize(config); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Initialize(old marker) = %v, want ErrInvalid", err)
	}
	retained, err := os.ReadFile(filepath.Join(legacy, markerName))
	if err != nil || !bytes.Equal(retained, oldMarker) {
		t.Fatalf("old marker changed: %v", err)
	}

	// A v3-marked root whose state file carries the old schema is invalid.
	migrated := instanceFixtureRoot(t)
	if err := os.WriteFile(filepath.Join(migrated, markerName), []byte(marker), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrated, lockName), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrated, stateName), []byte(`{"schema":"ardents-service-instance-root-v2"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(migrated); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Open(old state schema) = %v, want ErrInvalid", err)
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
