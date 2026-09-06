package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

func TestHeadlessOpenCarriesBytesThroughOnlyTheTargetLinkInterface(t *testing.T) {
	socket := filepath.Join(os.TempDir(), "aho-"+time.Now().Format("150405.000000")+".sock")
	defer os.Remove(socket)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	seen := make(chan struct {
		link, input string
	}, 1)
	go serveHeadlessOpenFixture(listener, seen)
	inputPath, outputPath := filepath.Join(t.TempDir(), "request"), filepath.Join(t.TempDir(), "response")
	if err := os.WriteFile(inputPath, []byte("request bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	targetLink := headlessTargetLink(t)
	var receipt bytes.Buffer
	if err := runHeadlessOpen(t.Context(), socket, targetLink, inputPath, outputPath, &receipt); err != nil {
		t.Fatal(err)
	}
	observed := <-seen
	if observed.link != targetLink || observed.input != "request bytes" {
		t.Fatalf("headless open observed %+v", observed)
	}
	response, err := os.ReadFile(outputPath)
	if err != nil || string(response) != "response bytes" {
		t.Fatalf("headless response = %q, %v", response, err)
	}
	if !bytes.Contains(receipt.Bytes(), []byte("headless-open-complete")) || bytes.Contains(receipt.Bytes(), []byte("target")) {
		t.Fatalf("headless open receipt = %s", receipt.Bytes())
	}
}

func serveHeadlessOpenFixture(listener net.Listener, seen chan<- struct{ link, input string }) {
	connection, err := listener.Accept()
	if err != nil {
		seen <- struct{ link, input string }{}
		return
	}
	defer connection.Close()
	header := make([]byte, 6)
	_, _ = io.ReadFull(connection, header)
	link := make([]byte, int(binary.BigEndian.Uint16(header[4:])))
	_, _ = io.ReadFull(connection, link)
	_, _ = connection.Write([]byte{1})
	var input bytes.Buffer
	for {
		var frame [4]byte
		_, _ = io.ReadFull(connection, frame[:])
		length := binary.BigEndian.Uint32(frame[:])
		if length == 0 {
			break
		}
		_, _ = io.CopyN(&input, connection, int64(length))
	}
	seen <- struct{ link, input string }{string(link), input.String()}
	response := []byte("response bytes")
	var frame [4]byte
	binary.BigEndian.PutUint32(frame[:], uint32(len(response)))
	_, _ = connection.Write(append(frame[:], response...))
	class, reason := []byte("clean service connection close"), []byte("fixture complete")
	terminal := make([]byte, 8+len(class)+len(reason))
	binary.BigEndian.PutUint32(terminal[:4], ^uint32(0))
	binary.BigEndian.PutUint16(terminal[4:6], uint16(len(class)))
	binary.BigEndian.PutUint16(terminal[6:8], uint16(len(reason)))
	copy(terminal[8:], class)
	copy(terminal[8+len(class):], reason)
	_, _ = connection.Write(terminal)
}

func TestHeadlessAdministrationUsesOnlyTheSelectedLocalOperation(t *testing.T) {
	socket := filepath.Join(os.TempDir(), "ahc-"+time.Now().Format("150405.000000")+".sock")
	defer os.Remove(socket)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	request := make(chan string, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			request <- ""
			return
		}
		defer connection.Close()
		raw := make([]byte, 9)
		_, _ = io.ReadFull(connection, raw)
		request <- string(raw)
		_, _ = connection.Write([]byte("withdrawn\n"))
	}()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	var output bytes.Buffer
	if err := runHeadlessAdministration(ctx, "withdraw", socket, &output); err != nil {
		t.Fatal(err)
	}
	if got := <-request; got != "withdraw\n" {
		t.Fatalf("administration request = %q", got)
	}
	if !bytes.Contains(output.Bytes(), []byte("headless-service-withdrawn")) {
		t.Fatalf("administration output = %s", output.Bytes())
	}
}

func TestHeadlessOpenReturnsFailureStatusAndRemovesPartialOutput(t *testing.T) {
	socket := filepath.Join(os.TempDir(), "ahf-"+time.Now().Format("150405.000000")+".sock")
	defer os.Remove(socket)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		header := make([]byte, 6)
		_, _ = io.ReadFull(connection, header)
		link := make([]byte, int(binary.BigEndian.Uint16(header[4:])))
		_, _ = io.ReadFull(connection, link)
		_, _ = connection.Write([]byte{1})
		for {
			var frame [4]byte
			_, _ = io.ReadFull(connection, frame[:])
			length := binary.BigEndian.Uint32(frame[:])
			if length == 0 {
				break
			}
			_, _ = io.CopyN(io.Discard, connection, int64(length))
		}
		class, reason := []byte("abrupt connection loss"), []byte("publisher stopped")
		terminal := make([]byte, 8+len(class)+len(reason))
		binary.BigEndian.PutUint32(terminal[:4], ^uint32(0))
		binary.BigEndian.PutUint16(terminal[4:6], uint16(len(class)))
		binary.BigEndian.PutUint16(terminal[6:8], uint16(len(reason)))
		copy(terminal[8:], class)
		copy(terminal[8+len(class):], reason)
		_, _ = connection.Write(terminal)
	}()
	inputPath, outputPath := filepath.Join(t.TempDir(), "request"), filepath.Join(t.TempDir(), "response")
	if err := os.WriteFile(inputPath, []byte("request"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runHeadlessOpen(t.Context(), socket, headlessTargetLink(t), inputPath, outputPath, io.Discard); err == nil {
		t.Fatal("failed Application terminal returned a successful CLI status")
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("failed Application retained output: %v", err)
	}
}

func TestHeadlessOpenCancellationInterruptsBlockedInputAndRemovesPartialOutput(t *testing.T) {
	socket := filepath.Join(os.TempDir(), fmt.Sprintf("aho-cancel-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() {
		if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
			t.Errorf("remove Unix socket: %v", err)
		}
		if _, err := os.Lstat(socket); !os.IsNotExist(err) {
			t.Errorf("Unix socket residue: %v", err)
		}
	})
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	type peerSetup struct {
		connection *net.UnixConn
		err        error
	}
	peerReady := make(chan peerSetup, 1)
	peerDone := make(chan struct{})
	response := []byte("partial response")
	go func() {
		defer close(peerDone)
		defer close(peerReady)
		connection, acceptErr := listener.AcceptUnix()
		if acceptErr != nil {
			peerReady <- peerSetup{err: acceptErr}
			return
		}
		if err := connection.SetReadBuffer(4 << 10); err != nil {
			peerReady <- peerSetup{err: errors.Join(err, connection.Close())}
			return
		}
		header := make([]byte, 6)
		if _, err := io.ReadFull(connection, header); err != nil {
			peerReady <- peerSetup{err: errors.Join(err, connection.Close())}
			return
		}
		link := make([]byte, int(binary.BigEndian.Uint16(header[4:])))
		if _, err := io.ReadFull(connection, link); err != nil {
			peerReady <- peerSetup{err: errors.Join(err, connection.Close())}
			return
		}
		if _, err := connection.Write([]byte{1}); err != nil {
			peerReady <- peerSetup{err: errors.Join(err, connection.Close())}
			return
		}
		var frame [4]byte
		binary.BigEndian.PutUint32(frame[:], uint32(len(response)))
		if _, err := connection.Write(append(frame[:], response...)); err != nil {
			peerReady <- peerSetup{err: errors.Join(err, connection.Close())}
			return
		}
		if _, err := io.ReadFull(connection, frame[:]); err != nil {
			peerReady <- peerSetup{err: errors.Join(err, connection.Close())}
			return
		}
		length := binary.BigEndian.Uint32(frame[:])
		if length == 0 {
			peerReady <- peerSetup{err: errors.Join(errors.New("headless input closed before backpressure"), connection.Close())}
			return
		}
		peerReady <- peerSetup{connection: connection}
	}()
	t.Cleanup(func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("close Unix listener: %v", err)
		}
		select {
		case <-peerDone:
		case <-time.After(time.Second):
			t.Error("headless peer goroutine did not stop")
		}
		for setup := range peerReady {
			if setup.err != nil {
				t.Errorf("abandoned headless peer setup: %v", setup.err)
			}
			if setup.connection != nil {
				if err := setup.connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
					t.Errorf("close abandoned headless peer: %v", err)
				}
			}
		}
	})
	inputPath, outputPath := filepath.Join(t.TempDir(), "request"), filepath.Join(t.TempDir(), "response")
	if err := os.WriteFile(inputPath, make([]byte, 8<<20), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		result <- runHeadlessOpen(ctx, socket, headlessTargetLink(t), inputPath, outputPath, io.Discard)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-runDone:
		case <-time.After(time.Second):
			t.Error("headless open goroutine did not stop")
		}
	})
	var peer *net.UnixConn
	select {
	case setup := <-peerReady:
		if setup.err != nil || setup.connection == nil {
			t.Fatalf("headless peer setup: %v", setup.err)
		}
		peer = setup.connection
	case <-time.After(time.Second):
		t.Fatal("headless peer setup did not complete")
	}
	t.Cleanup(func() {
		if err := peer.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("close headless peer: %v", err)
		}
	})
	awaitPartialOutput(t, outputPath, response)
	select {
	case err := <-result:
		t.Fatalf("headless open completed before cancellation: %v", err)
	default:
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("headless cancellation = %v", err)
		}
	case <-time.After(time.Second):
		if err := peer.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("emergency close headless peer: %v", err)
		}
		t.Fatal("headless cancellation waited for the non-reading peer")
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("canceled headless open retained partial output: %v", err)
	}
}

