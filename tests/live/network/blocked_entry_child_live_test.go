//go:build linux && live

package network_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

type blockedPathBoundary struct {
	Name, Source, Address string
	Port                  uint16
}

type blockedPathTarget struct {
	Address string `json:"address"`
	Port    uint16 `json:"port"`
}

type blockedPathManifest struct {
	Phase           string                `json:"phase"`
	Required        []blockedPathBoundary `json:"required"`
	Forbidden       []blockedPathBoundary `json:"forbidden,omitempty"`
	AllowedExternal []blockedPathTarget   `json:"allowed_external,omitempty"`
	DynamicLoopback []string              `json:"dynamic_loopback,omitempty"`
}

type blockedPathResult struct {
	Counts             map[string]int64 `json:"counts"`
	UnexpectedExternal int64            `json:"unexpected_external"`
	UnexpectedFlows    map[string]int64 `json:"unexpected_flows"`
	Packets            int64            `json:"packets"`
	Passed             bool             `json:"passed"`
}

type blockedDNSResult struct {
	Packets, Controls, Ambiguous                      int64
	IPv4UDPControls, IPv6UDPControls, IPv4TCPControls int64
	BoundaryControls                                  map[string]blockedDNSControl
}

type blockedDNSControl struct {
	IPv4UDP, IPv6UDP, IPv4TCP int64
	IfIndex                   int
	Token                     string
}

func writeBlockedJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBlockedEntryRole(t *testing.T) {
	role := os.Getenv("ARDENTS_BLOCKED_ROLE")
	if role == "" {
		t.Skip("container role only")
	}
	if start := os.Getenv("ARDENTS_BLOCKED_START_FILE"); start != "" {
		waitBlockedFile(t, start, 2*time.Minute)
	}
	if role != "policy" {
		if err := copyBlockedDirectory("/run/input", "/run/secure"); err != nil {
			t.Fatal(err)
		}
	}
	switch role {
	case "endpoint":
		runBlockedObserved(t, role, blockedManifest(role), runBlockedEndpoint)
	case "bridge":
		runBlockedBridge(t)
	case "probe":
		runBlockedProbe(t)
	case "policy":
		runBlockedPolicy(t)
	case "negative-endpoint":
		runBlockedNegativeEndpoint(t)
	case "fault-zero":
		runBlockedFaultZero(t)
	case "fault-one":
		runBlockedFaultOne(t)
	case "recovery-endpoint":
		runBlockedRecoveryParent(t)
	case "pressure":
		runBlockedPressure(t)
	case "capacity-probe":
		runBlockedCapacityProbe(t)
	case "resource-collector":
		runBlockedResourceCollector(t)
	case "carrier-collector":
		runBlockedCarrierCollector(t)
	case "initiator", "introduction", "rendezvous", "responder", "publisher":
		runBlockedObserved(t, role, blockedManifest(role), func(t *testing.T) {
			runBlockedCommand(t, "/usr/local/bin/ardents-route", "run", "/run/secure/plan.json")
		})
	case "client-service", "publisher-service":
		runBlockedCommand(t, "/usr/local/bin/ardents-service", "run", "/run/secure/plan.json")
	case "client-app":
		runBlockedCommand(t, "/usr/local/bin/ardents-stream-app", blockedStreamArguments(t, "client")...)
	case "publisher-app":
		runBlockedCommand(t, "/usr/local/bin/ardents-stream-app", blockedStreamArguments(t, "publisher")...)
	default:
		t.Fatalf("unknown blocked-entry role %q", role)
	}
}

func runBlockedEndpoint(t *testing.T) {
	t.Helper()
	prepareBlockedState(t, "route-state", "route-network")
	if os.Getenv("ARDENTS_BLOCKED_PROFILE") == "C0" {
		runBlockedCommand(t, "/usr/local/bin/ardents-route", "run", "/run/secure/route.json")
		return
	}
	prepareBlockedState(t, "bridge-network", "bridge-network")
	prepareBlockedState(t, "local-roles", "local-roles")
	runBlockedCommand(t, "/usr/local/bin/ardents-bridge", "import", "/run/secure/import.json")
	waitBlockedFile(t, filepath.Join(blockedSync(), "policy-ready"), 5*time.Second)
	timeline := proveBlockedPolicy(t)
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
	command.ExtraFiles = []*os.File{reader}
	done := make(chan error, 1)
	go func() {
		_, writeErr := writer.Write(transition)
		done <- errors.Join(writeErr, writer.Close())
	}()
	runPreparedBlockedCommand(t, command, os.Getenv("ARDENTS_BLOCKED_EXPECT_DRAIN") != "1")
	if err := errors.Join(<-done, reader.Close()); err != nil {
		t.Fatal(err)
	}
}

