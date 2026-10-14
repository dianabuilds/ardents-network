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
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// The root driver runs outside the Endpoint unit. Its actual pinned program is
// the non-root Endpoint MainPID; only genuine Installation commands create the
// account, fixed resources, Root completion exchange and selected generation.
// All signing, Source and clock fixtures have one controller. This proves no
// independent custody, trusted time, private Service or whole-host qualification.
func TestInstalledExecutionStartup(t *testing.T) {
	runInstalledServiceStartup(t, false, "")
}

func runInstalledServiceStartup(t *testing.T, publicationEnabled bool, carrier string) {
	_, artifacts := installedStartupPaths(t)
	resources := installedStartupResources(t, artifacts)
	var f *networkAdmissionFixture
	if publicationEnabled {
		f = installedServiceNetwork(t, carrier)
	} else {
		f = newNetworkAdmissionFixture(t)
	}
	network := installedSignedNetwork(t, f)
	runInstalledParticipant(t, publicationEnabled, network, resources, func(base, role string, request []byte) []byte {
		return installedServiceApproval(t, base, role, request, f)
	}, nil)
}

func installedSignedNetwork(t *testing.T, f *networkAdmissionFixture) installedStartupNetwork {
	t.Helper()
	epoch, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	var authority [32]byte
	copy(authority[:], f.spec.Authority.Public().(ed25519.PublicKey))
	return installedStartupNetwork{NetworkID: f.spec.NetworkID, Seed: f.spec.Seed, Authority: authority,
		Epoch: epoch.Epoch.Raw, Inputs: epoch.Epoch.Inputs, Materials: epoch.Epoch.Materials, Profile: epoch.Profile}
}

// This fixture input contains only public signed material and trust pins.
// Every participant still authenticates it through its own actual State;
// no live State, signing key, qualified operation or Instance crosses managers.
type installedStartupNetwork struct {
	NetworkID, Seed, Authority [32]byte
	Epoch, Profile             []byte
	Inputs, Materials          [][]byte
}

type installedStartupParticipant struct {
	base     string
	original [3]string
	stop     func()
}

func installedStartupPaths(t *testing.T) (string, string) {
	t.Helper()
	base, artifacts := os.Getenv("ARDENTS_EXECUTION_START_ROOT"), os.Getenv("ARDENTS_EXECUTION_START_RESOURCES")
	if os.Getuid() != 0 || !filepath.IsAbs(base) || !filepath.IsAbs(artifacts) {
		t.Fatal("invalid environment: root driver, fresh trusted root and pinned resources required")
	}
	program := filepath.Join(artifacts, "ardents-linux-amd64")
	commandBuild.once.Do(func() { commandBuild.path = program })
	if compiledCommand(t) != program {
		t.Fatal("invalid environment: native fixture did not retain the pinned program")
	}
	return base, artifacts
}

