package recoverysmoke

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestCarrierSocketIdentityEncodesEndpointsAndDigest(t *testing.T) {
	raw := make([]byte, carrierSocketIDBytes)
	binary.BigEndian.PutUint16(raw[0:2], 50123)
	binary.BigEndian.PutUint16(raw[2:4], 4604)
	copy(raw[4:8], net.ParseIP("172.31.21.13").To4())
	copy(raw[20:24], net.ParseIP("172.31.21.14").To4())
	observation := carrierObservationFromID(raw, 77)
	if observation.LocalAddress != "172.31.21.13:50123" || observation.RemoteAddress != "172.31.21.14:4604" ||
		observation.Inode != 77 || len(observation.SocketID) != 96 || len(observation.SocketIDSHA256) != 64 {
		t.Fatalf("unexpected observation: %+v", observation)
	}
	if !carrierMatchesRemote(raw, net.ParseIP("172.31.21.14"), 4604) {
		t.Fatal("socket did not match its exact remote endpoint")
	}
	if err := validateDedicatedCarrier(observation); err != nil {
		t.Fatalf("dedicated Carrier rejected: %v", err)
	}
	routeRaw := append([]byte(nil), raw...)
	copy(routeRaw[4:8], net.ParseIP("172.31.20.13").To4())
	copy(routeRaw[20:24], net.ParseIP("172.31.20.14").To4())
	routeSocket := carrierObservationFromID(routeRaw, 78)
	if err := validateDedicatedCarrier(routeSocket); err == nil {
		t.Fatal("route-network socket passed the privileged Carrier fault boundary")
	}
	if _, err := faultCarrierSocket(routeSocket.SocketID); err == nil {
		t.Fatal("route-network socket reached the privileged interface operation")
	}
}

func TestCarrierFaultWaitReportsReadyAndStopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() { done <- carrierFaultWait(ctx, func() { ready <- struct{}{} }) }()
	<-ready
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("carrierFaultWait returned %v", err)
	}
}

func TestCarrierFaultWaitAdapterRemainsAliveAfterReady(t *testing.T) {
	if os.Getenv("ARDENTS_TEST_CARRIER_WAIT") == "1" {
		code, handled := RunCarrierFaultAdapter([]string{"carrier-fault", "wait"}, os.Stdout, os.Stderr)
		if !handled {
			os.Exit(3)
		}
		os.Exit(code)
	}
	command := exec.Command(os.Args[0], "-test.run=TestCarrierFaultWaitAdapterRemainsAliveAfterReady")
	command.Env = append(os.Environ(), "ARDENTS_TEST_CARRIER_WAIT=1")
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics bytes.Buffer
	command.Stderr = &diagnostics
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	finished := false
	defer func() {
		if !finished {
			_ = command.Process.Kill()
			<-done
		}
	}()
	var ready map[string]string
	decoded := make(chan error, 1)
	go func() { decoded <- json.NewDecoder(stdout).Decode(&ready) }()
	select {
	case err := <-decoded:
		if err != nil || ready["kind"] != "ready" {
			t.Fatalf("controller readiness failed: %v %q", err, diagnostics.String())
		}
	case <-time.After(time.Second):
		t.Fatal("controller did not report readiness")
	}
	select {
	case err := <-done:
		finished = true
		t.Fatalf("controller exited after readiness: %v %q", err, diagnostics.String())
	case <-time.After(200 * time.Millisecond):
	}
	if err := command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	<-done
	finished = true
}
