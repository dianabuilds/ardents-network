//go:build linux && live

package network_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/camouflage"
)

type blockedFaultPlan struct {
	Envelope string `json:"envelope"`
	Identity string `json:"identity"`
}

func runBlockedNegativeEndpoint(t *testing.T) {
	t.Helper()
	profile := os.Getenv("ARDENTS_BLOCKED_PROFILE")
	expected := os.Getenv("ARDENTS_BLOCKED_EXPECTED_TERMINAL")
	if expected == "" {
		expected = "bridge-attempt-exhausted"
	}
	var timeline blockedTimeline
	if profile == "C4" {
		timeline = startBlockedTimeline(t)
	}
	manifest := blockedNegativeManifest(profile)
	prepareBlockedObservation(t, manifest)
	sendBlockedPathControl(t)
	prepareBlockedState(t, "route-state", "route-network")
	prepareBlockedState(t, "bridge-network", "bridge-network")
	prepareBlockedState(t, "local-roles", "local-roles")
	slots := 2
	if profile == "G5" {
		slots = 1
	}
	for slot := range slots {
		runBlockedCommand(t, "/usr/local/bin/ardents-bridge", "import",
			fmt.Sprintf("/run/secure/import-%d.json", slot))
	}
	if profile == "C3" {
		waitBlockedFile(t, filepath.Join(blockedSync(), "policy-ready"), 5*time.Second)
		timeline = readBlockedTimeline(t)
	}
	transition, err := os.ReadFile("/run/secure/transition.bin")
	if err != nil {
		t.Fatal(err)
	}
	transition = stampBlockedTransition(t, transition, timeline)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("/usr/local/bin/ardents-route", "run", "/run/secure/route.json",
		"--entry-plan", "/run/secure/entry.json")
	attachment, attachmentDone := serveBlockedNegativeAttachment(t)
	defer attachment.Close()
	command.ExtraFiles = []*os.File{reader}
	var captured bytes.Buffer
	command.Stdout, command.Stderr = io.MultiWriter(os.Stdout, &captured), io.MultiWriter(os.Stderr, &captured)
	done := make(chan error, 1)
	go func() {
		_, writeErr := writer.Write(transition)
		done <- errors.Join(writeErr, writer.Close())
	}()
	runErr := command.Run()
	pipeErr := errors.Join(<-done, reader.Close())
	_ = attachment.Close()
	<-attachmentDone
	if runErr == nil || pipeErr != nil || !strings.Contains(captured.String(), expected) {
		t.Fatalf("%s terminal = %v, pipe=%v\n%s", profile, runErr, pipeErr, captured.String())
	}
	finishBlockedObservation(t, manifest)
	result := map[string]any{"kind": "negative-result", "profile": profile,
		"terminal": expected, "forbidden_fallbacks": 0}
	raw, _ := json.Marshal(result)
	if err := os.WriteFile(filepath.Join(blockedSync(), "negative-result.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(raw))
}

func serveBlockedNegativeAttachment(t *testing.T) (net.Listener, <-chan struct{}) {
	t.Helper()
	path := "/run/ardents/client-route/route.sock"
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			_, _ = io.Copy(io.Discard, connection)
			_ = connection.Close()
		}
	}()
	return listener, done
}

func blockedNegativeManifest(profile string) blockedPathManifest {
	boundary := func(name, source, address string, port uint16) blockedPathBoundary {
		return blockedPathBoundary{Name: name, Source: source, Address: address, Port: port}
	}
	manifest := blockedPathManifest{Phase: "s5.3-" + profile,
		Required: []blockedPathBoundary{boundary("observer-control", "127.0.0.1", "127.0.0.1", 4999)},
		AllowedExternal: []blockedPathTarget{{Address: "203.0.113.20", Port: 8481},
			{Address: "203.0.113.21", Port: 8482}},
		DynamicLoopback: []string{"candidate-socks-0", "candidate-socks-1", "candidate-socks-2", "candidate-socks-3"}}
	if profile == "recovery" {
		manifest.DynamicLoopback = []string{"candidate-socks"}
	}
	if profile == "C4" {
		manifest.Required = append(manifest.Required,
			boundary("slot-zero-fault", "203.0.113.7", "203.0.113.20", 8481),
			boundary("slot-one-fault", "203.0.113.7", "203.0.113.21", 8482))
	}
	if profile == "G5" {
		variant := os.Getenv("ARDENTS_HOSTILE_VARIANT")
		if variant == "malformed-pt-control" || variant == "wrong-socks-listener-method" {
			manifest.AllowedExternal = nil
		} else {
			manifest.Required = append(manifest.Required,
				boundary("accept-then-stall", "203.0.113.7", "203.0.113.20", 8481))
			manifest.AllowedExternal = manifest.AllowedExternal[:1]
		}
		manifest.DynamicLoopback = manifest.DynamicLoopback[:1]
	}
	return manifest
}