func runBlockedBridge(t *testing.T) {
	t.Helper()
	prepareBlockedState(t, "bridge-network", "bridge-network")
	prepareBlockedState(t, "local-roles", "local-roles")
	runBlockedCommand(t, "/usr/local/bin/ardents-bridge", "import", "/run/secure/import.json")
	prepareBlockedObservation(t, blockedManifest("bridge"))
	command := exec.Command("/usr/local/bin/ardents-bridge", "serve", "/run/secure/serve.json")
	var captured bytes.Buffer
	command.Stdout, command.Stderr = io.MultiWriter(os.Stdout, &captured), io.MultiWriter(os.Stderr, &captured)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	wait := make(chan error, 1)
	go func() { wait <- command.Wait() }()
	stopPath := filepath.Join(blockedSync(), "bridge-stop")
	deadline := time.Now().Add(2 * time.Minute)
	for !fileExists(stopPath) {
		select {
		case err := <-wait:
			if os.Getenv("ARDENTS_BLOCKED_EXPECT_DRAIN") != "1" || err != nil ||
				!strings.Contains(captured.String(), `"state":"DRAIN"`) ||
				!strings.Contains(captured.String(), `"state":"EXIT"`) {
				t.Fatalf("Bridge exited before stop: %v\n%s", err, captured.String())
			}
			publishBlockedResourceCleanup(t)
			finishBlockedObservation(t, blockedManifest("bridge"))
			return
		case <-time.After(25 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for Bridge stop")
		}
	}
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-wait:
		if err == nil || !strings.Contains(captured.String(), "context canceled") {
			t.Fatalf("Bridge shutdown = %v\n%s", err, captured.String())
		}
	case <-time.After(8 * time.Second):
		_ = command.Process.Kill()
		t.Fatal("Bridge command exceeded the cleanup deadline")
	}
	publishBlockedResourceCleanup(t)
	finishBlockedObservation(t, blockedManifest("bridge"))
}

func runBlockedObserved(t *testing.T, _ string, manifest blockedPathManifest, run func(*testing.T)) {
	t.Helper()
	prepareBlockedObservation(t, manifest)
	run(t)
	publishBlockedResourceCleanup(t)
	finishBlockedObservation(t, manifest)
}

func publishBlockedResourceCleanup(t *testing.T) {
	t.Helper()
	root := blockedSync()
	if !fileExists(filepath.Join(root, "resource-ready")) {
		return
	}
	writeLiveFile(t, filepath.Join(root, "resource-cleanup"), []byte("cleanup\n"))
	waitBlockedFile(t, filepath.Join(root, "resource-cleanup-captured"), 3*time.Second)
	waitBlockedFile(t, filepath.Join(root, "resource-release"), 2*time.Minute)
}

func runBlockedCommand(t *testing.T, executable string, arguments ...string) {
	t.Helper()
	runPreparedBlockedCommand(t, exec.Command(executable, arguments...), os.Getenv("ARDENTS_BLOCKED_EXPECT_DRAIN") != "1")
}

func runPreparedBlockedCommand(t *testing.T, command *exec.Cmd, requireSuccess bool) {
	t.Helper()
	var captured bytes.Buffer
	command.Stdout, command.Stderr = io.MultiWriter(os.Stdout, &captured), io.MultiWriter(os.Stderr, &captured)
	err := command.Run()
	if requireSuccess && err != nil {
		t.Fatalf("%s failed: %v\n%s", filepath.Base(command.Path), err, captured.String())
	}
	if !requireSuccess && err == nil {
		t.Fatalf("%s unexpectedly completed during emergency DRAIN\n%s", filepath.Base(command.Path), captured.String())
	}
}

func prepareBlockedState(t *testing.T, inputName, stateName string) {
	t.Helper()
	source, destination := filepath.Join("/run/secure", inputName), filepath.Join("/run/state", stateName)
	if err := copyBlockedDirectory(source, destination); err != nil {
		t.Fatal(err)
	}
}

func copyBlockedDirectory(source, destination string) error {
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		from, to := filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name())
		if entry.IsDir() {
			if err := copyBlockedDirectory(from, to); err != nil {
				return err
			}
			continue
		}
		raw, readErr := os.ReadFile(from)
		if readErr != nil {
			return readErr
		}
		if writeErr := os.WriteFile(to, raw, 0o600); writeErr != nil {
			return writeErr
		}
	}
	return nil
}

func prepareBlockedObservation(t *testing.T, manifest blockedPathManifest) {
	t.Helper()
	root := blockedSync()
	waitBlockedFile(t, filepath.Join(root, "ready"), 10*time.Second)
	runBlockedControlPlan(t, root, manifest)
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "path-manifest.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	waitBlockedFile(t, filepath.Join(root, "path-ready"), 3*time.Second)
}

func finishBlockedObservation(t *testing.T, manifest blockedPathManifest) {
	t.Helper()
	root := blockedSync()
	writeBlockedSignal(t, filepath.Join(root, "path-done"))
	waitBlockedFile(t, filepath.Join(root, "path-result.json"), 3*time.Second)
	var path blockedPathResult
	readBlockedJSON(t, filepath.Join(root, "path-result.json"), &path)
	if !path.Passed || path.UnexpectedExternal != 0 || path.Packets == 0 {
		t.Fatalf("path evidence = %+v", path)
	}
	writeBlockedSignal(t, filepath.Join(root, "stop"))
	waitBlockedFile(t, filepath.Join(root, "result.json"), 3*time.Second)
	var dns blockedDNSResult
	readBlockedJSON(t, filepath.Join(root, "result.json"), &dns)
	if !completeBlockedDNSObservation(dns, manifest) || dns.Packets != 0 || dns.Ambiguous != 0 {
		t.Fatalf("DNS evidence = %+v", dns)
	}
	fmt.Printf("{\"kind\":\"blocked-observation\",\"path_packets\":%d,\"dns_controls\":%d}\n",
		path.Packets, dns.Controls)
}

