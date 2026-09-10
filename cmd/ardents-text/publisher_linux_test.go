//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev1/administration"
)

type publicationPeer struct {
	calls atomic.Int32
	body  []byte
}

func (*publicationPeer) Publish(context.Context) error {
	return errors.New("bodyless publication forbidden")
}
func (*publicationPeer) Withdraw(context.Context) error { return errors.New("not selected") }
func (peer *publicationPeer) PublishSnapshot(_ context.Context, body []byte) error {
	peer.calls.Add(1)
	peer.body = bytes.Clone(body)
	return nil
}
func TestPublishCommandImportsThenTransfersSnapshotWithoutPath(t *testing.T) {
	root := t.TempDir()
	path, socket := filepath.Join(root, "document"), filepath.Join(root, "a.sock")
	body := []byte("document\n\u202e")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	peer := &publicationPeer{}
	server, err := administration.Listen(socket, peer)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	if err := run([]string{"publish", socket, path}); err != nil {
		t.Fatal(err)
	}
	if peer.calls.Load() != 1 || !bytes.Equal(peer.body, body) {
		t.Fatal("command did not transfer exact document once")
	}
	if err := os.WriteFile(path, []byte{0xff}, 0600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"publish", socket, path}); err == nil || peer.calls.Load() != 1 {
		t.Fatal("invalid document reached publication owner")
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"publish", socket, link}); err == nil || peer.calls.Load() != 1 {
		t.Fatal("symlink reached publication")
	}
}
