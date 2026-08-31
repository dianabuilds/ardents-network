package administration

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalAdministrationDispatchesOnlyClosedOperations(t *testing.T) {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("aa-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() { _ = os.Remove(path) })
	published, withdrawn := make(chan struct{}, 1), make(chan struct{}, 1)
	server, err := Listen(path, InterfaceFuncs{
		PublishFunc:  func(context.Context) error { published <- struct{}{}; return nil },
		WithdrawFunc: func(context.Context) error { withdrawn <- struct{}{}; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	if outcome, err := Request(t.Context(), path, Publish); err != nil || outcome != Published {
		t.Fatalf("publish = %q, %v", outcome, err)
	}
	<-published
	if outcome, err := Request(t.Context(), path, Withdraw); err != nil || outcome != Withdrawn {
		t.Fatalf("withdraw = %q, %v", outcome, err)
	}
	<-withdrawn
	if _, err := Request(t.Context(), path, Operation("route")); err == nil {
		t.Fatal("unknown Administration operation was accepted")
	}
	raw, err := (&net.Dialer{}).DialContext(t.Context(), "unix", path)
	if err != nil {
		t.Fatal(err)
	}
	connection := raw.(*net.UnixConn)
	_, _ = connection.Write([]byte("publish\nsurplus"))
	_ = connection.CloseWrite()
	response := make([]byte, 12)
	read, _ := connection.Read(response)
	_ = connection.Close()
	if string(response[:read]) != "unavailable\n" {
		t.Fatalf("surplus response = %q", response[:read])
	}
}
