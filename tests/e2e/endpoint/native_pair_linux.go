//go:build linux && endpoint_pair_native

package endpoint

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

const pairBaseRootfsEnv = "ARDENTS_PAIR_BASE_ROOTFS"

// PairGuest denotes a booted manager under the native test cleanup.
type PairGuest struct {
	name   string
	rootfs string
	leader string
}

// RepositoryRoot locates the module from an Endpoint qualification driver.
func RepositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Ardents module root not found")
		}
		dir = parent
	}
}

// GoEnv returns an explicit Go runtime or cache path used by the guests.
func GoEnv(t *testing.T, name string) string {
	t.Helper()
	out, err := exec.Command("go", "env", name).Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		t.Fatalf("go env %s: %v: %s", name, err, out)
	}
	return strings.TrimSpace(string(out))
}

// EnsurePairBaseRootfs caches one pristine debootstrapped Ubuntu 24.04 rootfs and
// clones a fresh guest from it per run, so each manager starts with absent
// accounts/units/state. The base is environment provisioning, not an Ardents
// artifact; it stays outside Git.
func EnsurePairBaseRootfs(t *testing.T) string {
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
			"noble", base, "https://archive.ubuntu.com/ubuntu")
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
printf 'deb https://archive.ubuntu.com/ubuntu noble main restricted universe multiverse\ndeb https://archive.ubuntu.com/ubuntu noble-updates main restricted universe multiverse\ndeb https://archive.ubuntu.com/ubuntu noble-security main restricted universe multiverse\n' > /etc/apt/sources.list
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

// BootPairGuest starts a fresh manager and registers its joined cleanup.
func BootPairGuest(t *testing.T, base, runID, label, goRoot, repoRoot, modCache, buildCache string) *PairGuest {
	t.Helper()
	guest := &PairGuest{
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

func pairTerminateGuest(t *testing.T, guest *PairGuest) {
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

// VerifyDistinctManagers asserts each guest is a separate admitted manager:
// distinct PID/mount/cgroup namespaces from the host and from each other, shared
// host network namespace, Ubuntu 24.04 + systemd 255 + cgroup v2 + original
// manager at /init.scope. These are the same manager-local facts the product
// platform admission enforces.
// VerifyDistinctManagers checks the guests use independent system managers.
func VerifyDistinctManagers(t *testing.T, guests []*PairGuest) {
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

// Name returns the machine name used for systemd-run.
func (guest *PairGuest) Name() string { return guest.name }

// Rootfs returns the original guest filesystem root.
func (guest *PairGuest) Rootfs() string { return guest.rootfs }
