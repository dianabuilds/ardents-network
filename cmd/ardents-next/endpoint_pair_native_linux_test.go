//go:build linux && endpoint_pair_native

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This native qualification proves the platform-led launch foundation on TWO
// separately admitted Ubuntu/systemd managers (#391 S2.1). One Endpoint per
// system manager is a fixed, multiply-enforced contract (dedicated account, unit
// names, fixed resource paths, preflight absence and DynamicUser name-hash UIDs),
// so the installed two-Endpoint oracle requires two independent managers. The
// controller provisions two fresh systemd-nspawn guests, each its own PID-1
// systemd with separate PID/mount/cgroup namespaces and a pivoted Ubuntu 24.04
// root, sharing the host network namespace (fixture loopback). Each guest passes
// the same manager-local platform admission the product enforces (Ubuntu
// 24.04/systemd 255, cgroup v2, root-owned os-release, reachable system bus) and
// independently runs the checked endpoint_launch_native full-launch qualification
// to protected text Service readiness.
//
// It is component/acceptance evidence with fixture Release/Network authority for
// two independently admitted managers reaching platform-led Service readiness.
// It is NOT the cross-Endpoint publish/link/read journey (#391 S2.2) and NOT the
// whole-host two-participant privacy qualification. The transitional
// guarded-start/ACK/recovery orchestration is neither exercised nor retired
// (ADR-0125 D6 gates retirement on proven replacement guarantees).

const pairBaseRootfsEnv = "ARDENTS_PAIR_BASE_ROOTFS"

func TestNativeEndpointPlatformLedPair(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("endpoint platform-led pair qualification requires a root driver")
	}
	for _, tool := range []string{"systemd-nspawn", "machinectl", "debootstrap", "cp", "systemd-run"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("invalid environment: %s unavailable: %v", tool, err)
		}
	}
	if out, err := exec.Command("stat", "-fc", "%T", "/sys/fs/cgroup").Output(); err != nil || strings.TrimSpace(string(out)) != "cgroup2fs" {
		t.Fatalf("invalid environment: host cgroup v2 required: %v: %s", err, out)
	}
	repoRoot := pairRepoRoot(t)
	goRoot := pairGoEnv(t, "GOROOT")
	modCache := pairGoEnv(t, "GOMODCACHE")
	buildCache := pairGoEnv(t, "GOCACHE")
	base := pairEnsureBaseRootfs(t)

	runID := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
	var guests [2]*pairGuest
	for i, label := range []string{"a", "b"} {
		guests[i] = pairBootGuest(t, base, runID, label, goRoot, repoRoot, modCache, buildCache)
	}
	pairVerifyDistinctManagers(t, guests[:])

	// Run the checked full-launch qualification independently in each guest. The
	// runs are serial so the two guests' ephemeral fixture Source ports on the
	// shared loopback cannot race; both managers remain admitted throughout.
	var invocations [2]string
	for i, guest := range guests {
		invocations[i] = pairRunLaunchQualification(t, guest, repoRoot)
	}
	if invocations[0] == "" || invocations[1] == "" || invocations[0] == invocations[1] {
		t.Fatal("two managers did not produce distinct original invocations", invocations)
	}
	t.Logf("two independently admitted managers reached platform-led Service readiness; InvocationIDs %s / %s", invocations[0], invocations[1])
}

type pairGuest struct {
	name   string
	rootfs string
	leader string
}

func pairRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repository root not found from %s: %v", wd, err)
	}
	return root
}

func pairGoEnv(t *testing.T, name string) string {
	t.Helper()
	out, err := exec.Command("go", "env", name).Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		t.Fatalf("go env %s: %v: %s", name, err, out)
	}
	return strings.TrimSpace(string(out))
}

