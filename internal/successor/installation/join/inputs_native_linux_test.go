//go:build linux && network_signing_native

package join

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

type inputFixture struct {
	path, prefix string
	request      Request
	plan         Plan
}

func TestJoinInputsNativeUnknownFutureJournalDeniesInitialEffects(t *testing.T) {
	for _, suffix := range []string{".join-first-start-journal", ".join-invocation-00000063-journal", ".join-retirement-00000000-journal", ".join-retirement-00000063-journal"} {
		t.Run(suffix, func(t *testing.T) {
			f := prepareInputFixture(t)
			if err := os.Mkdir(f.prefix, 0755); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Remove(f.prefix); err != nil {
					t.Error(err)
				}
			})
			unknown := f.request.InstallationRoot + suffix
			before := []byte("foreign retained prefix; never adopt\n")
			if err := os.WriteFile(unknown, before, 0600); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Remove(unknown); err != nil {
					t.Error(err)
				}
			})
			input, err := ReadInputs(t.Context(), f.path)
			if err != nil {
				t.Fatal(err)
			}
			// Exercise the creator's actual fresh-namespace check directly; no
			// manager map, Start, account creation or accepting preflight is faked.
			if err := input.observeFresh([]string{filepath.Dir(f.path)}); !errors.Is(err, ErrBinding) {
				t.Fatal("unknown future attempt did not deny initial namespace", err)
			}
			if err := input.Close(); !errors.Is(err, ErrBinding) {
				t.Fatal("original refusal lost at Close", err)
			}
			after, err := os.ReadFile(unknown)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("unknown history changed", err)
			}
			for _, path := range []string{f.request.InstallationRoot, f.request.StateRoot, f.request.ParticipationRoot, f.request.SpendRoot, f.request.HostingRoot, f.request.CredentialDirectory, f.request.InstallationRoot + ".join-initial-journal"} {
				if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("refused namespace check had a native effect", path, err)
				}
			}
		})
	}
}

func prepareInputFixture(t *testing.T) inputFixture {
	t.Helper()
	marker, err := os.ReadFile("/etc/ardents-join-preparation-native-stand")
	if err != nil || string(marker) != "ardents-join-preparation-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("invalid environment: separate root Join preparation stand required", err)
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	var fs syscall.Statfs_t
	if err := syscall.Statfs(parent, &fs); err != nil || fs.Type != 0xef53 {
		t.Fatal("invalid environment: native ext4 required", err)
	}
	prefix := fmt.Sprintf("/var/lib/join-input-%d", os.Getpid())
	if _, err := os.Lstat(prefix); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("invalid environment: future runtime namespace must be absent", err)
	}
	clockParent := filepath.Join(parent, "clock")
	if err := os.Mkdir(clockParent, 0755); err != nil {
		t.Fatal(err)
	}
	r := Request{Schema: "ardents-join-installation-v1", Invocation: "join", Profile: "ardents-join-process-v1", InstallationRoot: prefix + "/installation", StateRoot: prefix + "/state", ParticipationRoot: prefix + "/participation", SpendRoot: prefix + "/spend", HostingRoot: prefix + "/hosting", CredentialDirectory: prefix + "/credentials", Plan: filepath.Join(parent, "plan.json"), Certificate: filepath.Join(parent, "certificate.pem"), PrivateKey: filepath.Join(parent, "key.pem"), Clock: filepath.Join(clockParent, "observation")}
	p := Plan{Schema: "ardents-join-runtime-v1", Network: NetworkInputs{Root: r.StateRoot, NetworkID: [32]byte{1}, Authorities: [][32]byte{{2}}, Threshold: 1, ProfileAuthority: [32]byte{3}, ClockObservationFile: r.Clock}, NodeID: [32]byte{4}, ProcessProfile: r.Profile, ParticipationRoot: r.ParticipationRoot, SpendRoot: r.SpendRoot, HostingRoot: r.HostingRoot, Certificate: filepath.Join(r.CredentialDirectory, "certificate.pem"), PrivateKey: filepath.Join(r.CredentialDirectory, "private-key.pem"), RetirementWaitMS: 1000}
	p.Work.Tx, p.Work.Rx, p.Termination.Tx, p.Termination.Rx = 40000000, 40000000, 1024, 1024
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	now := time.Now()
	certificate := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, public, key)
	if err != nil {
		t.Fatal(err)
	}
	private, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(private)
	requestPath := filepath.Join(parent, "request.json")
	requestRaw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	planRaw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	for path, raw := range map[string][]byte{requestPath: append(requestRaw, '\n'), r.Plan: append(planRaw, '\n'), r.Certificate: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), r.PrivateKey: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}), r.Clock: []byte("live clock observation\n")} {
		mode := os.FileMode(0600)
		if path == r.Clock {
			mode = 0644
		}
		if err := os.WriteFile(path, raw, mode); err != nil {
			t.Fatal(err)
		}
		if path == r.PrivateKey {
			clear(raw)
		}
	}
	return inputFixture{requestPath, prefix, r, p}
}

