package serviceendpoint

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/applicationipc"
)

func TestPartialAdministrationFrameStopsAtOperationDeadline(t *testing.T) {
	socket := filepath.Join(os.TempDir(), "asa-"+time.Now().Format("150405.000000")+".sock")
	defer os.Remove(socket)
	listener, err := listenLocal(socket, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan *net.UnixConn, 1)
	go func() { connection, _ := listener.AcceptUnix(); accepted <- connection }()
	peer, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	connection := <-accepted
	defer connection.Close()
	if _, err := peer.Write([]byte("pub")); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := applicationipc.ReadControl(ctx, connection, 8); err == nil {
		t.Fatal("partial administration frame was accepted")
	}
	if time.Since(started) > 500*time.Millisecond {
		t.Fatal("partial administration frame outlived the operation deadline")
	}
}