func runInstalledParticipant(t *testing.T, publicationEnabled bool, network installedStartupNetwork, resources map[string][]byte,
	approve func(base, role string, request []byte) []byte, exercise func(installedStartupParticipant)) {
	base, _ := installedStartupPaths(t)
	if approve == nil {
		t.Fatal("invalid environment: independent permission approval is required")
	}
	// Both callers verify the pinned inventory before starting any fixture
	// network activity, then retain those exact bytes for signed installation.
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
	key := network.Authority
	plan := &networkAuthorityPlan{Root: filepath.Join(base, "state"), NetworkID: network.NetworkID,
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
		_, acceptErr := seed.Accept(t.Context(), network.Epoch, network.Inputs, network.Materials)
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
	if publicationEnabled {
		request = installedServiceDeclaration(t, request, base)
	}
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
	request = strings.ReplaceAll(request, strings.Repeat("05", 32), hex.EncodeToString(network.Seed[:]))
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
	}{plan, network.Epoch, network.Inputs, network.Materials}, "accepted-epoch")
	accept("accept-profile", struct {
		Network *networkAuthorityPlan "json:\"network\""
		Profile []byte                "json:\"profile\""
	}{plan, network.Profile}, "accepted-profile")
	candidateResources := make(map[string][]byte, len(resources))
	for name, body := range resources {
		candidateResources[name] = append([]byte(nil), body...)
	}
	candidateResources["ardents-endpoint.service"] = append([]byte("# signed successor fixture\n"), candidateResources["ardents-endpoint.service"]...)
	// Exercise a signed static-resource successor as well as the Endpoint
	// template: its distinct manifest requires an actual fixed-file replacement.
	candidateResources["50-ardents-text.rules"] = append([]byte("// signed successor fixture\n"), candidateResources["50-ardents-text.rules"]...)
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
	stopParticipant := sync.OnceFunc(func() {
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
	})
	t.Cleanup(stopParticipant)
	// Hosting is an explicit provider input, never created/reset by runtime.
	// The real installed command initializes it under the installed account
	// after original Root completion and before Permission handoff; no token is granted.
	period := time.Now().UTC().Truncate(time.Second)
	budgetInput, err := json.Marshal(map[string]any{
		"root": filepath.Join(base, "entry", "hosting"),
		"policy": map[string]any{"provider": "installed-startup-fixture", "start": period.Add(-time.Minute),
			"end": period.Add(time.Hour), "unit": "GiB", "quantity": 16,
			"directions": "tx+rx", "interfaces": []string{"lo"},
			"initial_used_bytes": 0, "low_watermark_bytes": 1 << 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	budgetPath := filepath.Join(bootstrap, "hosting.json")
	installedStartupWrite(t, budgetPath, budgetInput, 0644)
	installedStartupCommand(t, installedProgram, credential, "completed", "hosting", "initialize", "--config", budgetPath)
	if publicationEnabled {
		installedServiceInstance(t, installedProgram, credential, base, network.NetworkID)
	}
	installedServicePermissions(t, base, original, uid, gid, approve, publicationEnabled)
	if current := installedStartupIdentity(t); current != original {
		t.Fatal("accepted permission lost original invocation", current)
	}
	installedStartupWait(t, func() ([]byte, bool) {
		conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: filepath.Join(base, "application", "reader"), Net: "unix"})
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) {
			return nil, false
		}
		if err != nil {
			t.Fatal("installed Reader listener", err)
		}
		defer conn.Close()
		raw, err := conn.SyscallConn()
		if err != nil {
			t.Fatal(err)
		}
		var peer *syscall.Ucred
		var peerErr error
		if err := raw.Control(func(fd uintptr) {
			peer, peerErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
		}); err != nil || peerErr != nil {
			t.Fatal("installed listener peer", err, peerErr)
		}
		if peer == nil || strconv.FormatInt(int64(peer.Pid), 10) != original[0] || peer.Uid != uint32(uid) || peer.Gid != uint32(gid) {
			t.Fatal("Reader listener is not owned by the original installed process")
		}
		return nil, true
	})
	if current := installedStartupIdentity(t); current != original {
		t.Fatal("Reader continuation lost original invocation", current)
	}
	t.Log("actual non-root installed program completed Root exchange, qualified permission bootstrap and original-process Reader listener; no Service request accepted")
	if exercise != nil {
		exercise(installedStartupParticipant{base: base, original: original, stop: stopParticipant})
	} else if publicationEnabled {
		installedServicePublication(t, base, original)
	}
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

func installedStartupCommand(t *testing.T, program string, credential *syscall.Credential, outcome string, args ...string) []byte {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), program, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential, Pdeathsig: syscall.SIGKILL}
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	runtime.LockOSThread()
	out, err := cmd.Output()
	runtime.UnlockOSThread()
	var response struct{ Outcome string }
	if err != nil || json.Unmarshal(out, &response) != nil || response.Outcome != outcome {
		t.Fatalf("actual %v: %v: %s; diagnostics: %s", args, err, out, diagnostic.String())
	}
	return out
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
