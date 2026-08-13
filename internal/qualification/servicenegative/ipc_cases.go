package servicenegative

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/applicationipc"
)

func malformedIPCFrame(context.Context) bool {
	return controlFrameRejected([]byte("wrong!!!"), false, 100*time.Millisecond)
}

func oversizedIPCFrame(context.Context) bool {
	return controlFrameRejected([]byte("publish\n!"), false, 100*time.Millisecond)
}

func partialIPCFrame(context.Context) bool {
	return controlFrameRejected([]byte("pub"), false, 100*time.Millisecond)
}

func slowIPCFrame(context.Context) bool {
	return controlFrameRejected([]byte("pub"), true, 50*time.Millisecond)
}

func controlFrameRejected(payload []byte, keepOpen bool, deadline time.Duration) bool {
	random := make([]byte, 4)
	if _, err := rand.Read(random); err != nil {
		return false
	}
	socket := filepath.Join(os.TempDir(), "asn-"+hex.EncodeToString(random)+".sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		return false
	}
	defer listener.Close()
	defer os.Remove(socket)
	accepted := make(chan *net.UnixConn, 1)
	go func() { connection, _ := listener.AcceptUnix(); accepted <- connection }()
	peer, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		return false
	}
	defer peer.Close()
	connection := <-accepted
	defer connection.Close()
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	result := make(chan bool, 1)
	go func() {
		raw, readErr := applicationipc.ReadControl(ctx, connection, 8)
		result <- readErr != nil || !bytes.Equal(raw, []byte("publish\n"))
	}()
	if _, err := peer.Write(payload); err != nil {
		return false
	}
	if !keepOpen {
		if err := peer.CloseWrite(); err != nil {
			return false
		}
	}
	select {
	case rejected := <-result:
		return rejected
	case <-time.After(time.Second):
		return false
	}
}