func TestHeadlessOpenCancellationInterruptsConnectionSetup(t *testing.T) {
	socket := filepath.Join(os.TempDir(), fmt.Sprintf("aho-setup-cancel-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() { _ = os.Remove(socket) })
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	peerReady := make(chan *net.UnixConn, 1)
	go func() {
		peer, acceptErr := listener.AcceptUnix()
		if acceptErr != nil {
			return
		}
		header := make([]byte, 6)
		if _, readErr := io.ReadFull(peer, header); readErr != nil {
			_ = peer.Close()
			return
		}
		link := make([]byte, int(binary.BigEndian.Uint16(header[4:])))
		if _, readErr := io.ReadFull(peer, link); readErr != nil {
			_ = peer.Close()
			return
		}
		peerReady <- peer
	}()
	inputPath, outputPath := filepath.Join(t.TempDir(), "request"), filepath.Join(t.TempDir(), "response")
	if err := os.WriteFile(inputPath, []byte("request"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- runHeadlessOpen(ctx, socket, headlessTargetLink(t), inputPath, outputPath, io.Discard)
	}()
	var peer *net.UnixConn
	select {
	case peer = <-peerReady:
		defer peer.Close()
	case <-time.After(time.Second):
		t.Fatal("headless open did not send its setup request")
	}
	cancel()
	select {
	case runErr := <-result:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("headless setup cancellation = %v", runErr)
		}
	case <-time.After(time.Second):
		t.Fatal("headless open waited for setup status after cancellation")
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("canceled setup retained output: %v", err)
	}
	if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var one [1]byte
	if _, err := peer.Read(one[:]); err == nil {
		t.Fatal("headless setup cancellation left the local transport open")
	}
}

func awaitPartialOutput(t *testing.T, path string, want []byte) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		got, err := os.ReadFile(path)
		if err == nil && bytes.Equal(got, want) {
			return
		}
		runtime.Gosched()
	}
	t.Fatal("headless open did not write the peer response prefix")
}

func headlessTargetLink(t *testing.T) string {
	t.Helper()
	link, err := targetlink.Encode(targetlink.Link{Network: [32]byte{1}, Target: [32]byte{2}})
	if err != nil {
		t.Fatal(err)
	}
	return link
}
