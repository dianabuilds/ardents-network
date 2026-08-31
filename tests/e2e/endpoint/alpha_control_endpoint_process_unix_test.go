//go:build linux

package endpoint_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
)

const alphaControlEndpointCohort = "closed-cohort-1"
const alphaControlEndpointRelease = "alpha-1"

func TestAlphaControlReaderTwoFreshEnrolledEndpointsAgree(t *testing.T) {
	endpoint := buildArdents(t)
	control := buildControl(t)
	fixture := alphaControlBundle(t, endpoint, control)
	endpointRoots := [2]string{freshEndpointProcessRoot(t, "alpha-control-e1-"), freshEndpointProcessRoot(t, "alpha-control-e2-")}
	for index, root := range endpointRoots {
		assertAlphaControlPathAbsent(t, filepath.Join(root, "state", "ardents", "floors", "release-decision"), fmt.Sprintf("fresh Endpoint %d Release floor", index))
	}
	endpoints := [2]*liveEnrolledEndpoint{
		startLiveEnrolledEndpoint(t, fixture.artifact, fixture.input, endpointRoots[0], alphaControlEndpointCohort, alphaControlEndpointRelease),
		startLiveEnrolledEndpoint(t, fixture.artifact, fixture.input, endpointRoots[1], alphaControlEndpointCohort, alphaControlEndpointRelease),
	}
	if endpoints[0].command.Process.Pid == endpoints[1].command.Process.Pid || endpointRoots[0] == endpointRoots[1] {
		t.Fatalf("fresh Endpoints did not have distinct processes and roots: pids=%d/%d roots=%q/%q",
			endpoints[0].command.Process.Pid, endpoints[1].command.Process.Pid, endpointRoots[0], endpointRoots[1])
	}
	for index, running := range endpoints {
		if running.command.Path != fixture.artifact {
			t.Fatalf("fresh Endpoint %d executable = %q, want exact bundle artifact %q", index, running.command.Path, fixture.artifact)
		}
	}
	if endpoints[0].releaseOutcome != endpoints[1].releaseOutcome {
		t.Fatalf("fresh Endpoint release outcomes differ: %q/%q", endpoints[0].releaseOutcome, endpoints[1].releaseOutcome)
	}
	var endpointFloorInventories [2][]string
	for index, root := range endpointRoots {
		var err error
		endpointFloorInventories[index], err = canonicalReleaseFloorInventory(filepath.Join(root, "state", "ardents", "floors", "release-decision"))
		if err != nil {
			t.Fatalf("fresh Endpoint %d Release floor inventory: %v", index, err)
		}
	}
	if strings.Join(endpointFloorInventories[0], "\n") != strings.Join(endpointFloorInventories[1], "\n") {
		t.Fatalf("fresh Endpoint Release floor inventories differ:\n%s\n%s",
			strings.Join(endpointFloorInventories[0], "\n"), strings.Join(endpointFloorInventories[1], "\n"))
	}
	readerRoots := [2]string{filepath.Join(t.TempDir(), "reader"), filepath.Join(t.TempDir(), "reader")}
	assertDistinctAlphaControlRoots(t, endpointRoots, readerRoots)
	for index, root := range readerRoots {
		assertAlphaControlPathAbsent(t, root, fmt.Sprintf("fresh reader %d inspection root", index))
	}
	var reports [2]alphaControlParticipantReport
	var rawReports [2][]byte
	var readerFloorInventories [2][]string
	for index := range endpoints {
		if err := endpoints[index].command.Process.Signal(syscall.Signal(0)); err != nil {
			t.Fatalf("fresh Endpoint %d was not live during inspection: %v", index, err)
		}
		rawReports[index], reports[index] = inspectLiveEndpointControl(t, fixture, readerRoots[index])
		assertExactAlphaControlReport(t, fixture, reports[index])
		readerFloor := filepath.Join(readerRoots[index], "catalog", "catalog-floor.bin")
		if info, err := os.Lstat(readerFloor); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("fresh reader %d floor = %v / %v", index, info, err)
		}
		var err error
		readerFloorInventories[index], err = canonicalReleaseFloorInventory(filepath.Join(readerRoots[index], "release"))
		if err != nil {
			t.Fatalf("fresh reader %d Release floor inventory: %v", index, err)
		}
	}
	if strings.Join(readerFloorInventories[0], "\n") != strings.Join(readerFloorInventories[1], "\n") ||
		strings.Join(endpointFloorInventories[0], "\n") != strings.Join(readerFloorInventories[0], "\n") {
		t.Fatalf("four fresh Release floor inventories are not identical:\nendpoint=%s\nreader-a=%s\nreader-b=%s",
			strings.Join(endpointFloorInventories[0], "\n"), strings.Join(readerFloorInventories[0], "\n"), strings.Join(readerFloorInventories[1], "\n"))
	}
	if reports[0] != reports[1] || !bytes.Equal(rawReports[0], rawReports[1]) {
		t.Fatalf("fresh Endpoint reports differ:\n%s\n%s", rawReports[0], rawReports[1])
	}
	for index := range endpoints {
		endpoints[index].stop(t)
	}
}

