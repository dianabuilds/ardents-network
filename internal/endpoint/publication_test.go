package endpoint

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
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
	if _, err := ReadControl(ctx, connection, 8); err == nil {
		t.Fatal("partial administration frame was accepted")
	}
	if time.Since(started) > 500*time.Millisecond {
		t.Fatal("partial administration frame outlived the operation deadline")
	}
}

func TestWithdrawalAdministrationOwnsAnExplicitTerminalOperation(t *testing.T) {
	socket := filepath.Join(os.TempDir(), "awd-"+time.Now().Format("150405.000000")+".sock")
	defer os.Remove(socket)
	principal, capability := [32]byte{1}, [32]byte{2}
	fixture := &withdrawalAdministrationFixture{principal: principal, capability: capability}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := withdrawCurrent(ctx, fixture, func(string, int) uint32 { return 0 }, socket, principal, time.Now(), time.Second)
		done <- err
	}()
	var connection net.Conn
	var err error
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		connection, err = (&net.Dialer{}).DialContext(ctx, "unix", socket)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connection.Write([]byte("withdraw\n")); err != nil {
		t.Fatal(err)
	}
	if err := connection.(*net.UnixConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	response, err := io.ReadAll(connection)
	_ = connection.Close()
	if err != nil || string(response) != "withdrawn\n" {
		t.Fatalf("withdrawal response = %q, %v", response, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !fixture.withdrawn {
		t.Fatal("withdrawal operation did not reach the Service Administration owner")
	}
}

func TestWithdrawalPartialFrameStopsAtItsLifetime(t *testing.T) {
	socket := filepath.Join(os.TempDir(), "awp-"+time.Now().Format("150405.000000")+".sock")
	defer os.Remove(socket)
	principal, capability := [32]byte{1}, [32]byte{2}
	fixture := &withdrawalAdministrationFixture{principal: principal, capability: capability}
	done := make(chan error, 1)
	go func() {
		_, err := withdrawCurrent(context.Background(), fixture, func(string, int) uint32 { return 0 }, socket,
			principal, time.Now(), 50*time.Millisecond)
		done <- err
	}()
	var connection net.Conn
	var err error
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		connection, err = (&net.Dialer{}).DialContext(t.Context(), "unix", socket)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if _, err := connection.Write([]byte("with")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("partial withdrawal frame was accepted")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("partial withdrawal frame outlived its operation lifetime")
	}
}

type withdrawalAdministrationFixture struct {
	principal, capability [32]byte
	withdrawn             bool
}

func (fixture *withdrawalAdministrationFixture) Admit(principal [32]byte, surface broker.Surface) ([32]byte, error) {
	if principal != fixture.principal || surface != broker.Administration {
		return [32]byte{}, errors.New("unexpected administration admission")
	}
	return fixture.capability, nil
}

func (fixture *withdrawalAdministrationFixture) Withdraw(_ context.Context, request WithdrawalRequest) (WithdrawalResult, error) {
	if request.Principal != fixture.principal || request.Capability != fixture.capability {
		return WithdrawalResult{}, errors.New("unexpected withdrawal authority")
	}
	fixture.withdrawn = true
	return WithdrawalResult{Class: "unpublished", AuthenticatedTarget: [32]byte{3}, Generation: 1}, nil
}
