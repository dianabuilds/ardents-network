package endpoint

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIntroductionControlReadStopsAtOperationDeadline(t *testing.T) {
	socket := filepath.Join(os.TempDir(), "asi-"+time.Now().Format("150405.000000")+".sock")
	defer os.Remove(socket)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() { connection, _ := listener.Accept(); accepted <- connection }()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, requestErr := requestIntroductionAcknowledgement(ctx, socket, Credential{
			Target: [32]byte{1}, Generation: 1, NotAfter: time.Now().Add(time.Minute).Unix(), NetworkID: [32]byte{2}},
			[32]byte{3}, newResourceObserver())
		done <- requestErr
	}()
	connection := <-accepted
	defer connection.Close()
	if err := <-done; err == nil {
		t.Fatal("stalled Introduction control peer outlived the operation deadline")
	}
}