func TestAlphaControlFreshLifecycleBindsCandidateIdentity(t *testing.T) {
	valid := [3]endpointLifecycleEvent{
		{Kind: "endpoint-lifecycle", State: "starting"},
		{Kind: "release-decision", Outcome: "release-accepted", Cohort: "closed-cohort-1", Release: "alpha-1"},
		{Kind: "endpoint-lifecycle", State: "ready", Attachment: "/tmp/endpoint.sock"},
	}
	if !validFreshEndpointStartup(valid, "closed-cohort-1", "alpha-1") {
		t.Fatal("exact fresh Endpoint lifecycle was rejected")
	}
	for name, mutate := range map[string]func(*[3]endpointLifecycleEvent){
		"no-update":     func(events *[3]endpointLifecycleEvent) { events[1].Outcome = "no-update" },
		"wrong-cohort":  func(events *[3]endpointLifecycleEvent) { events[1].Cohort = "other-cohort" },
		"wrong-release": func(events *[3]endpointLifecycleEvent) { events[1].Release = "other-release" },
		"absent-socket": func(events *[3]endpointLifecycleEvent) { events[2].Attachment = "" },
	} {
		t.Run(name, func(t *testing.T) {
			observed := valid
			mutate(&observed)
			if validFreshEndpointStartup(observed, "closed-cohort-1", "alpha-1") {
				t.Fatalf("invalid fresh Endpoint lifecycle was accepted: %+v", observed)
			}
		})
	}
}

func TestAlphaControlFreshFloorInventoryRejectsNonCanonicalPointer(t *testing.T) {
	root := t.TempDir()
	writeEnrollmentFile(t, filepath.Join(root, ".ardents-release-decision-v1"), []byte("ardents-release-decision-v1\n"), 0o600)
	if err := os.Mkdir(filepath.Join(root, "generations"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeEnrollmentFile(t, filepath.Join(root, "current"), []byte("not-a-generation\n"), 0o600)
	if _, err := canonicalReleaseFloorInventory(root); err == nil || !strings.Contains(err.Error(), "pointer") {
		t.Fatal("non-canonical Release floor pointer was accepted")
	}
}

func TestAlphaControlFallbackCleanupReapsProcessAndRejectsAttachmentResidue(t *testing.T) {
	attachment := filepath.Join(t.TempDir(), "endpoint.sock")
	writeEnrollmentFile(t, attachment, []byte("residue"), 0o600)
	ctx, cancel := context.WithCancel(t.Context())
	command := exec.CommandContext(ctx, "/bin/sleep", "30")
	command.WaitDelay = time.Second
	if err := command.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	running := &liveEnrolledEndpoint{command: command, cancel: cancel, attachment: attachment}
	err := running.cleanup()
	if err == nil || !strings.Contains(err.Error(), "attachment remains") {
		t.Fatalf("fallback cleanup residue result = %v", err)
	}
	if command.ProcessState == nil || !errors.Is(syscall.Kill(command.Process.Pid, 0), syscall.ESRCH) {
		t.Fatalf("fallback cleanup did not reap pid %d: state=%v", command.Process.Pid, command.ProcessState)
	}
}

type liveEnrolledEndpoint struct {
	command        *exec.Cmd
	scanner        *bufio.Scanner
	stderr         bytes.Buffer
	attachment     string
	releaseOutcome string
	cancel         context.CancelFunc
	finished       bool
}

type endpointLifecycleEvent struct {
	Kind, State, Outcome, Cohort, Release string
	Attachment                            string
}

func startLiveEnrolledEndpoint(t *testing.T, artifact, input, root, expectedCohort, expectedRelease string) *liveEnrolledEndpoint {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	running := &liveEnrolledEndpoint{command: exec.CommandContext(ctx, artifact, "endpoint", "enroll", input), cancel: cancel}
	running.command.WaitDelay = time.Second
	running.command.Env = endpointEnvironment(root)
	running.command.Stderr = &running.stderr
	stdout, err := running.command.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if err := running.command.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := running.cleanup(); err != nil {
			t.Errorf("fresh Endpoint fallback cleanup: %v", err)
		}
	})
	running.scanner = bufio.NewScanner(stdout)
	events := [3]endpointLifecycleEvent{}
	for index := range events {
		if !running.scanner.Scan() {
			t.Fatalf("fresh Endpoint startup event %d is absent: %v; stderr=%s", index, running.scanner.Err(), running.stderr.String())
		}
		if err := json.Unmarshal(running.scanner.Bytes(), &events[index]); err != nil {
			t.Fatalf("decode fresh Endpoint startup event %d: %v; line=%q", index, err, running.scanner.Text())
		}
	}
	if !validFreshEndpointStartup(events, expectedCohort, expectedRelease) {
		t.Fatalf("fresh Endpoint startup events = %+v; stderr=%s", events, running.stderr.String())
	}
	running.attachment = events[2].Attachment
	running.releaseOutcome = events[1].Outcome
	if info, err := os.Lstat(running.attachment); err != nil || info.Mode()&os.ModeSocket == 0 {
		t.Fatalf("fresh Endpoint attachment = %v / %v", info, err)
	}
	return running
}

