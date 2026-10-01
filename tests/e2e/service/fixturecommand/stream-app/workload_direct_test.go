package main

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
	const measurementWindow = 200 * time.Millisecond
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
			Bytes: maximumDirectBytes, MeasureDuration: measurementWindow, Output: &result.output,
			Ready: func(address string) { ready <- address }})
		serverResult <- result
	}()
	var clientOutput bytes.Buffer
	if err := Direct(ctx, DirectConfig{Role: "direct-connect", Address: <-ready, Seed: seed,
		Bytes: maximumDirectBytes, MeasureDuration: measurementWindow, Output: &clientOutput}); err != nil {
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
		client.DurationMillis < uint32(measurementWindow/time.Millisecond) {
		t.Fatalf("timed direct observations differ: client=%+v receiver=%+v", client, receiver)
	}
}

func TestTimedDirectWorkloadWaitsForReceiverMeasurementStart(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	seed := [32]byte{11}
	var receiverOutput bytes.Buffer
	done := make(chan error, 1)
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer connection.Close()
		if err = setDirectLifetime(ctx, connection); err != nil {
			done <- err
			return
		}
		// Model delayed receiver scheduling after TCP connect, before measurement.
		time.Sleep(50 * time.Millisecond)
		done <- receiveTimedDirect(ctx, connection, DirectConfig{Seed: seed, Bytes: maximumDirectBytes,
			MeasureDuration: 200 * time.Millisecond, Output: &receiverOutput})
	}()
	var senderOutput bytes.Buffer
	senderErr := Direct(ctx, DirectConfig{Role: "direct-connect", Address: listener.Addr().String(), Seed: seed,
		Bytes: maximumDirectBytes, MeasureDuration: 200 * time.Millisecond, Output: &senderOutput})
	receiverErr := <-done
	if senderErr != nil || receiverErr != nil {
		t.Fatalf("delayed receiver exchange: sender=%v receiver=%v", senderErr, receiverErr)
	}
	var sender, receiver Observation
	if json.Unmarshal(bytes.TrimSpace(senderOutput.Bytes()), &sender) != nil ||
		json.Unmarshal(bytes.TrimSpace(receiverOutput.Bytes()), &receiver) != nil ||
		sender.SentBytes == 0 || sender.SentBytes != receiver.ReceivedBytes || sender.SentDigest != receiver.ReceivedDigest {
		t.Fatalf("delivered observations differ: sender=%+v receiver=%+v", sender, receiver)
	}
}

func TestTimedDirectWorkloadCancelsBeforeReceiverStart(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	peer := make(chan net.Conn, 1)
	go func() { connection, _ := listener.Accept(); peer <- connection }()
	done := make(chan error, 1)
	go func() {
		done <- Direct(ctx, DirectConfig{Role: "direct-connect", Address: listener.Addr().String(),
			Bytes: maximumDirectBytes, MeasureDuration: time.Second, Output: &bytes.Buffer{}})
	}()
	connection := <-peer
	if connection == nil {
		t.Fatal("peer accept failed")
	}
	defer connection.Close()
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled receiver-start wait succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("receiver-start wait did not join after cancellation")
	}
}
