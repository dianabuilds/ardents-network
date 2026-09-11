//go:build linux && text_worker_installed

package state_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev1/administration"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// Root orchestrates real commands outside the Endpoint service. The ordinary
// Endpoint itself remains the unprivileged systemd MainPID required by launch.
func TestInstalledClosedTextCommandsThroughNodeProcesses(t *testing.T) {
	if os.Geteuid() != 0 || os.Getenv("ARDENTS_TEXT_COMMAND_QUALIFICATION") != "1" || os.Getenv("ARDENTS_E2E_COMMAND_ROOT") == "" {
		t.Fatal("invalid environment: select the dedicated installed root command profile with prebuilt commands")
	}
	for _, carrier := range []string{"ardents-carrier-tcp-tls-v2", "ardents-carrier-quic-v2"} {
		t.Run(carrier, func(t *testing.T) {
			authority := createClosedCommandAuthority(t, [32]byte{1})
			testClosedIssuerProvisioningParticipant(t, carrier, 16, authority.Public, nil, func(config state.Config, binary, resolutionRoot string, sourcePlan map[string]any) {
				runInstalledClosedTextParticipant(t, config, binary, resolutionRoot, authority, sourcePlan)
			})
		})
	}
}

func runInstalledClosedTextParticipant(t *testing.T, config state.Config, binary, resolutionRoot string, authority closedCommandAuthority, sourcePlan map[string]any) {
	t.Helper()
	account, err := user.Lookup("ardents-endpoint")
	if err != nil {
		t.Fatal(err)
	}
	uid, err := strconv.Atoi(account.Uid)
	if err != nil {
		t.Fatal(err)
	}
	gid, err := strconv.Atoi(account.Gid)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	instance := acquireCommandServiceInstance(t, binary, config.NetworkID, now, now.Add(time.Hour))
	directory, err := os.MkdirTemp("", "ardents-command-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	path := func(name string) string { return filepath.Join(directory, name) }
	if err := os.Mkdir(path("tokens"), 0700); err != nil {
		t.Fatal(err)
	}
	clock := path("clock")
	t.Cleanup(startClockObserver(t, clock))
	stateOwner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	view, viewErr := stateOwner.CurrentClosedRoute()
	closeErr := stateOwner.Close()
	if viewErr != nil || closeErr != nil {
		t.Fatalf("read current profile: %v / %v", viewErr, closeErr)
	}
	public := config.ClosedProfileAuthority
	plan := map[string]any{"schema": "ardents-headless-runtime-v2", "network_state_root": config.Root,
		"entry_state_root": path("entry"), "local_role_state_root": path("roles"), "text_token_root": path("tokens"),
		"publication_root": path("publications"), "service_instance_root": instance, "application_socket": path("reader.sock"), "administration_socket": path("publisher.sock"),
		"time_confidence_file": clock, "network_id": hex.EncodeToString(config.NetworkID[:]), "network_authorities": []string{hex.EncodeToString(public)},
		"network_threshold": 1, "network_profile": "ardents-route-v3", "closed_profile_authority": hex.EncodeToString(public),
		"broker_id": identifierNode(91), "connection_principal": identifierNode(92), "administration_principal": identifierNode(93)}
	for _, role := range []string{"reader", "publisher"} {
		plan[role+"_permission"] = map[string]any{"request_path": path(role + ".request"), "response_path": path(role + ".response"), "maxima": [3]uint32{512, 512, 512}}
	}
	plan["network_source_plan"] = installedCommandSourcePlan(t, directory, sourcePlan)
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	planPath := path("runtime.json")
	if err := os.WriteFile(planPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{directory, config.Root, filepath.Dir(instance)} {
		// Only these test-created roots change ownership; other Node/Custody roots
		// remain private to the root orchestrator. The shared test parent is traversable.
		if root != directory {
			if err := os.Chmod(filepath.Dir(root), 0711); err != nil {
				t.Fatal(err)
			}
		}
		if err := filepath.WalkDir(root, func(name string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("unexpected symlink in private test root")
			}
			return os.Chown(name, uid, gid)
		}); err != nil {
			t.Fatal(err)
		}
	}
	assertInstalledCommandEqualSourceFamiliesRefuse(t, binary, plan, plan["network_source_plan"].(string), path("equal-family-runtime.json"), uid, gid)
	t.Log("completed: Service Instance acquired through commands and Endpoint inputs prepared")
	invocation := startInstalledCommandEndpoint(t, binary, planPath)
	for _, role := range []string{"reader", "publisher"} {
		request := waitInstalledCommandRequest(t, invocation, path(role+".request"))
		response := authority.issue(t, request)
		staged := path(role + ".response.pending")
		if err := os.WriteFile(staged, response, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chown(staged, uid, gid); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(staged, path(role+".response")); err != nil {
			t.Fatal(err)
		}
	}
	waitInstalledCommandSockets(t, invocation, path("reader.sock"), path("publisher.sock"))
	t.Log("completed: ordinary Endpoint started with separately issued reader/publisher permissions")
	body := bytes.Repeat([]byte("x"), 64<<10)
	document := path("document.txt")
	if err := os.WriteFile(document, body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(document, uid, gid); err != nil {
		t.Fatal(err)
	}
	textBinary := buildCommand(t, "ardents-text")
	// The UI reopens stdio for polling. User-owned pipeline endpoints keep
	// that check real; pipefail retains the actual text command exit status.
	run := func(stage string, input []byte, args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
		defer cancel()
		output, diagnostic, err := installedCommandExecAs(ctx, input, uid, gid, "bash", append([]string{"-o", "pipefail", "-c", `cat | "$@" | cat`, "ardents-text-command", textBinary}, args...)...)
		if err != nil {
			t.Fatalf("ordinary text stage %s (%s) failed: %v / %s", stage, args[0], err, diagnostic)
		}
		return output
	}
	publicationStarted := time.Now()
	if output := run("publish", nil, "publish", path("publisher.sock"), document); len(output) != 0 {
		t.Fatal("publish produced unexpected output")
	}
	firstPublication := readInstalledCommandDescriptor(t, resolutionRoot, view.Profile)
	t.Log("completed: publication command and independently verified signed Descriptor")
	destination := run("published Link", nil, "link", path("publisher.sock"))
	if len(destination) < 2 || bytes.Count(destination, []byte{'\n'}) != 1 || destination[len(destination)-1] != '\n' {
		t.Fatal("invalid Link output")
	}
	if actual := run("initial read", destination, "read", path("reader.sock")); !bytes.Equal(actual, body) {
		t.Fatal("ordinary command document mismatch")
	}
	t.Log("completed: initial exact 64 KiB document read")
	observeInstalledCommandRefresh(t, resolutionRoot, view.Profile, firstPublication, publicationStarted)
	t.Log("completed: observed signed Descriptor refresh and elapsed overlap")
	if actual := run("read after refresh", destination, "read", path("reader.sock")); !bytes.Equal(actual, body) {
		t.Fatal("document changed after elapsed refresh")
	}
	t.Log("completed: exact document read through the original Link after refresh")
	withdrawal, cancelWithdrawal := context.WithTimeout(t.Context(), 15*time.Second)
	outcome, withdrawalErr := administration.Request(withdrawal, path("publisher.sock"), administration.Withdraw)
	cancelWithdrawal()
	if withdrawalErr != nil || outcome != administration.Withdrawn {
		t.Fatalf("ordinary Endpoint withdrawal: %s, %v", outcome, withdrawalErr)
	}
	refused, cancelRefused := context.WithTimeout(t.Context(), 20*time.Second)
	output, diagnostic, linkErr := installedCommandExecAs(refused, nil, uid, gid, "bash", "-o", "pipefail", "-c", `cat | "$@" | cat`, "ardents-text-command", textBinary, "link", path("publisher.sock"))
	cancelRefused()
	var exit *exec.ExitError
	if !errors.As(linkErr, &exit) || exit.ExitCode() != 2 || len(output) != 0 || strings.TrimSpace(string(diagnostic)) != "text operation unavailable" {
		t.Fatalf("withdrawn publication still exposed a Link or failed for another reason: %v / %s", linkErr, diagnostic)
	}
	active := strings.TrimSpace(string(installedCommandTool(t, "systemctl", "show", "ardents-endpoint.service", "-p", "ActiveState", "--value")))
	retained := strings.TrimSpace(string(installedCommandTool(t, "systemctl", "show", "ardents-endpoint.service", "-p", "InvocationID", "--value")))
	if active != "active" || retained != invocation {
		t.Fatal("Link refusal was accompanied by Endpoint loss or replacement")
	}
	workers := installedCommandTool(t, "systemctl", "list-units", "--state=active,activating,deactivating", "--no-legend", "ardents-text-reader@*.service", "ardents-text-publisher@*.service")
	if len(bytes.TrimSpace(workers)) != 0 {
		t.Fatalf("withdrawal retained worker units: %s", workers)
	}
	t.Log("completed: withdrawal, exact Link refusal, same live Endpoint, and no retained workers")
}

// Equal Source families are forbidden before State opens or either configured
// Source can be dialled. This runs the ordinary Endpoint command under its
// unprivileged account against the same prepared State/time/provisioning chain
// as the successful command journey; it does not substitute a direct fixture.
func assertInstalledCommandEqualSourceFamiliesRefuse(t *testing.T, binary string, runtime map[string]any, sourcePath, badRuntime string, uid, gid int) {
	t.Helper()
	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	var sourcePlan map[string]any
	if err := json.Unmarshal(raw, &sourcePlan); err != nil {
		t.Fatalf("decode prepared Source plan: %v", err)
	}
	sources, ok := sourcePlan["sources"].([]any)
	if !ok || len(sources) != 2 {
		t.Fatal("prepared Source plan has no exact Source pair")
	}
	firstSource, firstOK := sources[0].(map[string]any)
	secondSource, secondOK := sources[1].(map[string]any)
	if !firstOK || !secondOK {
		t.Fatal("prepared Source plan entries are invalid")
	}
	first, second := firstSource["family"], secondSource["family"]
	if first == nil || second == nil || first == second {
		t.Fatal("prepared Source plan did not retain two distinct permitted families")
	}
	secondSource["family"] = first
	badSource, err := json.Marshal(sourcePlan)
	if err != nil {
		t.Fatal(err)
	}
	badSourcePath := filepath.Join(filepath.Dir(sourcePath), "equal-family-sources.json")
	if err := os.WriteFile(badSourcePath, badSource, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(badSourcePath, uid, gid); err != nil {
		t.Fatal(err)
	}
	badPlan := make(map[string]any, len(runtime))
	for name, value := range runtime {
		badPlan[name] = value
	}
	badPlan["network_source_plan"] = badSourcePath
	raw, err = json.Marshal(badPlan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badRuntime, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(badRuntime, uid, gid); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	output, diagnostic, commandErr := installedCommandExecAs(ctx, nil, uid, gid, "bash", "-o", "pipefail", "-c", `cat | "$@" | cat`, "endpoint-source-plan", binary, "endpoint", "headless", badRuntime)
	if commandErr == nil || len(output) != 0 || !strings.Contains(string(diagnostic), "source identities, families, handles, addresses, and keys must be distinct") {
		t.Fatalf("equal Source families reached ordinary Endpoint work: output=%q diagnostic=%q err=%v", output, diagnostic, commandErr)
	}
}

func installedCommandTool(t *testing.T, name string, arguments ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	output, diagnostic, err := installedCommandExec(ctx, nil, name, arguments...)
	if err != nil {
		t.Fatalf("qualification command %s failed: %v / %s", name, err, diagnostic)
	}
	return output
}

// installedCommandRefreshFailure stops only an already failing Endpoint and
// returns its exact invocation journal. The refresh oracle calls it after its
// fixed observation window has elapsed, so this diagnostic cannot alter a
// successful publication, its schedule, or predecessor validity.
func installedCommandRefreshFailure(t *testing.T) string {
	t.Helper()
	installedCommandTool(t, "systemctl", "kill", "--signal=QUIT", "ardents-endpoint.service")
	time.Sleep(250 * time.Millisecond)
	return string(installedCommandTool(t, "journalctl", "--no-pager", "-o", "cat", "-u", "ardents-endpoint.service"))
}

func startInstalledCommandEndpoint(t *testing.T, binary, plan string) string {
	t.Helper()
	if strings.TrimSpace(string(installedCommandTool(t, "systemctl", "show", "ardents-endpoint.service", "-p", "ActiveState", "--value"))) != "inactive" {
		t.Fatal("invalid environment: Endpoint must be inactive")
	}
	if strings.TrimSpace(string(installedCommandTool(t, "systemctl", "show", "ardents-endpoint.service", "-p", "DropInPaths", "--value"))) != "" {
		t.Fatal("invalid environment: Endpoint has drop-ins")
	}
	const unit = "/run/systemd/system/ardents-endpoint.service"
	info, err := os.Lstat(unit)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatal("invalid environment: existing temporary qualification unit required")
	}
	previous, err := os.ReadFile(unit)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if _, diagnostic, err := installedCommandExec(ctx, nil, "systemctl", "stop", "ardents-endpoint.service"); err != nil {
			t.Errorf("stop Endpoint: %v / %s", err, diagnostic)
		}
		if err := os.WriteFile(unit, previous, info.Mode().Perm()); err != nil {
			t.Error(err)
			return
		}
		restoreCtx, restoreCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer restoreCancel()
		if _, diagnostic, err := installedCommandExec(restoreCtx, nil, "systemctl", "daemon-reload"); err != nil {
			t.Errorf("restore Endpoint unit: %v / %s", err, diagnostic)
		}
	})
	content := fmt.Sprintf("[Unit]\nDescription=Ardents command qualification\n[Service]\nType=exec\nUser=ardents-endpoint\nGroup=ardents-endpoint\nExecStart=%s endpoint headless %s\nRemainAfterExit=no\nExitType=main\nRestart=no\nRestartMode=normal\nRuntimeMaxSec=600s\n", binary, plan)
	if err := os.WriteFile(unit, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	installedCommandTool(t, "systemctl", "daemon-reload")
	installedCommandTool(t, "systemctl", "start", "ardents-endpoint.service")
	invocation := strings.TrimSpace(string(installedCommandTool(t, "systemctl", "show", "ardents-endpoint.service", "-p", "InvocationID", "--value")))
	if len(invocation) != 32 {
		t.Fatal("missing exact Endpoint invocation")
	}
	return invocation
}

func waitInstalledCommandRequest(t *testing.T, invocation, path string) []byte {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		journal := installedCommandTool(t, "journalctl", "--no-pager", "-o", "cat", "_SYSTEMD_INVOCATION_ID="+invocation)
		for _, line := range bytes.Split(journal, []byte{'\n'}) {
			var event struct {
				Kind   string
				Digest string `json:"request_digest"`
			}
			if json.Unmarshal(line, &event) != nil || event.Kind != "headless-runtime-permission-required" {
				continue
			}
			raw, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(raw)
			if hex.EncodeToString(digest[:]) == event.Digest {
				return raw
			}
		}
		active := strings.TrimSpace(string(installedCommandTool(t, "systemctl", "show", "ardents-endpoint.service", "-p", "ActiveState", "--value")))
		if active != "active" {
			t.Fatalf("Endpoint stopped before permission: %s", journal)
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("Endpoint permission request deadline exceeded")
	return nil
}

func waitInstalledCommandSockets(t *testing.T, invocation string, paths ...string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		ready := true
		for _, path := range paths {
			info, err := os.Lstat(path)
			ready = ready && err == nil && info.Mode()&os.ModeSocket != 0
		}
		if ready {
			return
		}
		active := strings.TrimSpace(string(installedCommandTool(t, "systemctl", "show", "ardents-endpoint.service", "-p", "ActiveState", "--value")))
		if active != "active" {
			t.Fatalf("Endpoint stopped before sockets: %s", installedCommandTool(t, "journalctl", "--no-pager", "-o", "cat", "_SYSTEMD_INVOCATION_ID="+invocation))
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("Endpoint command sockets deadline exceeded")
}
