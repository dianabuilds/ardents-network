//go:build linux && text_worker_installed

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/allocation"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// The root driver runs outside the Endpoint unit. Its actual pinned program is
// the non-root Endpoint MainPID; only genuine Installation commands create the
// account, fixed resources, Root completion exchange and selected generation.
// All signing, Source and clock fixtures have one controller. This proves no
// independent custody, trusted time, private Service or whole-host qualification.
func TestInstalledExecutionStartup(t *testing.T) {
	base, artifacts := os.Getenv("ARDENTS_EXECUTION_START_ROOT"), os.Getenv("ARDENTS_EXECUTION_START_RESOURCES")
	if os.Getuid() != 0 || !filepath.IsAbs(base) || !filepath.IsAbs(artifacts) {
		t.Fatal("invalid environment: root driver, fresh trusted root and pinned resources required")
	}
	resources := installedStartupResources(t, artifacts)
	program := filepath.Join(artifacts, "ardents-linux-amd64")
	commandBuild.once.Do(func() { commandBuild.path = program })
	if compiledCommand(t) != program {
		t.Fatal("invalid environment: native fixture did not retain the pinned program")
	}
	f := newNetworkAdmissionFixture(t)
	epoch, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	clock := filepath.Join(base, "clock")
	installedStartupWrite(t, clock, nil, 0644)
	stopClock, clockJoined := make(chan struct{}), make(chan error, 1)
	go func() {
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stopClock:
				clockJoined <- nil
				return
			case now := <-tick.C:
				if err := os.Chtimes(clock, now, now); err != nil {
					clockJoined <- err
					return
				}
			}
		}
	}()
	t.Cleanup(func() {
		close(stopClock)
		if err := <-clockJoined; err != nil {
			t.Error(err)
		}
	})
	var key [32]byte
	copy(key[:], f.spec.Authority.Public().(ed25519.PublicKey))
	plan := &networkAuthorityPlan{Root: filepath.Join(base, "state"), NetworkID: f.spec.NetworkID,
		Authorities: [][32]byte{key}, Threshold: 1, ProfileAuthority: key, ClockObservationFile: clock}
	config, err := networkStateConfig(plan)
	if err != nil {
		t.Fatal(err)
	}
	credentials := filepath.Join(base, "credentials")
	if err := os.Mkdir(credentials, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(credentials, 0755); err != nil {
		t.Fatal(err)
	}
	_, clientPEM, clientKey, clientPin := installedSourceCertificate(t, "client", false)
	clientPath, keyPath := filepath.Join(credentials, "client.pem"), filepath.Join(credentials, "client.key")
	installedStartupWrite(t, clientPath, clientPEM, 0600)
	installedStartupWrite(t, keyPath, clientKey, 0600)
	clear(clientKey)
	var listeners [2]net.Listener
	var addresses [2]string
	var sourcePins [2][32]byte
	for i := range listeners {
		listeners[i], err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = listeners[i].Close() })
		addresses[i] = listeners[i].Addr().String()
	}
	for _, listener := range listeners {
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for i, address := range addresses {
		name := []string{"source-a.example", "source-b.example"}[i]
		certificate, rootPEM, keyPEM, pin := installedSourceCertificate(t, name, true)
		clear(keyPEM)
		sourcePins[i] = pin
		installedStartupWrite(t, filepath.Join(credentials, []string{"a.pem", "b.pem"}[i]), rootPEM, 0644)
		cfg := config
		cfg.Root = filepath.Join(t.TempDir(), "source-state")
		cfg.LocalRoleStateRoot = filepath.Join(t.TempDir(), "source-roles")
		seed, err := state.Open(cfg)
		if err != nil {
			t.Fatal(err)
		}
		_, acceptErr := seed.Accept(t.Context(), epoch.Epoch.Raw, epoch.Epoch.Inputs, epoch.Epoch.Materials)
		closeErr := seed.Close()
		if acceptErr != nil || closeErr != nil {
			t.Fatal(acceptErr, closeErr)
		}
		cfg.Source.ServeAddress, cfg.Source.ServeCertificate = address, certificate
		cfg.Source.ServeClientRootPEM, cfg.Source.ServeClientKeyDigests = clientPEM, [][32]byte{clientPin}
		server, err := state.Open(cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := server.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	rootAuthority := newConsumerReleaseAuthority(t, time.Date(2031, 1, 2, 3, 4, 5, 0, time.UTC))
	bundle, pin, initialProgram := signedConsumerRelease(t, "linux-amd64", true, true, 1, resources, rootAuthority, 1, 1)
	request := string(installationRequestBytes(t, bundle, pin, filepath.Join(base, "floors")))
	quoted := func(s string) string {
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	for old, newPath := range map[string]string{
		"/installation": filepath.Join(base, "installation"), "/state": plan.Root,
		"/entry": filepath.Join(base, "entry"), "/roles": filepath.Join(base, "roles"),
		"/tokens": filepath.Join(base, "tokens"), "/clock": clock,
		"/socket/reader":          filepath.Join(base, "application", "reader"),
		"/permissions/request":    filepath.Join(base, "permission", "request"),
		"/permissions/response":   filepath.Join(base, "permission", "response"),
		"/credentials/client.pem": clientPath, "/credentials/client.key": keyPath,
		"/credentials/a.pem": filepath.Join(credentials, "a.pem"), "/credentials/b.pem": filepath.Join(credentials, "b.pem"),
	} {
		request = strings.ReplaceAll(request, quoted(old), quoted(newPath))
	}
	request = strings.ReplaceAll(request, strings.Repeat("02", 32), hex.EncodeToString(plan.NetworkID[:]))
	request = strings.ReplaceAll(request, strings.Repeat("ab", 32), hex.EncodeToString(key[:]))
	request = strings.ReplaceAll(request, strings.Repeat("05", 32), hex.EncodeToString(f.spec.Seed[:]))
	for i, address := range addresses {
		request = strings.ReplaceAll(request, quoted([]string{"source-a.example:443", "source-b.example:443"}[i]), quoted(address))
		identity := sha256.Sum256([]byte([]string{"source-a.example", "source-b.example"}[i]))
		request = strings.ReplaceAll(request, strings.Repeat([]string{"06", "07"}[i], 32), hex.EncodeToString(identity[:]))
		request = strings.Replace(request, strings.Repeat("08", 32), hex.EncodeToString(sourcePins[i][:]), 1)
	}
	request = strings.Replace(request, "\"clock_observed_at\":\"2030-01-02T03:04:05Z\"", "\"clock_observed_at\":"+quoted(time.Now().UTC().Format(time.RFC3339)), 1)
	requestPath := filepath.Join(base, "initial-request.json")
	installedStartupWrite(t, requestPath, []byte(request), 0600)
	installedStartupCommand(t, initialProgram, nil, "installed-stopped", "installation", "provision", "--request", requestPath)
	account, err := user.Lookup("ardents-endpoint")
	if err != nil {
		t.Fatal(err)
	}
	uid, err := strconv.ParseUint(account.Uid, 10, 32)
	if err != nil || uid == 0 {
		t.Fatal("invalid installed UID", err)
	}
	gid, err := strconv.ParseUint(account.Gid, 10, 32)
	if err != nil || gid == 0 {
		t.Fatal("invalid installed GID", err)
	}
	for _, path := range []string{clientPath, keyPath} {
		if err := os.Chown(path, int(uid), int(gid)); err != nil {
			t.Fatal(err)
		}
	}
	selection, err := os.ReadFile(filepath.Join(base, "installation", "selection.json"))
	if err != nil {
		t.Fatal(err)
	}
	var selected struct {
		GenerationDigest string "json:\"generation_digest\""
	}
	if err := json.Unmarshal(selection, &selected); err != nil || len(selected.GenerationDigest) != 64 {
		t.Fatalf("selection: %v: %s", err, selection)
	}
	installedProgram := filepath.Join(base, "installation", "generations", selected.GenerationDigest, "ardents-linux-amd64")
	credential := &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), Groups: []uint32{}}
	bootstrap := filepath.Join(base, "bootstrap")
	if err := os.Mkdir(bootstrap, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(bootstrap, 0755); err != nil {
		t.Fatal(err)
	}
	accept := func(operation string, input any, outcome string) {
		raw, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(bootstrap, operation+".json")
		installedStartupWrite(t, path, raw, 0644)
		installedStartupCommand(t, installedProgram, credential, outcome, "network", operation, "--config", path)
	}
	accept("accept-epoch", struct {
		Network   *networkAuthorityPlan "json:\"network\""
		Epoch     []byte                "json:\"epoch\""
		Inputs    [][]byte              "json:\"inputs\""
		Materials [][]byte              "json:\"materials\""
	}{plan, epoch.Epoch.Raw, epoch.Epoch.Inputs, epoch.Epoch.Materials}, "accepted-epoch")
	accept("accept-profile", struct {
		Network *networkAuthorityPlan "json:\"network\""
		Profile []byte                "json:\"profile\""
	}{plan, epoch.Profile}, "accepted-profile")
	candidateResources := make(map[string][]byte, len(resources))
	for name, body := range resources {
		candidateResources[name] = append([]byte(nil), body...)
	}
	candidateResources["ardents-endpoint.service"] = append([]byte("# signed successor fixture\n"), candidateResources["ardents-endpoint.service"]...)
	candidate, _, _ := signedConsumerRelease(t, "linux-amd64", true, true, 2, candidateResources, rootAuthority, 2, 2)
	candidateRequest := strings.Replace(request, quoted(bundle), quoted(candidate), 1)
	candidateRequest = strings.Replace(candidateRequest, ",\"manifest_sha256\":"+quoted(pin), "", 1)
	candidatePath := filepath.Join(base, "candidate-request.json")
	installedStartupWrite(t, candidatePath, []byte(candidateRequest), 0600)
	installedStartupCommand(t, initialProgram, nil, "installed-started", "installation", "upgrade-installed", "--request", candidatePath)
	original := installedStartupIdentity(t)
	if original[0] == "0" || len(original[1]) != 32 || original[2] != "active" {
		t.Fatal("actual installed participant is not active", original)
	}
	proc, err := os.Open(filepath.Join("/proc", original[0]))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := proc.Close(); err != nil {
			t.Error(err)
		}
	})
	scope, err := os.Open("/sys/fs/cgroup/system.slice/ardents-endpoint.service/cgroup.events")
	if err != nil {
		t.Fatal(err)
	}
	var scopeFS syscall.Statfs_t
	if err := syscall.Fstatfs(int(scope.Fd()), &scopeFS); err != nil || scopeFS.Type != 0x63677270 {
		t.Fatal("original cgroup events is not a kernel cgroup-v2 object", err)
	}
	t.Cleanup(func() {
		if err := scope.Close(); err != nil {
			t.Error(err)
		}
	})
	stopParticipant := func() {
		if current := installedStartupIdentity(t); current != original {
			t.Errorf("original installed invocation changed: %v", current)
			return
		}
		cmd := exec.Command("systemctl", "stop", "ardents-endpoint.service")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("original stop: %v: %s", err, out)
		}
		if _, err := os.ReadFile(filepath.Join("/proc/self/fd", strconv.FormatUint(uint64(proc.Fd()), 10), "stat")); !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ESRCH) {
			t.Errorf("original process did not join: %v", err)
		}
		installedStartupWait(t, func() ([]byte, bool) {
			if _, err := scope.Seek(0, 0); errors.Is(err, syscall.ENODEV) {
				return nil, true
			} else if err != nil {
				t.Fatal(err)
			}
			var body [1025]byte
			n, err := scope.Read(body[:])
			if errors.Is(err, syscall.ENODEV) && n == 0 {
				return nil, true
			}
			if err != nil {
				t.Fatal("original kernel scope read", err)
			}
			return body[:n], false
		})
	}
	t.Cleanup(stopParticipant)
	// File birth precedes checked write/sync. Only the actual participant's
	// post-durability diagnostic establishes that its public request is complete.
	installedStartupWait(t, func() ([]byte, bool) {
		raw, err := exec.CommandContext(t.Context(), "journalctl", "--no-pager", "-o", "cat", "_SYSTEMD_INVOCATION_ID="+original[1]).CombinedOutput()
		if err != nil {
			t.Fatalf("original pending journal: %v: %s", err, raw)
		}
		return raw, bytes.Contains(raw, []byte("\"phase\":\"permission-pending\""))
	})
	requestBytes := installedStartupWait(t, func() ([]byte, bool) {
		raw, err := os.ReadFile(filepath.Join(base, "permission", "request"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		return raw, err == nil
	})
	prepared, err := allocation.Prepare(requestBytes, f.profile.NetworkID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	decision, err := prepared.Decide(nil, f.profile.IssuanceAuthorityKey, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(base, "allocation")
	installedStartupWrite(t, journal, decision.Journal(), 0600)
	readback, err := os.ReadFile(journal)
	if err != nil || !bytes.Equal(readback, decision.Journal()) {
		t.Fatal("allocation durability readback", err)
	}
	permission := decision.Permission()
	copy(permission.Signature[:], ed25519.Sign(f.spec.Authority, admission.PermissionTranscript(permission)))
	response, err := admission.EncodePermission(permission)
	if err != nil {
		t.Fatal(err)
	}
	temporary := filepath.Join(base, "permission", "response.pending")
	installedStartupWrite(t, temporary, response, 0600)
	if err := os.Chown(temporary, int(uid), int(gid)); err != nil {
		t.Fatal(err)
	}
	installedStartupSync(t, temporary)
	if _, err := os.Lstat(filepath.Join(base, "permission", "response")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("response already exists", err)
	}
	if err := os.Rename(temporary, filepath.Join(base, "permission", "response")); err != nil {
		t.Fatal(err)
	}
	installedStartupSync(t, filepath.Join(base, "permission"))
	installedStartupWait(t, func() ([]byte, bool) {
		cmd := exec.CommandContext(t.Context(), "journalctl", "--no-pager", "-o", "cat", "_SYSTEMD_INVOCATION_ID="+original[1])
		raw, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("original journal: %v: %s", err, raw)
		}
		return raw, bytes.Contains(raw, []byte("\"phase\":\"permission-accepted\""))
	})
	if current := installedStartupIdentity(t); current != original {
		t.Fatal("accepted permission lost original invocation", current)
	}
	t.Log("actual non-root installed program completed Root exchange and qualified permission bootstrap")
}

func installedStartupResources(t *testing.T, root string) map[string][]byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "inventory.json"))
	if err != nil {
		t.Fatal(err)
	}
	var inventory struct {
		Files []struct {
			Name   string
			SHA256 string
		}
	}
	if err := json.Unmarshal(raw, &inventory); err != nil || len(inventory.Files) != 9 {
		t.Fatal("invalid pinned inventory", err)
	}
	result := make(map[string][]byte)
	for _, file := range inventory.Files {
		if filepath.Base(file.Name) != file.Name || result[file.Name] != nil {
			t.Fatal("invalid inventory name")
		}
		body, err := os.ReadFile(filepath.Join(root, file.Name))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(body)
		if hex.EncodeToString(digest[:]) != file.SHA256 {
			t.Fatal("resource differs from independent pin", file.Name)
		}
		result[file.Name] = body
	}
	return result
}

