package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
)

func TestNetworkSigningInitialKeyArguments(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	parent := t.TempDir()
	if c := runInitialKeys(ctx, []string{"prepare-initial-keys", filepath.Join(parent, "plan"), filepath.Join(parent, "destination")}, &bytes.Buffer{}); c != 130 {
		t.Fatal(c)
	}
	if c := runInitialKeys(context.Background(), []string{"prepare-initial-keys", "relative", "/destination"}, &bytes.Buffer{}); c != 2 {
		t.Fatal(c)
	}
}
