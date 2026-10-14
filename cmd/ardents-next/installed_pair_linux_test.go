//go:build linux && text_worker_installed

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/administration"
	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

// This is a local test-controller exchange, not an Ardents protocol or authority.
// It carries public signed inputs, actual permission requests/responses and
// explicit fixture commands. No private key, live owner or accepting flag is sent.
type installedPairMessage struct {
	Operation string
	Role      string
	Nonce     string
	Network   *installedStartupNetwork
	Body      []byte
	Link      string
	Original  [3]string
}

func installedPairSend(t *testing.T, conn net.Conn, value installedPairMessage) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > 4<<20 {
		t.Fatal("invalid pair fixture message", err)
	}
	packet := binary.BigEndian.AppendUint32(nil, uint32(len(raw)))
	packet = append(packet, raw...)
	if _, err := io.Copy(conn, bytes.NewReader(packet)); err != nil {
		t.Fatal("pair fixture output", err)
	}
}

func installedPairReceive(t *testing.T, conn net.Conn) installedPairMessage {
	t.Helper()
	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		t.Fatal("pair fixture input", err)
	}
	size := binary.BigEndian.Uint32(header[:])
	if size == 0 || size > 4<<20 {
		t.Fatal("pair fixture message bound")
	}
	raw := make([]byte, size)
	if _, err := io.ReadFull(conn, raw); err != nil {
		t.Fatal(err)
	}
	var value installedPairMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		t.Fatal("pair fixture trailing input", err)
	}
	return value
}

func TestInstalledServicePairParticipant(t *testing.T) {
	_, artifacts := installedStartupPaths(t)
	resources := installedStartupResources(t, artifacts)
	role := os.Getenv("ARDENTS_EXECUTION_PAIR_ROLE")
	if role != "publisher" && role != "reader" {
		t.Fatal("invalid environment: selected pair role required")
	}
	nonce := os.Getenv("ARDENTS_EXECUTION_PAIR_NONCE")
	decoded, err := hex.DecodeString(nonce)
	if err != nil || len(decoded) != 32 {
		t.Fatal("invalid environment: original fixture nonce required")
	}
	conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", os.Getenv("ARDENTS_EXECUTION_PAIR_ADDRESS"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(4 * time.Minute))
	installedPairSend(t, conn, installedPairMessage{Operation: "hello", Role: role, Nonce: nonce})
	input := installedPairReceive(t, conn)
	if input.Operation != "network" || input.Network == nil {
		t.Fatal("pair fixture lacks signed Network input")
	}
	runInstalledParticipant(t, true, *input.Network, resources, func(_ string, permissionRole string, request []byte) []byte {
		installedPairSend(t, conn, installedPairMessage{Operation: "permission", Role: permissionRole, Body: request})
		response := installedPairReceive(t, conn)
		if response.Operation != "permission" || response.Role != permissionRole {
			t.Fatal("foreign fixture approval response")
		}
		return response.Body
	}, func(participant installedStartupParticipant) {
		installedPairSend(t, conn, installedPairMessage{Operation: "ready", Original: participant.original})
		for {
			command := installedPairReceive(t, conn)
			if installedStartupIdentity(t) != participant.original {
				t.Fatal("pair command lost original installed participant")
			}
			response := installedPairMessage{Operation: command.Operation, Original: participant.original}
			switch command.Operation {
			case "publish":
				if role != "publisher" {
					t.Fatal("Reader received publication fixture command")
				}
				ctx, cancel := context.WithTimeout(t.Context(), administration.SnapshotTimeout)
				_, err := administration.RequestSnapshot(ctx, filepath.Join(participant.base, "administration", "socket"), command.Body)
				cancel()
				if err != nil {
					t.Fatal("pair installed Snapshot", err)
				}
				ctx, cancel = context.WithTimeout(t.Context(), 15*time.Second)
				response.Link, err = administration.RequestPublishedLink(ctx, filepath.Join(participant.base, "administration", "socket"))
				cancel()
				if err != nil {
					t.Fatal("pair installed Link", err)
				}
			case "read":
				if role != "reader" {
					t.Fatal("Publisher received Reader fixture command")
				}
				ctx, cancel := context.WithTimeout(t.Context(), application.ReaderTimeout)
				stream, err := application.Dial(ctx, filepath.Join(participant.base, "application", "reader"), application.Request{Destination: application.TargetLink, Value: command.Link})
				if err == nil {
					response.Body, err = textdocument.Read(ctx, stream)
				}
				cancel()
				if err != nil {
					t.Fatal("pair installed Reader exchange", err)
				}
			case "withdraw":
				if role != "publisher" {
					t.Fatal("Reader received withdrawal fixture command")
				}
				ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
				_, err := administration.Request(ctx, filepath.Join(participant.base, "administration", "socket"), administration.Withdraw)
				cancel()
				if err != nil {
					t.Fatal("pair installed withdrawal", err)
				}
				ctx, cancel = context.WithTimeout(t.Context(), 15*time.Second)
				link, err := administration.RequestPublishedLink(ctx, filepath.Join(participant.base, "administration", "socket"))
				cancel()
				if err == nil || link != "" {
					t.Fatal("withdrawn installed Publisher still exposes a Link")
				}
			case "finish":
				installedPairSend(t, conn, response)
				return // The original participant's registered cleanup must join.
			case "restart-refusal":
				installedPairRestartRefusal(t, participant)
				installedPairSend(t, conn, response)
				return
			default:
				t.Fatal("unknown pair fixture command")
			}
			installedPairSend(t, conn, response)
		}
	})
}