// pairEnsureBaseRootfs caches one pristine debootstrapped Ubuntu 24.04 rootfs and
// clones a fresh guest from it per run, so each manager starts with absent
// accounts/units/state. The base is environment provisioning, not an Ardents
// artifact; it stays outside Git.
func pairEnsureBaseRootfs(t *testing.T) string {
	t.Helper()
	base := os.Getenv(pairBaseRootfsEnv)
	if base == "" {
		base = "/var/lib/machines/ardents-pair-base"
	}
	if _, err := os.Stat(filepath.Join(base, "usr", "lib", "os-release")); err != nil {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "debootstrap", "--variant=minbase",
			"--include=systemd,systemd-sysv,dbus,passwd,ca-certificates,iproute2,kmod",
			"noble", base, "http://archive.ubuntu.com/ubuntu")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("invalid environment: base rootfs debootstrap failed: %v: %s", err, out)
		}
	}
	pairPatchBaseSystemd(t, base)
	return base
}

// pairPatchBaseSystemd upgrades the cached base rootfs systemd to the current
// patched noble release. debootstrap installs the release snapshot (.8), which
// predates the reviewed stand security fixes; the qualification managers must run
// systemd at or above the CVE-2026-40226 .16 fix (see dependencies.md). The step
// is idempotent: a base already at or above .16 is left unchanged.
func pairPatchBaseSystemd(t *testing.T, base string) {
	t.Helper()
	if pairSystemdPatchLevel(pairChrootSystemdVersion(t, base)) >= 16 {
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Minute)
	defer cancel()
	// debootstrap configures only the release pocket (systemd .8) and a
	// systemd-resolved stub resolv.conf that does not resolve in an ephemeral
	// container. Enable the updates/security pockets and a static resolver so the
	// patched noble systemd (>= .16) is actually reachable.
	script := `set -e
printf 'deb http://archive.ubuntu.com/ubuntu noble main restricted universe multiverse\ndeb http://archive.ubuntu.com/ubuntu noble-updates main restricted universe multiverse\ndeb http://archive.ubuntu.com/ubuntu noble-security main restricted universe multiverse\n' > /etc/apt/sources.list
rm -f /etc/resolv.conf
printf 'nameserver 1.1.1.1\nnameserver 8.8.8.8\n' > /etc/resolv.conf
apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y --only-upgrade systemd systemd-sysv libnss-systemd dbus`
	cmd := exec.CommandContext(ctx, "systemd-nspawn", "-D", base, "--machine=ardpairbase",
		"-q", "--", "/bin/bash", "-c", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("invalid environment: base systemd patch upgrade failed: %v: %s", err, out)
	}
	if level := pairSystemdPatchLevel(pairChrootSystemdVersion(t, base)); level < 16 {
		t.Fatalf("invalid environment: base systemd still below .16 after upgrade (level %d)", level)
	}
}