func blockedManifest(role string) blockedPathManifest {
	boundary := func(name, source, address string, port uint16) blockedPathBoundary {
		return blockedPathBoundary{Name: name, Source: source, Address: address, Port: port}
	}
	manifest := blockedPathManifest{Phase: "s5.3-" + role}
	switch role {
	case "endpoint":
		endpointAddress := os.Getenv("ARDENTS_BLOCKED_ENDPOINT_ADDRESS")
		if endpointAddress == "" {
			endpointAddress = "203.0.113.7"
		}
		if os.Getenv("ARDENTS_BLOCKED_PROFILE") == "C0" {
			manifest.Required = []blockedPathBoundary{boundary("E-to-O-Initiator", "172.31.20.7", "172.31.20.11", 4601)}
			manifest.AllowedExternal = []blockedPathTarget{{Address: "172.31.20.11", Port: 4601}}
			break
		}
		manifest.Required = []blockedPathBoundary{boundary("E-to-B-front", endpointAddress, "203.0.113.8", 8480)}
		manifest.AllowedExternal = []blockedPathTarget{{Address: "203.0.113.8", Port: 8480}}
		manifest.DynamicLoopback = []string{"candidate-socks"}
	case "bridge":
		addresses := []string{"203.0.113.7"}
		if encoded := os.Getenv("ARDENTS_BLOCKED_ENDPOINT_ADDRESSES"); encoded != "" {
			addresses = strings.Split(encoded, ",")
		}
		for index, address := range addresses {
			name := "E-to-B-front"
			if len(addresses) > 1 {
				name = fmt.Sprintf("E-to-B-front-%02d", index)
			}
			manifest.Required = append(manifest.Required, boundary(name, address, "203.0.113.8", 8480))
		}
		manifest.Required = append(manifest.Required,
			boundary("B-to-Initiator", "172.31.20.8", "172.31.20.11", 4601))
		manifest.AllowedExternal = []blockedPathTarget{{Address: "203.0.113.8", Port: 8480},
			{Address: "172.31.20.11", Port: 4601}}
		if os.Getenv("ARDENTS_BLOCKED_PROFILE") == "C2" {
			manifest.Required = append(manifest.Required,
				boundary("probe-to-B-front", "203.0.113.10", "203.0.113.8", 8480))
		}
		manifest.DynamicLoopback = []string{"front-to-WebTunnel-server"}
	case "probe":
		manifest.Required = []blockedPathBoundary{boundary("probe-to-B-front", "203.0.113.10", "203.0.113.8", 8480)}
		manifest.AllowedExternal = []blockedPathTarget{{Address: "203.0.113.8", Port: 8480}}
	case "initiator":
		firstName, firstSource := "B-to-Initiator", "172.31.20.8"
		if os.Getenv("ARDENTS_BLOCKED_PROFILE") == "C0" {
			firstName, firstSource = "E-to-O-Initiator", "172.31.20.7"
		}
		manifest.Required = []blockedPathBoundary{boundary(firstName, firstSource, "172.31.20.11", 4601),
			boundary("Initiator-to-Introduction", "172.31.20.11", "172.31.20.12", 4602)}
	case "introduction":
		manifest.Required = []blockedPathBoundary{boundary("Initiator-to-Introduction", "172.31.20.11", "172.31.20.12", 4602),
			boundary("Introduction-to-Rendezvous", "172.31.20.12", "172.31.20.13", 4603)}
	case "rendezvous":
		manifest.Required = []blockedPathBoundary{boundary("Introduction-to-Rendezvous", "172.31.20.12", "172.31.20.13", 4603),
			boundary("Rendezvous-to-Responder", "172.31.20.13", "172.31.20.14", 4604)}
	case "responder":
		manifest.Required = []blockedPathBoundary{boundary("Rendezvous-to-Responder", "172.31.20.13", "172.31.20.14", 4604),
			boundary("Responder-to-Publisher", "172.31.20.14", "172.31.20.16", 4605)}
	case "publisher":
		manifest.Required = []blockedPathBoundary{boundary("Responder-to-Publisher", "172.31.20.14", "172.31.20.16", 4605)}
	}
	return manifest
}

func blockedSync() string { return "/run/evidence" }

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func writeBlockedSignal(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("ready\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func waitBlockedFile(t *testing.T, path string, bound time.Duration) {
	t.Helper()
	deadline := time.Now().Add(bound)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", filepath.Base(path))
}

func readBlockedJSON(t *testing.T, path string, output any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(raw, output) != nil {
		t.Fatalf("read %s: %v", filepath.Base(path), err)
	}
}
