//go:build browsercompat

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCarrierRelayForwardsOpaqueBytesWithoutTerminatingTLS(t *testing.T) {
	upstream := listenCarrierRelayTest(t)
	relay, err := startCarrierRelay("127.0.0.1:0", upstream.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { relay.close() })

	client, err := net.Dial("tcp", relay.endpoint())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server := acceptCarrierRelayTest(t, upstream)
	defer server.Close()

	clientBytes := []byte{0x16, 0x03, 0x03, 0x00, 0x07, 0x00, 0xff, 0x01, 0x02, 0x03, 0x00, 0x04}
	serverBytes := []byte{0x17, 0x03, 0x03, 0x00, 0x05, 0xfe, 0x00, 0xfd, 0x00, 0xfc}
	if _, err := client.Write(clientBytes); err != nil {
		t.Fatal(err)
	}
	if got := readCarrierRelayTest(t, server, len(clientBytes)); !bytes.Equal(got, clientBytes) {
		t.Fatalf("client-to-Node bytes = %x", got)
	}
	if _, err := server.Write(serverBytes); err != nil {
		t.Fatal(err)
	}
	if got := readCarrierRelayTest(t, client, len(serverBytes)); !bytes.Equal(got, serverBytes) {
		t.Fatalf("Node-to-client bytes = %x", got)
	}
	waitCarrierRelayTest(t, func() bool {
		snapshot := relay.snapshot()
		return snapshot.ClientToNodeBytes == uint64(len(clientBytes)) && snapshot.NodeToClientBytes == uint64(len(serverBytes))
	})
	if snapshot := relay.snapshot(); snapshot.ActiveBridges != 1 || snapshot.ClientToNodeBytes != uint64(len(clientBytes)) ||
		snapshot.NodeToClientBytes != uint64(len(serverBytes)) {
		t.Fatalf("transparent relay snapshot = %+v", snapshot)
	}
}

func TestCarrierRelayResetClosesAllActiveBridgesAndKeepsListener(t *testing.T) {
	upstream := listenCarrierRelayTest(t)
	relay, err := startCarrierRelay("127.0.0.1:0", upstream.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { relay.close() })

	clients := make([]net.Conn, 2)
	servers := make([]net.Conn, 2)
	for index := range clients {
		clients[index], err = net.Dial("tcp", relay.endpoint())
		if err != nil {
			t.Fatal(err)
		}
		servers[index] = acceptCarrierRelayTest(t, upstream)
		payload := []byte{byte(index + 1)}
		if _, err := clients[index].Write(payload); err != nil {
			t.Fatal(err)
		}
		if got := readCarrierRelayTest(t, servers[index], len(payload)); !bytes.Equal(got, payload) {
			t.Fatalf("bridge %d readiness bytes = %x", index, got)
		}
	}
	waitCarrierRelayTest(t, func() bool { return relay.snapshot().ActiveBridges == 2 })
	reset, err := relay.reset()
	if err != nil {
		t.Fatal(err)
	}
	if reset.Schema != "ardents-h4-8-a11-carrier-relay-reset-v1" || reset.ResetCount != 1 || reset.ResetBridges != 2 ||
		reset.ActiveBefore != 2 || reset.SelectedBridgeID != 0 || reset.ActiveBridges != 0 || !reset.ListenerLive {
		t.Fatalf("carrier reset receipt = %+v", reset)
	}
	for _, client := range clients {
		requireCarrierRelayClosed(t, client)
	}

	fresh, err := net.Dial("tcp", relay.endpoint())
	if err != nil {
		t.Fatalf("relay listener did not survive reset: %v", err)
	}
	server := acceptCarrierRelayTest(t, upstream)
	_ = fresh.Close()
	_ = server.Close()
	for index := range clients {
		_ = clients[index].Close()
		_ = servers[index].Close()
	}
}

func TestCarrierRelayResetRefusesNonFrozenActiveTopology(t *testing.T) {
	upstream := listenCarrierRelayTest(t)
	relay, err := startCarrierRelay("127.0.0.1:0", upstream.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { relay.close() })

	client, err := net.Dial("tcp", relay.endpoint())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server := acceptCarrierRelayTest(t, upstream)
	defer server.Close()
	if _, err := client.Write([]byte{0x01}); err != nil {
		t.Fatal(err)
	}
	_ = readCarrierRelayTest(t, server, 1)
	if _, err := relay.reset(); err == nil || !strings.Contains(err.Error(), "exactly two active bridges") {
		t.Fatalf("one-bridge reset error = %v", err)
	}
	if snapshot := relay.snapshot(); snapshot.ResetCount != 0 || snapshot.ActiveBridges != 1 {
		t.Fatalf("refused reset mutated relay = %+v", snapshot)
	}
}

