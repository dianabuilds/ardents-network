package route

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPartialAcknowledgementFrameStopsAtRouteDeadline(t *testing.T) {
	root := t.TempDir()
	socket := filepath.Join(os.TempDir(), "ara-"+time.Now().Format("150405.000000")+".sock")
	defer os.Remove(socket)
	keyPath := filepath.Join(root, "ack.hex")
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte(hex.EncodeToString(private)), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	stop, completed, err := startAcknowledgement(ctx, socket, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	peer, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	if _, err := peer.Write([]byte("ASIA\x01")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-completed:
		if err == nil {
			t.Fatal("partial acknowledgement frame was accepted")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("partial acknowledgement frame outlived the Route deadline")
	}
}