func nativeInputFDCount(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal("invalid environment: actual proc FD inventory required", err)
	}
	return len(entries)
}

func TestJoinInputsNativeOriginalCustodyAndLiveClock(t *testing.T) {
	f := prepareInputFixture(t)
	input, err := ReadInputs(t.Context(), f.path)
	if err != nil {
		t.Fatal(err)
	}
	copy := input
	if input.original.request != f.request {
		t.Fatal("detached request changed original custody")
	}
	// The declared root writer may update or replace this live file. Neither
	// its inode nor its mtime/content is frozen as a substitute for State time.
	if err := os.WriteFile(f.request.Clock, []byte("new observation\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := input.Check(); err != nil {
		t.Fatal("live clock write was frozen", err)
	}
	if err := os.Rename(f.request.Clock, f.request.Clock+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.request.Clock, []byte("new original inode\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := input.Check(); err != nil {
		t.Fatal("live clock inode was frozen", err)
	}
	old := filepath.Dir(f.request.Clock) + ".retained"
	if err := os.Rename(filepath.Dir(f.request.Clock), old); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Dir(f.request.Clock), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.request.Clock, []byte("foreign ancestor\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := input.Check(); err == nil {
		t.Fatal("changed original clock ancestor accepted")
	}
	if err := copy.Close(); err == nil || input.Check() == nil {
		t.Fatal("copied/closed input forgot the original failure")
	}
	if _, err := os.Lstat(f.prefix); !os.IsNotExist(err) {
		t.Fatal("input inspection created a runtime root", err)
	}
}

func TestJoinInputsNativeRefusalReturnsOwnDescriptors(t *testing.T) {
	for _, fault := range []string{"request-mode", "plan-binding", "key-mode", "key-pair", "key-symlink", "key-hardlink", "plan-symlink", "ancestor-symlink", "clock-mode", "clock-symlink"} {
		t.Run(fault, func(t *testing.T) {
			f := prepareInputFixture(t)
			warm, err := ReadInputs(t.Context(), f.path)
			if err != nil {
				t.Fatal(err)
			}
			if err := warm.Close(); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "request-mode":
				err = os.Chmod(f.path, 0644)
			case "plan-binding":
				f.plan.HostingRoot += "-other-allowance"
				var raw []byte
				raw, err = json.Marshal(f.plan)
				if err == nil {
					err = os.WriteFile(f.request.Plan, append(raw, '\n'), 0600)
				}
			case "key-mode":
				err = os.Chmod(f.request.PrivateKey, 0644)
			case "key-pair":
				err = os.WriteFile(f.request.PrivateKey, []byte("not a matching Ed25519 key\n"), 0600)
			case "key-hardlink":
				err = os.Link(f.request.PrivateKey, f.request.PrivateKey+".alias")
			case "clock-mode":
				err = os.Chmod(f.request.Clock, 0666)
			default:
				path := f.request.PrivateKey
				if fault == "plan-symlink" {
					path = f.request.Plan
				} else if fault == "ancestor-symlink" {
					path = filepath.Dir(f.request.Clock)
				} else if fault == "clock-symlink" {
					path = f.request.Clock
				}
				err = os.Rename(path, path+".original")
				if err == nil {
					err = os.Symlink(path+".original", path)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			before := nativeInputFDCount(t)
			for range 16 {
				input, err := ReadInputs(t.Context(), f.path)
				if err == nil || input.Check() == nil {
					_ = input.Close()
					t.Fatal("foreign original input accepted")
				}
				if got := nativeInputFDCount(t); got != before {
					t.Fatal("refused constructor retained native descriptors without GC", before, got)
				}
			}
		})
	}
}