// The outer controller is fixture infrastructure, never an admitted Endpoint.
// Each child runs the checked profile under its own original system manager.
func TestInstalledServicePair(t *testing.T) {
	base, artifacts := installedStartupPaths(t)
	installedStartupResources(t, artifacts)
	carrier := os.Getenv("ARDENTS_EXECUTION_START_CARRIER")
	if carrier != "ardents-carrier-tcp-tls-v2" && carrier != "ardents-carrier-quic-v2" {
		t.Fatal("invalid environment: pair Carrier required")
	}
	managers := strings.Split(os.Getenv("ARDENTS_EXECUTION_PAIR_MANAGERS"), ",")
	if len(managers) != 2 || managers[0] == managers[1] {
		t.Fatal("invalid environment: two original managers required")
	}
	seen := make(map[string]bool)
	var originalManagers [2]*os.File
	var managerResources [2][]*os.File
	for i, manager := range managers {
		pid, err := strconv.Atoi(manager)
		if err != nil || pid <= 1 {
			t.Fatal("invalid original manager PID")
		}
		original, err := os.Open(filepath.Join("/proc", manager))
		if err != nil {
			t.Fatal("open original manager", err)
		}
		originalManagers[i] = original
		t.Cleanup(func() { _ = original.Close() })
		comm, err := os.ReadFile(filepath.Join("/proc", manager, "comm"))
		if err != nil || string(comm) != "systemd\n" {
			t.Fatal("original pair manager absent", err)
		}
		for _, namespace := range []string{"pid", "mnt", "cgroup"} {
			path := filepath.Join("/proc", manager, "ns", namespace)
			identity, err := os.Readlink(path)
			local, localErr := os.Readlink(filepath.Join("/proc/self/ns", namespace))
			if err != nil || localErr != nil || identity == local || seen[identity] {
				t.Fatal("pair managers do not have separate original namespaces", namespace, err, localErr)
			}
			seen[identity] = true
		}
		// Pass the actual opened namespace and root descriptors to nsenter.
		// A later PID reuse cannot redirect entry to a different manager.
		for _, resource := range []string{"ns/mnt", "ns/pid", "ns/cgroup", "root"} {
			file, err := os.Open(filepath.Join("/proc", manager, resource))
			if err != nil {
				t.Fatal("retain original manager resource", err)
			}
			managerResources[i] = append(managerResources[i], file)
			t.Cleanup(func() { _ = file.Close() })
		}
	}
	checkManagers := func() {
		for i, original := range originalManagers {
			retained, retainedErr := original.Stat()
			current, currentErr := os.Stat(filepath.Join("/proc", managers[i]))
			if retainedErr != nil || currentErr != nil || !os.SameFile(retained, current) {
				t.Fatal("lost original pair manager", retainedErr, currentErr)
			}
		}
	}
	checkManagers()
	f := installedServiceNetwork(t, carrier)
	network := installedSignedNetwork(t, f)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	stop := context.AfterFunc(t.Context(), func() { _ = listener.Close() })
	defer stop()
	var peers [2]net.Conn
	var identities [2][3]string
	var commands [2]*exec.Cmd
	var outputs [2]bytes.Buffer
	var joined [2]bool
	t.Cleanup(func() {
		for _, peer := range peers {
			if peer != nil {
				_ = peer.Close()
			}
		}
		for i, command := range commands {
			if command != nil && !joined[i] {
				// Closing the original fixture channel interrupts the child's
				// wait and lets its real Installation cleanup run before Wait.
				if err := command.Wait(); err != nil {
					t.Errorf("pair child cleanup: %v\n%s", err, outputs[i].String())
				}
			}
		}
	})
	for i, role := range []string{"publisher", "reader"} {
		var nonce [32]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			t.Fatal(err)
		}
		checkManagers()
		args := []string{"--mount=/proc/self/fd/3", "--pid=/proc/self/fd/4", "--cgroup=/proc/self/fd/5", "--root=/proc/self/fd/6", "--", "env",
			"ARDENTS_EXECUTION_START_TEST=/root/startup.test", "ARDENTS_EXECUTION_START_RESOURCES=/root/artifacts",
			"ARDENTS_EXECUTION_START_ROOT=/var/lib/profile-root/reader", "ARDENTS_EXECUTION_START_LOG=/root/pair-participant.log",
			"ARDENTS_EXECUTION_START_TEST_SHA256=" + os.Getenv("ARDENTS_EXECUTION_START_TEST_SHA256"),
			"ARDENTS_EXECUTION_START_INVENTORY_SHA256=" + os.Getenv("ARDENTS_EXECUTION_START_INVENTORY_SHA256"),
			"ARDENTS_EXECUTION_START_CARRIER=" + carrier, "ARDENTS_EXECUTION_PAIR_ROLE=" + role,
			"ARDENTS_EXECUTION_PAIR_ADDRESS=" + listener.Addr().String(), "ARDENTS_EXECUTION_PAIR_NONCE=" + hex.EncodeToString(nonce[:]),
			"sh", "/root/run-profile.sh", "service-pair-participant", "-timeout=4m"}
		// The checked guest profile owns its finite test timeout and cleanup.
		// Cancelling the outer test must close the fixture connection, not kill
		// this driver before its original Installation resources have joined.
		command := exec.Command("nsenter", args...)
		command.ExtraFiles = managerResources[i]
		command.Stdout, command.Stderr = &outputs[i], &outputs[i]
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		commands[i] = command
		if listener, ok := listener.(*net.TCPListener); ok {
			_ = listener.SetDeadline(time.Now().Add(30 * time.Second))
		}
		conn, err := listener.Accept()
		if err != nil {
			t.Fatal("pair participant did not connect", err)
		}
		peers[i] = conn
		_ = conn.SetDeadline(time.Now().Add(4 * time.Minute))
		hello := installedPairReceive(t, conn)
		if hello.Operation != "hello" || hello.Role != role || hello.Nonce != hex.EncodeToString(nonce[:]) {
			t.Fatal("foreign pair fixture client")
		}
		installedPairSend(t, conn, installedPairMessage{Operation: "network", Network: &network})
		approvalRoot := filepath.Join(base, role)
		if err := os.Mkdir(approvalRoot, 0700); err != nil {
			t.Fatal(err)
		}
		approved := make(map[string]bool)
		for {
			request := installedPairReceive(t, conn)
			if request.Operation == "ready" {
				if !approved["reader"] || !approved["publisher"] || request.Original[1] == "" {
					t.Fatal("participant omitted independent admission")
				}
				identities[i] = request.Original
				break
			}
			if request.Operation != "permission" || (request.Role != "reader" && request.Role != "publisher") || approved[request.Role] {
				t.Fatal("unexpected pair approval request")
			}
			response := installedServiceApproval(t, approvalRoot, request.Role, request.Body, f)
			approved[request.Role] = true
			installedPairSend(t, conn, installedPairMessage{Operation: "permission", Role: request.Role, Body: response})
		}
	}
	if identities[0][1] == identities[1][1] {
		t.Fatal("participants share a manager invocation")
	}
	exchange := func(index int, command installedPairMessage) installedPairMessage {
		checkManagers()
		started := time.Now()
		t.Logf("installed participant %d: %s started", index, command.Operation)
		defer func() {
			t.Logf("installed participant %d: %s returned after %s", index, command.Operation, time.Since(started))
		}()
		installedPairSend(t, peers[index], command)
		response := installedPairReceive(t, peers[index])
		if response.Operation != command.Operation || response.Original != identities[index] {
			t.Fatal("pair handoff lost original participant")
		}
		checkManagers()
		return response
	}
	body := []byte("one request across independently installed participants\n")
	publication := exchange(0, installedPairMessage{Operation: "publish", Body: body})
	if publication.Link == "" {
		t.Fatal("installed Publisher supplied no Link")
	}
	read := exchange(1, installedPairMessage{Operation: "read", Link: publication.Link})
	if !bytes.Equal(read.Body, body) {
		t.Fatal("installed Reader returned different Application bytes")
	}
	exchange(0, installedPairMessage{Operation: "withdraw"})
	for i := range peers {
		exchange(i, installedPairMessage{Operation: "restart-refusal"})
		_ = peers[i].Close()
		peers[i] = nil
		waitErr := commands[i].Wait()
		joined[i] = true
		t.Log(outputs[i].String())
		if waitErr != nil {
			t.Fatal("original participant did not join", waitErr)
		}
	}
	t.Log("two independent installed admissions, one actual AAI3 text request, exact response, owning withdrawal and participant cleanup completed")
}

