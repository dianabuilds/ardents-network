package instance

import (
	"bytes"
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
	if view.NetworkID != config.NetworkID || view.InstancePublic == [32]byte{} || view.IntroductionPublic == [32]byte{} ||
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

func instanceFixtureRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "instance-root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}