func validFreshEndpointStartup(events [3]endpointLifecycleEvent, expectedCohort, expectedRelease string) bool {
	return events[0].Kind == "endpoint-lifecycle" && events[0].State == "starting" &&
		events[1].Kind == "release-decision" && events[1].Outcome == "release-accepted" &&
		events[1].Cohort == expectedCohort && events[1].Release == expectedRelease &&
		events[2].Kind == "endpoint-lifecycle" && events[2].State == "ready" && events[2].Attachment != ""
}

func (running *liveEnrolledEndpoint) stop(t *testing.T) {
	t.Helper()
	if err := running.command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if !running.scanner.Scan() {
		t.Fatalf("fresh Endpoint did not report stopped: %v; stderr=%s", running.scanner.Err(), running.stderr.String())
	}
	var stopped endpointLifecycleEvent
	if err := json.Unmarshal(running.scanner.Bytes(), &stopped); err != nil || stopped.Kind != "endpoint-lifecycle" || stopped.State != "stopped" {
		t.Fatalf("fresh Endpoint stop event = %q / %+v / %v", running.scanner.Text(), stopped, err)
	}
	waitErr := running.command.Wait()
	running.finished = true
	running.cancel()
	if waitErr != nil {
		t.Fatalf("fresh Endpoint exit: %v; stderr=%s", waitErr, running.stderr.String())
	}
	if _, err := os.Lstat(running.attachment); !os.IsNotExist(err) {
		t.Fatalf("fresh Endpoint attachment remains after stop: %v", err)
	}
}

func (running *liveEnrolledEndpoint) cleanup() error {
	if running.finished {
		return nil
	}
	var failures []error
	pid := running.command.Process.Pid
	if err := running.command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		failures = append(failures, fmt.Errorf("kill pid %d: %w", pid, err))
	}
	running.cancel()
	waitErr := running.command.Wait()
	running.finished = true
	state := running.command.ProcessState
	var exitError *exec.ExitError
	if state == nil {
		failures = append(failures, fmt.Errorf("pid %d has no reaped process state", pid))
	} else if status := state.Sys().(syscall.WaitStatus); waitErr != nil &&
		(!errors.As(waitErr, &exitError) || !status.Signaled() || status.Signal() != syscall.SIGKILL) {
		failures = append(failures, fmt.Errorf("wait pid %d: %w", pid, waitErr))
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		failures = append(failures, fmt.Errorf("pid %d remains after cleanup: %v", pid, err))
	}
	if running.attachment != "" {
		if _, err := os.Lstat(running.attachment); !os.IsNotExist(err) {
			failures = append(failures, fmt.Errorf("attachment remains after cleanup: %s: %v", running.attachment, err))
		}
	}
	return errors.Join(failures...)
}