func installedStartupWrite(t *testing.T, path string, raw []byte, mode os.FileMode) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		t.Fatal(err)
	}
	n, writeErr := file.Write(raw)
	modeErr := file.Chmod(mode)
	syncErr := file.Sync()
	closeErr := file.Close()
	if n != len(raw) || writeErr != nil || modeErr != nil || syncErr != nil || closeErr != nil {
		t.Fatal("durable fixture write", writeErr, modeErr, syncErr, closeErr)
	}
	installedStartupSync(t, filepath.Dir(path))
}

func installedStartupSync(t *testing.T, path string) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	syncErr, closeErr := file.Sync(), file.Close()
	if syncErr != nil || closeErr != nil {
		t.Fatal(syncErr, closeErr)
	}
}

func installedStartupCommand(t *testing.T, program string, credential *syscall.Credential, outcome string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), program, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential, Pdeathsig: syscall.SIGKILL}
	runtime.LockOSThread()
	out, err := cmd.CombinedOutput()
	runtime.UnlockOSThread()
	var response struct{ Outcome string }
	if err != nil || json.Unmarshal(out, &response) != nil || response.Outcome != outcome {
		t.Fatalf("actual %v: %v: %s", args, err, out)
	}
}

func installedStartupIdentity(t *testing.T) [3]string {
	t.Helper()
	var result [3]string
	for i, property := range []string{"MainPID", "InvocationID", "ActiveState"} {
		out, err := exec.Command("systemctl", "show", "ardents-endpoint.service", "-p", property, "--value").CombinedOutput()
		if err != nil {
			t.Fatalf("actual %s: %v: %s", property, err, out)
		}
		result[i] = strings.TrimSpace(string(out))
	}
	return result
}

func installedStartupWait(t *testing.T, observe func() ([]byte, bool)) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		raw, done := observe()
		if done {
			return raw
		}
		select {
		case <-ctx.Done():
			t.Fatal("actual installed observation exceeded bound", ctx.Err())
		case <-tick.C:
		}
	}
}
