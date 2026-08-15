package streamworkload

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestDirectWorkloadMeasuresTheSameBoundedBytes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	seed := [32]byte{7}
	ready := make(chan string, 1)
	type outcome struct {
		output bytes.Buffer
		err    error
	}
	serverResult := make(chan outcome, 1)
	go func() {
		var result outcome
		result.err = Direct(ctx, DirectConfig{Role: "direct-listen", Address: "127.0.0.1:0", Seed: seed,
			Bytes: 1 << 20, Output: &result.output, Ready: func(address string) { ready <- address }})
		serverResult <- result
	}()
	address := <-ready
	var clientOutput bytes.Buffer
	if err := Direct(ctx, DirectConfig{Role: "direct-connect", Address: address, Seed: seed,
		Bytes: 1 << 20, Output: &clientOutput}); err != nil {
		t.Fatal(err)
	}
	server := <-serverResult
	if server.err != nil {
		t.Fatal(server.err)
	}
	for role, raw := range map[string][]byte{"direct-client": clientOutput.Bytes(), "direct-server": server.output.Bytes()} {
		var observation Observation
		if err := json.Unmarshal(bytes.TrimSpace(raw), &observation); err != nil || observation.Terminal != "success" ||
			(observation.SentBytes != 1<<20 && observation.ReceivedBytes != 1<<20) {
			t.Fatalf("%s observation=%+v err=%v", role, observation, err)
		}
	}
}

func TestTimedDirectWorkloadHonorsEarlierCallerDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	peer := make(chan net.Conn, 1)
	go func() {
		connection, _ := listener.Accept()
		peer <- connection
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = Direct(ctx, DirectConfig{Role: "direct-connect", Address: listener.Addr().String(), Seed: [32]byte{9},
		Bytes: maximumDirectBytes, MeasureDuration: time.Second, Output: &bytes.Buffer{}})
	connection := <-peer
	if connection != nil {
		connection.Close()
	}
	if err == nil || time.Since(started) > 200*time.Millisecond {
		t.Fatalf("caller deadline returned %v after %s", err, time.Since(started))
	}
}

func TestTimedDirectReceiverRejectsEarlyCorrectEOF(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	seed := [32]byte{10}
	ready := make(chan string, 1)
	serverResult := make(chan error, 1)
	go func() {
		serverResult <- Direct(ctx, DirectConfig{Role: "direct-listen", Address: "127.0.0.1:0", Seed: seed,
			Bytes: 1 << 20, MeasureDuration: 100 * time.Millisecond, Output: &bytes.Buffer{},
			Ready: func(address string) { ready <- address }})
	}()
	connection, err := net.Dial("tcp", <-ready)
	if err != nil {
		t.Fatal(err)
	}
	source, value := generator{seed: seed}, make([]byte, 1024)
	source.fill(value)
	if _, err := connection.Write(value); err != nil {
		t.Fatal(err)
	}
	if err := connection.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	if err := <-serverResult; err == nil {
		t.Fatal("timed receiver accepted an early correct prefix")
	}
	connection.Close()
}

func TestDirectWorkloadRejectsUnboundedBytesAndCancelsStartDelay(t *testing.T) {
	if err := Direct(context.Background(), DirectConfig{Role: "direct-connect", Address: "127.0.0.1:1",
		Bytes: (256 << 20) + 1, Output: &bytes.Buffer{}}); err == nil {
		t.Fatal("unbounded direct workload was accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	err := Direct(ctx, DirectConfig{Role: "direct-connect", Address: "127.0.0.1:1",
		Bytes: 1, Output: &bytes.Buffer{}, StartDelay: 5 * time.Second})
	if err == nil || time.Since(started) > 100*time.Millisecond {
		t.Fatalf("cancelled start delay returned %v after %s", err, time.Since(started))
	}
}

func TestTimedDirectWorkloadReportsDeliveredBytesInFixedWindow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	seed := [32]byte{8}
	ready := make(chan string, 1)
	type outcome struct {
		output bytes.Buffer
		err    error
	}
	serverResult := make(chan outcome, 1)
	go func() {
		var result outcome
		result.err = Direct(ctx, DirectConfig{Role: "direct-listen", Address: "127.0.0.1:0", Seed: seed,
			Bytes: maximumDirectBytes, MeasureDuration: 20 * time.Millisecond, Output: &result.output,
			Ready: func(address string) { ready <- address }})
		serverResult <- result
	}()
	var clientOutput bytes.Buffer
	if err := Direct(ctx, DirectConfig{Role: "direct-connect", Address: <-ready, Seed: seed,
		Bytes: maximumDirectBytes, MeasureDuration: 20 * time.Millisecond, Output: &clientOutput}); err != nil {
		t.Fatal(err)
	}
	server := <-serverResult
	if server.err != nil {
		t.Fatal(server.err)
	}
	var client, receiver Observation
	if json.Unmarshal(bytes.TrimSpace(clientOutput.Bytes()), &client) != nil ||
		json.Unmarshal(bytes.TrimSpace(server.output.Bytes()), &receiver) != nil ||
		client.Terminal != "success" || receiver.Terminal != "success" || client.SentBytes == 0 ||
		client.SentBytes != receiver.ReceivedBytes || client.SentDigest != receiver.ReceivedDigest ||
		client.DurationMillis < 20 {
		t.Fatalf("timed direct observations differ: client=%+v receiver=%+v", client, receiver)
	}
}