// A new process cannot adopt the first invocation's completed Root exchange.
// This is a real restart refusal, not permission to recover or replay ACK.
func installedPairRestartRefusal(t *testing.T, participant installedStartupParticipant) {
	t.Helper()
	participant.stop()
	if t.Failed() {
		t.Fatal("original participant did not join before restart")
	}
	before := installedPairFileDigests(t, participant.base)
	previousJournal, err := exec.CommandContext(t.Context(), "journalctl", "-u", "ardents-endpoint.service", "--no-pager", "-o", "cat").CombinedOutput()
	if err != nil {
		t.Fatal("original stopped participant journal", err)
	}
	refusalMarker := []byte(`"phase":"installed-startup"`)
	defer func() {
		if output, err := exec.Command("systemctl", "stop", "ardents-endpoint.service").CombinedOutput(); err != nil {
			t.Errorf("restart refusal cleanup: %v: %s", err, output)
		}
	}()
	// A Type=simple start can acknowledge creation before the process refuses.
	output, startErr := exec.CommandContext(t.Context(), "systemctl", "start", "ardents-endpoint.service").CombinedOutput()
	installedStartupWait(t, func() ([]byte, bool) {
		current := installedStartupIdentity(t)
		return nil, current[0] == "0" && current[2] == "failed"
	})
	journal, err := exec.CommandContext(t.Context(), "journalctl", "-u", "ardents-endpoint.service", "--no-pager", "-o", "cat").CombinedOutput()
	if err != nil || bytes.Count(journal, refusalMarker) != bytes.Count(previousJournal, refusalMarker)+1 {
		t.Fatalf("restart lacks actual Startup refusal: start=%v output=%s journal-error=%v", startErr, output, err)
	}
	after := installedPairFileDigests(t, participant.base)
	if len(before) != len(after) {
		t.Fatal("restart refusal changed retained file inventory")
	}
	for path, digest := range before {
		if retained, exists := after[path]; !exists || retained != digest {
			t.Fatal("restart refusal changed retained file bytes", path)
		}
	}
	t.Log("new installed process refused before Startup handoff; retained file inventory and bytes unchanged")
}

func installedPairFileDigests(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	result := make(map[string][32]byte)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, readErr := io.Copy(hash, file)
		closeErr := file.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		var digest [32]byte
		copy(digest[:], hash.Sum(nil))
		result[path] = digest
		return nil
	})
	if err != nil {
		t.Fatal("retain actual installed file inventory", err)
	}
	return result
}