type alphaControlParticipantReport struct {
	Schema            string `json:"schema"`
	Catalog           string `json:"catalog"`
	CatalogIdentity   string `json:"catalog_identity"`
	CatalogCohort     string `json:"catalog_cohort"`
	CatalogGeneration uint64 `json:"catalog_generation"`
	CatalogNotBefore  string `json:"catalog_not_before"`
	CatalogNotAfter   string `json:"catalog_not_after"`
	Components        [3]struct {
		Class      alphacontrol.ComponentClass `json:"class"`
		Outcome    string                      `json:"outcome"`
		RootID     string                      `json:"root_id"`
		Generation uint64                      `json:"generation"`
		Digest     string                      `json:"digest"`
		NotBefore  string                      `json:"not_before"`
		NotAfter   string                      `json:"not_after"`
	} `json:"components"`
	Release         string `json:"release"`
	ReleaseIdentity string `json:"release_identity"`
	BuildIdentity   string `json:"build_identity"`
	ArtifactDigest  string `json:"artifact_digest"`
	ProtocolPhase   string `json:"protocol_phase"`
	NetworkID       string `json:"network_id"`
	NetworkEpoch    uint64 `json:"network_epoch"`
	NetworkDigest   string `json:"network_digest"`
	NetworkProfile  string `json:"network_profile"`
}

func inspectLiveEndpointControl(t *testing.T, fixture alphaControlBundleFixture, stateRoot string) ([]byte, alphaControlParticipantReport) {
	t.Helper()
	arguments := []string{"inspect-bundle", "--enrollment", fixture.input, "--artifact", fixture.artifact,
		"--state-root", stateRoot, "--at", fixture.now.Format(time.RFC3339)}
	output, err := exec.Command(fixture.control, arguments...).CombinedOutput()
	if err != nil {
		t.Fatalf("inspect live fresh Endpoint control: %v\n%s", err, output)
	}
	var report alphaControlParticipantReport
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatalf("decode live fresh Endpoint report: %v\n%s", err, output)
	}
	return output, report
}

func assertExactAlphaControlReport(t *testing.T, fixture alphaControlBundleFixture, report alphaControlParticipantReport) {
	t.Helper()
	if report.Schema != "ardents-alpha-control-report-v1" || report.Catalog != "accepted" || report.CatalogCohort != "closed-cohort-1" ||
		report.CatalogGeneration != 1 || report.CatalogNotBefore != fixture.now.Add(-time.Minute).Format(time.RFC3339) ||
		report.CatalogNotAfter != fixture.now.Add(20*time.Minute).Format(time.RFC3339) || report.Release != "release-accepted" ||
		report.ReleaseIdentity != "ardents-alpha-1" || report.BuildIdentity != "test-build" || report.ProtocolPhase != "required" ||
		report.NetworkID != hex.EncodeToString(fixture.network[:]) || report.NetworkEpoch != 1 ||
		report.NetworkDigest != hex.EncodeToString(fixture.networkDigest[:]) || report.NetworkProfile != "h3-role-probe-v1" {
		t.Fatalf("fresh Endpoint control identities = %+v", report)
	}
	assertControlDigest(t, filepath.Join(fixture.bundle, "catalog.ac1"), report.CatalogIdentity)
	assertControlDigest(t, fixture.artifact, report.ArtifactDigest)
	componentNames := [3]string{"release", "network", "compatibility"}
	for index, component := range report.Components {
		if component.Class != alphacontrol.ComponentClass(index+1) || component.Outcome != "accepted" || component.Generation != 1 ||
			component.NotBefore != fixture.now.Add(-time.Minute).Format(time.RFC3339) || component.NotAfter != fixture.now.Add(20*time.Minute).Format(time.RFC3339) {
			t.Fatalf("fresh Endpoint component %d = %+v", index, component)
		}
		assertControlDigest(t, filepath.Join(fixture.bundle, componentNames[index]+".pub"), component.RootID)
		assertControlDigest(t, filepath.Join(fixture.bundle, componentNames[index]+".ac1"), component.Digest)
	}
}

func assertAlphaControlPathAbsent(t *testing.T, path, purpose string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("%s was not absent before its first observation: %v", purpose, err)
	}
}