func TestCarrierRelayRoleWritesBoundedReadyResetAndFinalReceipts(t *testing.T) {
	upstream := listenCarrierRelayTest(t)
	listen := availableCarrierRelayTestEndpoint(t)
	root := t.TempDir()
	input := config{Deadline: time.Now().Add(5 * time.Second).UTC().Format(time.RFC3339), CompletePath: filepath.Join(root, "complete"),
		CarrierRelayListenAddress: listen, CarrierRelayTargetAddress: upstream.Addr().String(),
		CarrierRelayReadyPath: filepath.Join(root, "carrier-relay-ready.json"), CarrierRelayResetPath: filepath.Join(root, "carrier-relay-reset"),
		CarrierRelayResetResultPath: filepath.Join(root, "carrier-relay-reset.json")}
	completed := make(chan struct {
		result carrierRelaySnapshot
		err    error
	}, 1)
	go func() {
		result, err := serveCarrierRelay(input)
		completed <- struct {
			result carrierRelaySnapshot
			err    error
		}{result, err}
	}()
	waitCarrierRelayFile(t, input.CarrierRelayReadyPath)
	var ready struct {
		Schema, Listen, Target string
		PID                    int
	}
	readCarrierRelayJSON(t, input.CarrierRelayReadyPath, &ready)
	if ready.Schema != "ardents-h4-8-a11-carrier-relay-ready-v1" || ready.Listen != listen || ready.Target != upstream.Addr().String() || ready.PID <= 0 {
		t.Fatalf("carrier relay ready receipt = %+v", ready)
	}

	clients := make([]net.Conn, 2)
	servers := make([]net.Conn, 2)
	for index := range clients {
		var err error
		clients[index], err = net.Dial("tcp", listen)
		if err != nil {
			t.Fatal(err)
		}
		servers[index] = acceptCarrierRelayTest(t, upstream)
	}
	for index := range clients {
		payload := []byte{byte(index + 1)}
		if _, err := clients[index].Write(payload); err != nil {
			t.Fatal(err)
		}
		if got := readCarrierRelayTest(t, servers[index], len(payload)); !bytes.Equal(got, payload) {
			t.Fatalf("bridge %d readiness bytes = %x", index, got)
		}
	}
	if err := os.WriteFile(input.CarrierRelayResetPath, []byte("reset\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitCarrierRelayFile(t, input.CarrierRelayResetResultPath)
	var reset carrierRelayResetReceipt
	readCarrierRelayJSON(t, input.CarrierRelayResetResultPath, &reset)
	if reset.ResetBridges != 2 || reset.ActiveBefore != 2 || reset.SelectedBridgeID != 0 ||
		reset.ActiveBridges != 0 || !reset.ListenerLive {
		t.Fatalf("carrier relay reset file = %+v", reset)
	}
	for index := range clients {
		_ = clients[index].Close()
		_ = servers[index].Close()
	}
	if err := os.WriteFile(input.CompletePath, []byte("complete\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case outcome := <-completed:
		if outcome.err != nil || outcome.result.ResetCount != 1 || outcome.result.ResetBridges != 2 ||
			outcome.result.ActiveBefore != 2 || outcome.result.SelectedBridgeID != 0 ||
			outcome.result.ActiveBridges != 0 || outcome.result.AcceptedAfterReset != 0 {
			t.Fatalf("carrier relay final result = %+v / %v", outcome.result, outcome.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("carrier relay role did not complete")
	}
}

func TestCarrierRelayRoleRefusesDrainedAfterUnexpectedAcceptFailure(t *testing.T) {
	upstream := listenCarrierRelayTest(t)
	root := t.TempDir()
	input := config{Deadline: time.Now().Add(5 * time.Second).UTC().Format(time.RFC3339), CompletePath: filepath.Join(root, "complete"),
		CarrierRelayListenAddress: "127.0.0.1:0", CarrierRelayTargetAddress: upstream.Addr().String(),
		CarrierRelayReadyPath: filepath.Join(root, "carrier-relay-ready.json"), CarrierRelayResetPath: filepath.Join(root, "carrier-relay-reset"),
		CarrierRelayResetResultPath: filepath.Join(root, "carrier-relay-reset.json")}
	started := make(chan *carrierRelay, 1)
	completed := make(chan error, 1)
	go func() {
		_, err := serveCarrierRelayWithStart(input, func(listen, target string) (*carrierRelay, error) {
			relay, startErr := startCarrierRelay(listen, target)
			if startErr == nil {
				started <- relay
			}
			return relay, startErr
		})
		completed <- err
	}()
	relay := <-started
	waitCarrierRelayFile(t, input.CarrierRelayReadyPath)
	if err := relay.listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input.CompletePath, []byte("complete\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-completed:
		if err == nil || !strings.Contains(err.Error(), "carrier relay accept failed") {
			t.Fatalf("unexpected Accept failure result = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("carrier relay role ignored unexpected Accept failure")
	}
}

func listenCarrierRelayTest(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	return listener
}

func availableCarrierRelayTestEndpoint(t *testing.T) string {
	t.Helper()
	listener := listenCarrierRelayTest(t)
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func acceptCarrierRelayTest(t *testing.T, listener net.Listener) net.Conn {
	t.Helper()
	if tcp, ok := listener.(*net.TCPListener); ok {
		_ = tcp.SetDeadline(time.Now().Add(2 * time.Second))
	}
	connection, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	return connection
}

func readCarrierRelayTest(t *testing.T, connection net.Conn, size int) []byte {
	t.Helper()
	_ = connection.SetReadDeadline(time.Now().Add(2 * time.Second))
	value := make([]byte, size)
	if _, err := io.ReadFull(connection, value); err != nil {
		t.Fatal(err)
	}
	return value
}

func waitCarrierRelayTest(t *testing.T, accepted func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if accepted() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("carrier relay state did not converge")
}

func waitCarrierRelayFile(t *testing.T, path string) {
	t.Helper()
	waitCarrierRelayTest(t, func() bool {
		info, err := os.Stat(path)
		return err == nil && info.Mode().IsRegular() && info.Size() > 0
	})
}

func readCarrierRelayJSON(t *testing.T, path string, destination any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(raw, destination) != nil {
		t.Fatalf("read carrier relay receipt %s: %v / %q", path, err, raw)
	}
}

func requireCarrierRelayClosed(t *testing.T, connection net.Conn) {
	t.Helper()
	_ = connection.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := connection.Read(make([]byte, 1)); err == nil {
		t.Fatal("reset carrier connection remained readable")
	}
}
