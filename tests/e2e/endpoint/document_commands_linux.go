//go:build linux && document_worker_installed

package endpoint

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/administration"
	"github.com/dianabuilds/ardents-network/internal/publication"
)

// VerifyInstalledDocumentCommands drives the pinned CLI through publish, Link, read and withdrawal.
func VerifyInstalledDocumentCommands(t *testing.T, ctx context.Context, readSocket, publishSocket string, snapshot []byte, network, target [32]byte, awaitRefresh func()) {
	t.Helper()
	input := filepath.Join(filepath.Dir(publishSocket), "document")
	if err := os.WriteFile(input, snapshot, 0600); err != nil {
		t.Fatal(err)
	}
	if output := installedDocumentCLI(t, ctx, nil, 0, "publish", publishSocket, input); len(output) != 0 {
		t.Fatal("publish wrote stdout")
	}
	if awaitRefresh != nil {
		awaitRefresh()
	}
	link := installedDocumentCLI(t, ctx, nil, 514, "link", publishSocket)
	if len(link) < 2 || link[len(link)-1] != '\n' || bytes.Count(link, []byte{'\n'}) != 1 {
		t.Fatal("link did not return one line")
	}
	linkNetwork, linkTarget, err := publication.ParseTargetLink(string(link[:len(link)-1]))
	if err != nil || linkNetwork != network || linkTarget != target {
		t.Fatal("link did not identify the original authenticated Service", err)
	}
	actual := installedDocumentCLI(t, ctx, link, len(snapshot), "read", readSocket)
	if !bytes.Equal(actual, snapshot) {
		t.Fatalf("read returned different bytes, got length %d, wanted %d", len(actual), len(snapshot))
	}
	outcome, err := administration.Request(ctx, publishSocket, administration.Withdraw)
	if err != nil || outcome != administration.Withdrawn {
		t.Fatal("publication did not withdraw and join", err)
	}
	if _, err := administration.RequestPublishedLink(ctx, publishSocket); err == nil {
		t.Fatal("retired Publisher retained a current Link")
	}
	if outcome, err := administration.Request(ctx, publishSocket, administration.Withdraw); outcome != "" || err == nil || err.Error() != "local Service Administration request failed" {
		t.Fatalf("repeated withdrawal must refuse through Administration: %q, %v", outcome, err)
	}
}

type installedDocumentOutput struct {
	body      bytes.Buffer
	remaining int
	cancel    context.CancelFunc
}

func (output *installedDocumentOutput) Write(body []byte) (int, error) {
	if len(body) > output.remaining {
		output.cancel()
		return 0, io.ErrShortBuffer
	}
	output.remaining -= len(body)
	return output.body.Write(body)
}

func installedDocumentCLI(t *testing.T, parent context.Context, input []byte, limit int, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(parent, 50*time.Second)
	defer cancel()
	stdout, stderr := &installedDocumentOutput{remaining: limit, cancel: cancel}, &installedDocumentOutput{remaining: 4096, cancel: cancel}
	command := exec.CommandContext(ctx, RecoveryPinnedCommand(t, "ardents-next", "ARDENTS_RECOVERY_INSTANCE_SHA256"), args...)
	command.WaitDelay = 5 * time.Second
	command.Stdin, command.Stdout, command.Stderr = bytes.NewReader(input), stdout, stderr
	if err := errors.Join(command.Run(), ctx.Err()); err != nil || stderr.body.Len() != 0 {
		t.Fatalf("installed document %s failed: %v, diagnostic %q", args[0], err, stderr.body.String())
	}
	return stdout.body.Bytes()
}
