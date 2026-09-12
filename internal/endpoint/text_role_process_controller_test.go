//go:build linux

package endpoint

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node"
)

type textRoleProcess struct {
	capture func(string)
	dump    func(string)
	stop    func() error
	output  string
	heap    bool
}

func startTextRoleProcess(t *testing.T, index int, config node.Config, root string, testName ...string) *textRoleProcess {
	t.Helper()
	facts, err := config.Current()
	if err != nil {
		t.Fatal(err)
	}
	fixture, ok := facts.(textNetworkDutyFixture)
	if !ok {
		t.Fatal("unexpected public State seam")
	}
	view, ok := config.CurrentClosedRoute()
	if !ok {
		t.Fatal("missing profile")
	}
	input := textRoleProcessInput{Snapshot: fixture.snapshot, View: view, Key: config.IdentityKey, StateRoot: config.LocalRoleStateRoot}
	switch {
	case config.ClosedIssuer.Root != "":
		v := config.ClosedIssuer
		input.Role, input.Root, input.AdmissionRoot, input.Certificates, input.Limit, input.Drain = "issuer", v.Root, v.AdmissionRoot, v.Certificate.Certificate, v.ConnectionLimit, v.DrainTimeout
	case config.ClosedResolution.Root != "":
		v := config.ClosedResolution
		input.Role, input.Root, input.AdmissionRoot, input.Certificates, input.Limit, input.Drain = "resolution", v.Root, v.AdmissionRoot, v.Certificate.Certificate, v.ConnectionLimit, v.DrainTimeout
	case config.ClosedIntroduction.AdmissionRoot != "":
		v := config.ClosedIntroduction
		input.Role, input.AdmissionRoot, input.Certificates, input.Limit, input.Drain = "introduction", v.AdmissionRoot, v.Certificate.Certificate, v.ConnectionLimit, v.DrainTimeout
	case config.ClosedDataJoin.AdmissionRoot != "":
		v := config.ClosedDataJoin
		input.Role, input.AdmissionRoot, input.Certificates, input.Limit, input.Drain = "join", v.AdmissionRoot, v.Certificate.Certificate, v.ConnectionLimit, v.DrainTimeout
	case config.ClosedForwarding.Root != "":
		v := config.ClosedForwarding
		input.Role, input.Root, input.Certificates, input.Limit, input.Drain = "forwarding", v.Root, v.Certificate.Certificate, v.ConnectionLimit, v.DrainTimeout
	default:
		t.Fatal("missing observed role")
	}
	input.Output = filepath.Join(root, fmt.Sprintf("%02d-%s", index, input.Role))
	if err := os.Mkdir(input.Output, 0700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(input.Output, "input.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	clear(raw)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	target := "^TestTextPublicationIsolatedRoleObservations$"
	if len(testName) == 1 {
		target = testName[0]
	}
	command := exec.CommandContext(ctx, binary, "-test.run="+target, "-test.timeout=110s")
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "ARDENTS_TEXT_ROLE_CHILD=" + path}
	stdin, err := command.StdinPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	joined := false
	t.Cleanup(func() {
		if !joined {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
		cancel()
	})
	reader := bufio.NewReader(stdout)
	expect := func(want string) {
		t.Helper()
		line, err := reader.ReadString('\n')
		if err != nil || strings.TrimSpace(line) != want {
			t.Fatalf("role%d expected %q got %q: %v", index, want, line, err)
		}
	}
	expect("role-ready")
	process := &textRoleProcess{output: input.Output, heap: true}
	process.capture = func(phase string) {
		t.Helper()
		if _, err := fmt.Fprintln(stdin, "state "+phase); err != nil {
			t.Fatal(err)
		}
		expect("role-state " + phase)
	}
	process.dump = func(phase string) {
		t.Helper()
		if _, err := fmt.Fprintln(stdin, phase); err != nil {
			t.Fatal(err)
		}
		expect("role-dump " + phase)
	}
	process.stop = func() error {
		if joined {
			return nil
		}
		if _, err := fmt.Fprintln(stdin, "stop"); err != nil {
			return err
		}
		expect("role-stopped")
		_ = stdin.Close()
		_, readErr := io.Copy(io.Discard, reader)
		err := command.Wait()
		joined = true
		cancel()
		if err != nil {
			return fmt.Errorf("role%d: %w; %s", index, err, stderr.String())
		}
		return readErr
	}
	return process
}
