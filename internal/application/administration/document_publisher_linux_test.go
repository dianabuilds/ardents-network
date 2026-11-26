//go:build linux

package administration_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/administration"
)

type snapshotPublicationPeer struct {
	mu    sync.Mutex
	calls int
	body  []byte
}

func (*snapshotPublicationPeer) Publish(context.Context) error {
	return errors.New("bodyless publication forbidden")
}
func (*snapshotPublicationPeer) Withdraw(context.Context) error { return errors.New("not selected") }
func (peer *snapshotPublicationPeer) PublishSnapshot(_ context.Context, body []byte) error {
	peer.mu.Lock()
	defer peer.mu.Unlock()
	peer.calls++
	peer.body = bytes.Clone(body)
	return nil
}
func (peer *snapshotPublicationPeer) observed() (int, []byte) {
	peer.mu.Lock()
	defer peer.mu.Unlock()
	return peer.calls, bytes.Clone(peer.body)
}

func TestPublishCommandImportsThenTransfersSnapshotWithoutPath(t *testing.T) {
	binary := buildArdentsNext(t)
	root := t.TempDir()
	path, socket := filepath.Join(root, "document"), filepath.Join(root, "a.sock")
	body := []byte("document\n\u202e")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	peer := &snapshotPublicationPeer{}
	server, err := administration.Listen(socket, peer)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	}()
	command := exec.CommandContext(t.Context(), binary, "publish", socket, path)
	var output, diagnostic bytes.Buffer
	command.Stdout, command.Stderr = &output, &diagnostic
	if err := command.Run(); err != nil || output.Len() != 0 || diagnostic.String() != `{"operation":"application.publish","phase":"command","outcome":"completed"}`+"\n" {
		t.Fatalf("publish: %v, output %q, diagnostic %q", err, &output, &diagnostic)
	}
	if calls, received := peer.observed(); calls != 1 || !bytes.Equal(received, body) {
		t.Fatal("command did not transfer exact document once")
	}
	if err := os.WriteFile(path, []byte{0xff}, 0600); err != nil {
		t.Fatal(err)
	}
	command = exec.CommandContext(t.Context(), binary, "publish", socket, path)
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("invalid document accepted: %q", output)
	}
	if calls, _ := peer.observed(); calls != 1 {
		t.Fatal("invalid document reached publication owner")
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	command = exec.CommandContext(t.Context(), binary, "publish", socket, link)
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("symlink accepted: %q", output)
	}
	if calls, _ := peer.observed(); calls != 1 {
		t.Fatal("symlink reached publication")
	}
}
