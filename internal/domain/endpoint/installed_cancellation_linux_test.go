//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Cancel the genuine parent at the boundary between CLI validation and the
// participant's own pre-input check, without supplying accepting Startup facts.
type installedOpeningCancellation struct {
	context.Context
	cancel context.CancelFunc
	checks int
}

func (c *installedOpeningCancellation) Err() error {
	c.checks++
	if c.checks == 2 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestInstalledCancellationAtParticipantEntryIsNotSuccessfulStartup(t *testing.T) {
	parent, cancel := context.WithCancel(t.Context())
	defer cancel()
	ctx := &installedOpeningCancellation{Context: parent, cancel: cancel}
	root := filepath.Join(t.TempDir(), "absent-installation")
	var diagnostic bytes.Buffer
	if code := RunInstalledCommand(ctx, []string{"start-installed", root}, &diagnostic); code != 1 {
		t.Fatalf("canceled pre-start was successful: code=%d", code)
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("canceled entry touched installation", err)
	}
	var event map[string]string
	if err := json.Unmarshal(diagnostic.Bytes(), &event); err != nil || event["phase"] != "installed-startup" || event["outcome"] != "unavailable" || event["operation"] != "endpoint.start-installed" || len(event) != 3 {
		t.Fatal("pre-start refusal projection", event, err)
	}
}
