package state_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestSourceProcessPrivateRuntimeDiagnostics(t *testing.T) {
	fixture := writeVerifierFixtureAt(t, time.Now().Unix())
	ardents := buildProductCommand(t, "ardents")
	node := buildProductCommand(t, "ardents-node")
	material := filepath.Join(t.TempDir(), "material.bin")
	if err := os.WriteFile(material, fixture.materializations[0], 0600); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	acceptFixtureWithCommand(t, ardents, fixture, material, root)
	clientAuthority := processAuthority(t, 0x31, "diagnostic-client-root")
	clientLeaf := processLeaf(t, clientAuthority, 0x32, "endpoint.test", false)
	serverAuthority := processAuthority(t, 0x41, "diagnostic-source-root")
	serverLeaf := processLeaf(t, serverAuthority, 0x43, "source.test", true)
	plan := writeServerPlan(t, fixture, root, freeProcessAddress(t), serverLeaf, clientAuthority.rootPath, clientLeaf.pin)
	private, err := os.MkdirTemp("", "ardents-live-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(private)
	socket := filepath.Join(private, "source.sock")
	command := exec.Command(node, "source", "--config", plan)
	command.Env = append(os.Environ(), "ARDENTS_DEBUG_SOCKET="+socket)
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	joined := false
	defer func() {
		if !joined {
			command.Process.Kill()
			command.Wait()
		}
	}()
	ready := make(chan bool, 1)
	go func() {
		reader := bufio.NewReader(stdout)
		line, err := reader.ReadBytes('\n')
		var event struct{ Kind string }
		ready <- err == nil && json.Unmarshal(line, &event) == nil && event.Kind == "source-ready"
		io.Copy(io.Discard, reader)
	}()
	select {
	case valid := <-ready:
		if !valid {
			command.Process.Kill()
			command.Wait()
			joined = true
			t.Fatalf("Source did not report readiness: %s", stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Source readiness timeout")
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	client := &http.Client{Transport: transport, Timeout: 4 * time.Second}
	defer client.CloseIdleConnections()
	for _, path := range []string{"/runtime", "/profile/goroutine", "/profile/heap", "/profile/cpu?seconds=1", "/trace?seconds=1"} {
		response, err := client.Get("http://diagnostic" + path)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != 200 || len(body) == 0 {
			t.Fatalf("live %s: status %d bytes %d: %v", path, response.StatusCode, len(body), err)
		}
		t.Logf("actual Source process %s: %d bytes", path, len(body))
	}
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	terminal := make(chan error, 1)
	go func() { terminal <- command.Wait() }()
	select {
	case err := <-terminal:
		joined = true
		if err != nil {
			t.Fatalf("Source shutdown: %v: %s", err, stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Source shutdown did not join diagnostics")
	}
	if _, err := os.Lstat(socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket survived process shutdown: %v", err)
	}
}