func pairChrootSystemdVersion(t *testing.T, base string) string {
	t.Helper()
	out, err := exec.Command("chroot", base, "dpkg-query", "-W", "-f=${Version}", "systemd").Output()
	if err != nil {
		t.Fatalf("invalid environment: base systemd version query failed: %v: %s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// pairSystemdPatchLevel extracts the Ubuntu noble patch suffix from a systemd
// package version such as 255.4-1ubuntu8.18 (returns 18). An unrecognised or
// unpatched release version returns 0.
func pairSystemdPatchLevel(version string) int {
	const marker = "1ubuntu8."
	index := strings.Index(version, marker)
	if index < 0 {
		return 0
	}
	level := 0
	for _, c := range version[index+len(marker):] {
		if c < '0' || c > '9' {
			break
		}
		level = level*10 + int(c-'0')
	}
	return level
}

func pairBootGuest(t *testing.T, base, runID, label, goRoot, repoRoot, modCache, buildCache string) *pairGuest {
	t.Helper()
	guest := &pairGuest{
		name:   "ardpair" + label + fmt.Sprintf("%d", time.Now().UnixNano()%100000000),
		rootfs: filepath.Join("/var/lib/machines", "ardents-pair-"+runID+"-"+label),
	}
	if out, err := exec.Command("cp", "-a", base, guest.rootfs).CombinedOutput(); err != nil {
		t.Fatalf("clone guest rootfs: %v: %s", err, out)
	}
	t.Cleanup(func() { pairTerminateGuest(t, guest) })
	args := []string{"-D", guest.rootfs, "--machine=" + guest.name, "--boot",
		"--bind=" + goRoot, "--bind=" + repoRoot, "--bind=" + modCache, "--bind=" + buildCache}
	command := exec.Command("systemd-nspawn", args...)
	command.Stdout, command.Stderr = nil, nil
	if err := command.Start(); err != nil {
		t.Fatalf("boot guest %s: %v", guest.name, err)
	}
	go func() { _ = command.Wait() }()
	// Wait for the guest's own PID-1 systemd to own the system bus.
	pairWait(t.Context(), t, 60*time.Second, func() bool {
		out, err := exec.Command("machinectl", "show", guest.name, "-p", "Leader", "--value").Output()
		if err != nil || strings.TrimSpace(string(out)) == "" || strings.TrimSpace(string(out)) == "0" {
			return false
		}
		guest.leader = strings.TrimSpace(string(out))
		return pairGuestBusReady(guest.name)
	})
	if guest.leader == "" {
		t.Fatalf("guest %s never exposed an original manager", guest.name)
	}
	return guest
}

func pairGuestBusReady(name string) bool {
	out, err := exec.Command("systemd-run", "-M", name, "--pipe", "--quiet",
		"/usr/bin/busctl", "--system", "--no-pager", "call", "org.freedesktop.systemd1",
		"/org/freedesktop/systemd1", "org.freedesktop.DBus.Properties", "Get", "ss",
		"org.freedesktop.systemd1.Manager", "Version").Output()
	return err == nil && bytes.Contains(out, []byte(`"255.`))
}

func pairTerminateGuest(t *testing.T, guest *pairGuest) {
	t.Helper()
	_ = exec.Command("machinectl", "terminate", guest.name).Run()
	// Cleanup runs after the test context is cancelled; wait on an independent
	// background bound so joined termination is still observed.
	pairWait(context.Background(), t, 30*time.Second, func() bool {
		out, err := exec.Command("machinectl", "show", guest.name, "-p", "Leader", "--value").Output()
		return err != nil || strings.TrimSpace(string(out)) == "" || strings.TrimSpace(string(out)) == "0"
	})
	_ = exec.Command("systemctl", "reset-failed", "machine-"+guest.name+".scope").Run()
	_ = os.RemoveAll(guest.rootfs)
}

// pairVerifyDistinctManagers asserts each guest is a separate admitted manager:
// distinct PID/mount/cgroup namespaces from the host and from each other, shared
// host network namespace, Ubuntu 24.04 + systemd 255 + cgroup v2 + original
// manager at /init.scope. These are the same manager-local facts the product
// platform admission enforces.
func pairVerifyDistinctManagers(t *testing.T, guests []*pairGuest) {
	t.Helper()
	hostNS := map[string]string{}
	for _, ns := range []string{"pid", "mnt", "cgroup", "net"} {
		hostNS[ns] = pairReadlink(t, filepath.Join("/proc/self/ns", ns))
	}
	seen := map[string]bool{}
	for _, guest := range guests {
		for _, ns := range []string{"pid", "mnt", "cgroup"} {
			identity := pairReadlink(t, filepath.Join("/proc", guest.leader, "ns", ns))
			if identity == hostNS[ns] || seen[ns+identity] {
				t.Fatalf("guest %s lacks a separate original %s namespace", guest.name, ns)
			}
			seen[ns+identity] = true
		}
		if net := pairReadlink(t, filepath.Join("/proc", guest.leader, "ns", "net")); net != hostNS["net"] {
			t.Fatalf("guest %s does not share the fixture loopback network namespace", guest.name)
		}
		// dpkg-query default format ("systemd\t<version>"); a -f=${Version}
		// format would be expanded as an empty environment reference by the
		// systemd-run ExecStart parser.
		out, err := exec.Command("systemd-run", "-M", guest.name, "--pipe", "--quiet", "/bin/bash", "-c",
			`. /usr/lib/os-release; echo "$ID:$VERSION_ID"; stat -fc %T /sys/fs/cgroup; cat /proc/1/cgroup; uname -m; dpkg-query -W systemd`).Output()
		if err != nil {
			t.Fatalf("guest %s platform observation: %v: %s", guest.name, err, out)
		}
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) != 5 || lines[0] != "ubuntu:24.04" || lines[1] != "cgroup2fs" || lines[2] != "0::/init.scope" || lines[3] != "x86_64" {
			t.Fatalf("guest %s is not an admitted original manager: %q", guest.name, out)
		}
		versionFields := strings.Fields(lines[4])
		if len(versionFields) != 2 || versionFields[0] != "systemd" {
			t.Fatalf("guest %s systemd package observation: %q", guest.name, lines[4])
		}
		if level := pairSystemdPatchLevel(versionFields[1]); level < 16 {
			t.Fatalf("guest %s systemd %s is below the patched .16 qualification floor", guest.name, versionFields[1])
		}
		if !pairGuestBusReady(guest.name) {
			t.Fatalf("guest %s system bus lacks admitted systemd 255", guest.name)
		}
	}
}

func pairReadlink(t *testing.T, path string) string {
	t.Helper()
	target, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("readlink %s: %v", path, err)
	}
	return target
}

// pairRunLaunchQualification runs the checked endpoint_launch_native full-launch
// qualification inside the guest against its own original manager, using the
// canonical repository and toolchain bound at their host paths. It returns the
// observed original InvocationID and refuses any skip.
func pairRunLaunchQualification(t *testing.T, guest *pairGuest, repoRoot string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 6*time.Minute)
	defer cancel()
	script := fmt.Sprintf("cd %s && go test -tags=endpoint_launch_native -run '^TestNativeEndpointPlatformLedLaunch' -count=1 -v ./cmd/ardents-next", repoRoot)
	cmd := exec.CommandContext(ctx, "systemd-run", "-M", guest.name, "--pipe", "--quiet",
		"--setenv=PATH=/usr/local/go/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"--setenv=GOTOOLCHAIN=local",
		"--setenv=GOMODCACHE="+pairGoEnv(t, "GOMODCACHE"),
		"--setenv=GOCACHE="+pairGoEnv(t, "GOCACHE"),
		"--setenv=GOFLAGS=-buildvcs=false",
		"/bin/bash", "-c", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("guest %s launch qualification failed: %v\n%s", guest.name, err, out)
	}
	if bytes.Contains(out, []byte("--- SKIP:")) {
		t.Fatalf("guest %s launch qualification passed by skip:\n%s", guest.name, out)
	}
	if !bytes.Contains(out, []byte("--- PASS: TestNativeEndpointPlatformLedLaunch ")) {
		t.Fatalf("guest %s lacks executed positive launch evidence:\n%s", guest.name, out)
	}
	if !bytes.Contains(out, []byte("--- PASS: TestNativeEndpointPlatformLedLaunchRefusals ")) {
		t.Fatalf("guest %s lacks executed causal refusal evidence:\n%s", guest.name, out)
	}
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		if marker := []byte("InvocationID="); bytes.Contains(line, marker) {
			rest := line[bytes.Index(line, marker)+len(marker):]
			if fields := strings.Fields(string(rest)); len(fields) > 0 && len(fields[0]) == 32 {
				return fields[0]
			}
		}
	}
	t.Fatalf("guest %s launch qualification observed no original InvocationID:\n%s", guest.name, out)
	return ""
}

func pairWait(ctx context.Context, t *testing.T, bound time.Duration, ready func() bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, bound)
	defer cancel()
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		if ready() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("guest observation exceeded bound:", ctx.Err())
		case <-tick.C:
		}
	}
}