func sendBlockedPathControl(t *testing.T) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:4999")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			_, acceptErr = connection.Write([]byte("path-control"))
			_ = connection.Close()
		}
		done <- acceptErr
	}()
	connection, err := net.DialTimeout("tcp4", listener.Addr().String(), time.Second)
	if err == nil {
		_, err = io.ReadAll(connection)
		_ = connection.Close()
	}
	_ = listener.Close()
	if err := errors.Join(err, <-done); err != nil {
		t.Fatal(err)
	}
}

func runBlockedFaultZero(t *testing.T) {
	t.Helper()
	listener, err := net.Listen("tcp4", "203.0.113.20:8481")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	variant := os.Getenv("ARDENTS_HOSTILE_VARIANT")
	fmt.Println(`{"kind":"fault","state":"READY","slot":0}`)
	if variant == "child-exit" {
		return
	}
	first, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	if variant == "slow-partial-handshake" {
		_, _ = first.Write([]byte{0x16, 0x03})
		time.Sleep(6 * time.Second)
	} else if variant == "malformed-pt-control" || variant == "wrong-socks-listener-method" ||
		variant == "malformed-carriage" {
		_, _ = first.Write([]byte("malformed-" + variant))
	}
	_ = first.Close()
	second, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	if variant == "accept-then-stall" || variant == "evidence-write-exhaustion" {
		time.Sleep(6 * time.Second)
	} else {
		_, _ = second.Write([]byte("rejected-" + variant))
	}
	_ = second.Close()
}

func runBlockedFaultOne(t *testing.T) {
	t.Helper()
	var plan blockedFaultPlan
	readBlockedJSON(t, "/run/secure/fault.json", &plan)
	envelope, err := hex.DecodeString(plan.Envelope)
	if err != nil {
		t.Fatal(err)
	}
	var identity [32]byte
	rawIdentity, err := hex.DecodeString(plan.Identity)
	if err != nil || len(rawIdentity) != len(identity) {
		t.Fatalf("fault identity is invalid: %v", err)
	}
	copy(identity[:], rawIdentity)
	config, err := camouflage.Validate(envelope, identity)
	if err != nil {
		t.Fatal(err)
	}
	next, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	nextDone := make(chan error, 1)
	go runBlockedNextFault(next, nextDone, os.Getenv("ARDENTS_FAULT_MODE"))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	serving, err := camouflage.Serve(ctx, config, camouflage.Server{Binary: "/candidate/webtunnel-server",
		StateRoot: "/run/state/fault-server", Certificate: "/run/secure/front-cert.pem",
		Key: "/run/secure/front-key.pem", NextLeg: next.Addr().String(), Deadline: time.Now().Add(50 * time.Second),
		ResourceProfile: "h3-s-v1"})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(`{"kind":"fault","state":"READY","slot":1}`)
	if err := <-nextDone; err != nil {
		t.Fatal(err)
	}
	if err := serving.Close(); err != nil {
		t.Fatal(err)
	}
}

func runBlockedNextFault(listener net.Listener, done chan<- error, mode string) {
	first, err := listener.Accept()
	if err != nil {
		done <- err
		return
	}
	if mode == "recovery" {
		_, err = io.Copy(io.Discard, first)
		_ = first.Close()
		done <- err
		return
	}
	time.Sleep(8 * time.Second)
	_ = first.Close()
	second, err := listener.Accept()
	if err == nil {
		_ = second.Close()
	}
	done <- err
}