func canonicalReleaseFloorInventory(root string) ([]string, error) {
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("Release floor root is not a real directory: %w", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if strings.Join(names, "\n") != ".ardents-release-decision-v1\ncurrent\ngenerations" {
		return nil, fmt.Errorf("Release floor top-level inventory is not canonical: %q", names)
	}
	marker, err := os.ReadFile(filepath.Join(root, ".ardents-release-decision-v1"))
	if err != nil || string(marker) != "ardents-release-decision-v1\n" {
		return nil, fmt.Errorf("Release floor marker is invalid: %w", err)
	}
	pointer, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
		return nil, fmt.Errorf("read Release floor pointer: %w", err)
	}
	generation := strings.TrimSuffix(string(pointer), "\n")
	if string(pointer) != generation+"\n" || !lowerSHA256(generation) {
		return nil, errors.New("Release floor pointer is not one canonical generation")
	}
	generationEntries, err := os.ReadDir(filepath.Join(root, "generations"))
	if err != nil || len(generationEntries) == 0 {
		return nil, fmt.Errorf("Release floor generation inventory is empty: %w", err)
	}
	currentFound := false
	for _, entry := range generationEntries {
		if !lowerSHA256(entry.Name()) || !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("Release floor generation is not canonical: %q", entry.Name())
		}
		if entry.Name() == generation {
			currentFound = true
		}
		generationRoot := filepath.Join(root, "generations", entry.Name())
		children, readErr := os.ReadDir(generationRoot)
		if readErr != nil || len(children) != 2 || children[0].Name() != "roots" || children[1].Name() != "state.bin" ||
			!children[0].IsDir() || !children[1].Type().IsRegular() {
			return nil, fmt.Errorf("Release floor generation %q has a non-canonical inventory: %w", entry.Name(), readErr)
		}
		rootEntries, readErr := os.ReadDir(filepath.Join(generationRoot, "roots"))
		if readErr != nil || len(rootEntries) == 0 {
			return nil, fmt.Errorf("Release floor generation %q has no root archive: %w", entry.Name(), readErr)
		}
		for _, archived := range rootEntries {
			version := strings.TrimSuffix(archived.Name(), ".root.json")
			parsed, parseErr := strconv.ParseUint(version, 10, 63)
			if parseErr != nil || parsed == 0 || archived.Name() != version+".root.json" || !archived.Type().IsRegular() {
				return nil, fmt.Errorf("Release floor archived root is not canonical: %q", archived.Name())
			}
		}
	}
	if !currentFound {
		return nil, errors.New("Release floor pointer does not name a retained generation")
	}
	var inventory []string
	err = filepath.Walk(root, func(path string, item os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		relative = filepath.ToSlash(relative)
		if item.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("Release floor inventory contains a symlink: %s", relative)
		}
		if item.IsDir() {
			inventory = append(inventory, "d  "+relative)
			return nil
		}
		if !item.Mode().IsRegular() {
			return fmt.Errorf("Release floor inventory contains a non-regular entry: %s", relative)
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		digest := sha256.Sum256(raw)
		inventory = append(inventory, "f "+hex.EncodeToString(digest[:])+"  "+relative)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(inventory)
	return inventory, nil
}

func lowerSHA256(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
func assertControlDigest(t *testing.T, path, got string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256(raw)
	if got != hex.EncodeToString(want[:]) {
		t.Fatalf("control identity for %s = %q, want %x", path, got, want)
	}
}
func freshEndpointProcessRoot(t *testing.T, pattern string) string {
	t.Helper()
	root, err := os.MkdirTemp("/tmp", pattern)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { removeEndpointProcessTree(t, root) })
	return root
}

func assertDistinctAlphaControlRoots(t *testing.T, endpointRoots, readerRoots [2]string) {
	t.Helper()
	seen := make(map[string]struct{}, 10)
	for _, root := range endpointRoots {
		for _, child := range []string{"config", "state", "cache", "runtime"} {
			path := filepath.Join(root, child, "ardents")
			if info, err := os.Lstat(path); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				t.Fatalf("fresh Endpoint XDG root %s = %v / %v", path, info, err)
			}
			if _, duplicate := seen[path]; duplicate {
				t.Fatalf("alpha control process root is duplicated: %s", path)
			}
			seen[path] = struct{}{}
		}
	}
	for _, root := range readerRoots {
		if _, duplicate := seen[root]; duplicate {
			t.Fatalf("alpha control reader root is duplicated: %s", root)
		}
		seen[root] = struct{}{}
	}
}
