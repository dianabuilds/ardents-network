package tooling

import (
	"bufio"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func runToolTracerRole(runID, role, controlDirectory, evidenceDirectory string) error {
	if !validToolRole(runID, role) {
		return errors.New("invalid tooling tracer role input")
	}
	result := newToolingRoleResult(runID, "tracer-"+role)
	finish := func(runErr error) error { return finishToolingRole(evidenceDirectory, &result, runErr) }
	capabilities, err := effectiveCapabilities()
	result.EffectiveCapabilities = capabilities
	if err != nil || !hasOnlyEffectiveCapability(capabilities) {
		return finish(errors.Join(err, errors.New("tracer role must have no effective capabilities")))
	}
	listener, err := net.Listen("tcp", ":"+toolingTCPPort)
	if err != nil {
		return finish(err)
	}
	defer listener.Close()
	if err := writeToolingReady(evidenceDirectory, runID, result.Role); err != nil {
		return finish(err)
	}
	if err := waitForControlFile(filepath.Join(controlDirectory, "start"), 20*time.Second); err != nil {
		return finish(err)
	}
	peer, peerAddress := "alpha", "tracer-alpha:"+toolingTCPPort
	if role == "alpha" {
		peer, peerAddress = "beta", "tracer-beta:"+toolingTCPPort
	}
	inbound := make(chan error, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			inbound <- acceptErr
			return
		}
		defer connection.Close()
		inbound <- exchangeToolingMarker(connection, runID, role, peer)
	}()
	connection, err := dialSmokePeer(peerAddress, 10*time.Second)
	if err != nil {
		return finish(err)
	}
	outboundErr := exchangeToolingMarker(connection, runID, role, peer)
	_ = connection.Close()
	if outboundErr != nil {
		return finish(outboundErr)
	}
	select {
	case err := <-inbound:
		if err != nil {
			return finish(err)
		}
	case <-time.After(10 * time.Second):
		return finish(errors.New("timed out waiting for synthetic tracer peer"))
	}
	result.ObservedPeer = peer
	if err := writeBoundedJSON(filepath.Join(evidenceDirectory, "exchange.json"), map[string]string{
		"schema_version": toolingRoleSchema, "run_id": runID, "role": result.Role, "status": "exchanged",
	}); err != nil {
		return finish(err)
	}
	if err := waitForControlFile(filepath.Join(controlDirectory, "stop"), 20*time.Second); err != nil {
		return finish(err)
	}
	return finish(nil)
}

func newToolingRoleResult(runID, role string) toolingRoleResult {
	return toolingRoleResult{SchemaVersion: toolingResultSchema, RunID: runID, Role: role, Status: "failed"}
}

func finishToolingRole(evidenceDirectory string, result *toolingRoleResult, runErr error) error {
	if runErr == nil {
		result.Status = "passed"
	} else {
		result.Status = "failed"
		result.Failure = runErr.Error()
	}
	if evidenceErr := writeBoundedJSON(filepath.Join(evidenceDirectory, "result.json"), result); evidenceErr != nil {
		return errors.Join(runErr, evidenceErr)
	}
	return runErr
}

func validToolRole(runID, role string) bool {
	return runIDPattern.MatchString(runID) && (role == "alpha" || role == "beta")
}

func writeToolingReady(evidenceDirectory, runID, role string) error {
	return writeBoundedJSON(filepath.Join(evidenceDirectory, "ready.json"), map[string]string{
		"schema_version": toolingRoleSchema, "run_id": runID, "role": role, "status": "ready",
	})
}

func waitForControlFile(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() && info.Size() <= 64 {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return fmt.Errorf("control file %s did not arrive", filepath.Base(path))
}

func effectiveCapabilities() (string, error) {
	file, err := os.Open("/proc/self/status")
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if value, found := strings.CutPrefix(scanner.Text(), "CapEff:\t"); found {
			return strings.TrimSpace(value), nil
		}
	}
	return "", errors.Join(scanner.Err(), errors.New("CapEff is absent from process status"))
}

func hasOnlyEffectiveCapability(value string, bits ...uint) bool {
	want := new(big.Int)
	for _, bit := range bits {
		want.SetBit(want, int(bit), 1)
	}
	actual := new(big.Int)
	if _, ok := actual.SetString(strings.TrimSpace(value), 16); !ok {
		return false
	}
	return actual.Cmp(want) == 0
}

func observeTools(lockPath string, names ...string) (toolBundle, map[string]observedTool, error) {
	identity, err := readToolLock(lockPath)
	if err != nil {
		return toolBundle{}, nil, err
	}
	versionOutput := make(map[string]string)
	if slices.Contains(names, "tc") {
		output, err := exec.Command("/usr/sbin/tc", "-Version").CombinedOutput()
		if err != nil {
			return toolBundle{}, nil, fmt.Errorf("observe tc version: %w: %s", err, strings.TrimSpace(string(output)))
		}
		versionOutput["tc"] = capturedVersion(tcVersionPattern, string(output))
	}
	if slices.Contains(names, "tcpdump") || slices.Contains(names, "libpcap") {
		output, err := exec.Command("/usr/bin/tcpdump", "--version").CombinedOutput()
		if err != nil {
			return toolBundle{}, nil, fmt.Errorf("observe tcpdump version: %w: %s", err, strings.TrimSpace(string(output)))
		}
		versionOutput["tcpdump"] = capturedVersion(tcpdumpVersionPattern, string(output))
		versionOutput["libpcap"] = capturedVersion(libpcapVersionPattern, string(output))
	}
	observed := make(map[string]observedTool, len(names))
	for _, name := range names {
		expected, found := identity.Tools[name]
		if !found {
			return toolBundle{}, nil, fmt.Errorf("tool lock does not name %s", name)
		}
		digest, err := hashRegularFile(expected.Path)
		if err != nil {
			return toolBundle{}, nil, err
		}
		observation := toolObservation{Name: name, Version: versionOutput[name], Path: expected.Path, SHA256: digest}
		if err := identity.verifyObservation(observation); err != nil {
			return toolBundle{}, nil, err
		}
		observed[name] = observedTool{Version: observation.Version, Path: observation.Path, SHA256: observation.SHA256}
	}
	return identity, observed, nil
}
