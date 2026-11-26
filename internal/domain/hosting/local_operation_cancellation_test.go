package hosting

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalInitializationCancellationPrecedesEffects(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	root := filepath.Join(t.TempDir(), "budget")
	_, err := ExecuteLocal(ctx, "initialize", CommandPlan{Root: root})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled initialization: %v", err)
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatalf("canceled initialization touched budget: %v", err)
	}
}
