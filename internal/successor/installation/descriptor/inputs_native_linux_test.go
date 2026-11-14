//go:build linux && network_signing_native

package descriptor

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// These generated TLS/public declaration fixtures prove protected input custody,
// never signed Network duty, Release permission or an installed Store ACK.
func descriptorNativeInputs(t *testing.T) (Request, string) {
	t.Helper()
	if os.Geteuid() != 0 || os.Getenv("ARDENTS_DESCRIPTOR_PREFLIGHT_NATIVE") != "1" {
		t.Fatal("invalid environment: explicit root Descriptor preflight stand required")
	}
	parent, err := os.MkdirTemp("/var/lib", "d548-input-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(parent); err != nil {
			t.Error(err)
		}
	})
	var fs syscall.Statfs_t
	if err := syscall.Statfs(parent, &fs); err != nil || fs.Type != 0xef53 {
		t.Fatal("invalid environment: protected ext4 required", err)
	}
	prefix := fmt.Sprintf("/var/lib/d548-in-%d", os.Getpid())
	if _, err := os.Lstat(prefix); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("invalid environment: future namespace must be absent", err)
	}
	// Provision only the protected enclosing namespace, never a semantic root.
	// The read-only owner must refuse missing parents instead of creating them.
	if err := os.Mkdir(prefix, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(prefix); err != nil {
			t.Error(err)
		}
	})
	r := Request{Schema: "ardents-descriptor-installation-v1", Invocation: "descriptor", Profile: "ardents-descriptor-process-v1", InstallationRoot: prefix + "/install", StateRoot: prefix + "/state", ParticipationRoot: prefix + "/participation", SpendRoot: prefix + "/spend", HostingRoot: prefix + "/hosting", DescriptorRoot: prefix + "/store", CredentialDirectory: prefix + "/credentials", Plan: parent + "/plan.json", Certificate: parent + "/certificate.pem", PrivateKey: parent + "/private-key.pem", Clock: parent + "/clock"}
	p := Plan{Schema: "ardents-descriptor-runtime-v1", Network: NetworkInputs{Root: r.StateRoot, NetworkID: [32]byte{1}, Authorities: [][32]byte{{2}}, Threshold: 1, ProfileAuthority: [32]byte{3}, ClockObservationFile: r.Clock}, NodeID: [32]byte{4}, ProcessProfile: r.Profile, ParticipationRoot: r.ParticipationRoot, SpendRoot: r.SpendRoot, HostingRoot: r.HostingRoot, DescriptorRoot: r.DescriptorRoot, Certificate: r.CredentialDirectory + "/certificate.pem", PrivateKey: r.CredentialDirectory + "/private-key.pem", RetirementWaitMS: 1000}
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
	marshal := func(value any) []byte {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return append(raw, '\n')
	}
	path := parent + "/request.json"
	for file, raw := range map[string][]byte{path: marshal(r), r.Plan: marshal(p), r.Certificate: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), r.PrivateKey: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}), r.Clock: []byte("live root clock declaration\n")} {
		mode := os.FileMode(0600)
		if file == r.Clock {
			mode = 0644
		}
		if err := os.WriteFile(file, raw, mode); err != nil {
			t.Fatal(err)
		}
		if file == r.PrivateKey {
			clear(raw)
		}
	}
	return r, path
}

func requireDescriptorFutureAbsent(t *testing.T, r Request) {
	t.Helper()
	for _, path := range append([]string{r.InstallationRoot, r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot, r.DescriptorRoot, r.CredentialDirectory}, r.reservedSiblingPaths()...) {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("read-only Descriptor observation created a future resource", path, err)
		}
	}
}

func TestDescriptorInputsNativeOriginalCustodyAndLiveClock(t *testing.T) {
	r, path := descriptorNativeInputs(t)
	input, err := ReadInputs(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	copy := input
	t.Cleanup(func() { _ = input.Close() })
	if err := os.Rename(r.Clock, r.Clock+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.Clock, []byte("fresh root observation\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := copy.Check(); err != nil {
		t.Fatal("live root clock was incorrectly frozen", err)
	}
	original, err := os.ReadFile(r.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(original)
	if err := os.WriteFile(r.PrivateKey, []byte("changed original key"), 0600); err != nil {
		t.Fatal(err)
	}
	if copy.Check() == nil {
		t.Fatal("original key mutation was accepted")
	}
	if err := os.WriteFile(r.PrivateKey, original, 0600); err != nil {
		t.Fatal(err)
	}
	if input.Check() == nil || input.Close() == nil || copy.Close() == nil {
		t.Fatal("restoration revived original input custody or erased failure")
	}
	// A mutation after the final explicit Check must still fail original Close.
	fresh, err := ReadInputs(t.Context(), path)
	t.Cleanup(func() { _ = fresh.Close() })
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.Plan, []byte("late plan mutation"), 0600); err != nil {
		t.Fatal(err)
	}
	if fresh.Close() == nil {
		t.Fatal("late original mutation was lost at physical release")
	}
	requireDescriptorFutureAbsent(t, r)
}

func TestDescriptorInputsNativeFreshManagerAndIndependentStore(t *testing.T) {
	r, path := descriptorNativeInputs(t)
	input, err := ReadInputs(t.Context(), path)
	t.Cleanup(func() { _ = input.Close() })
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(InspectInitialInputs(input), input.Close()); err != nil {
		t.Fatal("actual read-only five-slot preflight", err)
	}
	command := os.Getenv("ARDENTS_DESCRIPTOR_NODE_PROGRAM")
	program, err := os.ReadFile(command)
	pin := sha256.Sum256(program)
	if err != nil || !filepath.IsAbs(command) || hex.EncodeToString(pin[:]) != os.Getenv("ARDENTS_DESCRIPTOR_NODE_PROGRAM_SHA256") {
		t.Fatal("invalid environment: independently pinned actual Node required", err)
	}
	diagnostic := filepath.Join(filepath.Dir(path), "diagnostic.socket")
	child := exec.CommandContext(t.Context(), command, "descriptor", "inspect-inputs", path)
	child.Env = append(os.Environ(), "ARDENTS_DEBUG_SOCKET="+diagnostic)
	output, err := child.CombinedOutput()
	if err != nil || string(output) != "{\"outcome\":\"descriptor-inputs-preflight-completed\",\"installed\":false}\n" {
		t.Fatal("actual canonical Node read-only preflight", string(output), err)
	}
	if _, err := os.Lstat(diagnostic); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("read-only preflight opened process diagnostics", err)
	}
	requireDescriptorFutureAbsent(t, r)
	// A retained Store root denies initial admission even when all other future
	// roots are absent; inspection must preserve those bytes without adoption.
	if err := os.Mkdir(r.DescriptorRoot, 0700); err != nil {
		t.Fatal(err)
	}
	retained := r.DescriptorRoot + "/retained-floor"
	before := []byte("foreign existing Store history; preserve\n")
	if err := os.WriteFile(retained, before, 0600); err != nil {
		t.Fatal(err)
	}
	input, err = ReadInputs(t.Context(), path)
	retainedInput := input
	t.Cleanup(func() { _ = retainedInput.Close() })
	if err != nil {
		t.Fatal(err)
	}
	if err := InspectInitialInputs(input); !errors.Is(err, ErrBinding) {
		t.Fatal("existing Store did not deny initial preflight", err)
	}
	if err := input.Close(); !errors.Is(err, ErrBinding) {
		t.Fatal("original namespace refusal lost", err)
	}
	after, err := os.ReadFile(retained)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("existing Store history changed", err)
	}
}

func TestDescriptorInputsNativePrivateAccessAndStorePlanRefuse(t *testing.T) {
	for _, fault := range []string{"private-access", "store-binding"} {
		t.Run(fault, func(t *testing.T) {
			r, path := descriptorNativeInputs(t)
			if fault == "private-access" {
				if err := os.Chmod(r.PrivateKey, 0644); err != nil {
					t.Fatal(err)
				}
			} else {
				raw, err := os.ReadFile(r.Plan)
				if err != nil {
					t.Fatal(err)
				}
				raw = bytes.Replace(raw, []byte(`"descriptor_root":"`+r.DescriptorRoot+`"`), []byte(`"descriptor_root":"`+r.HostingRoot+`"`), 1)
				if err := os.WriteFile(r.Plan, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			input, err := ReadInputs(t.Context(), path)
			t.Cleanup(func() { _ = input.Close() })
			if err == nil || input.original != nil || input.Close() != nil {
				t.Fatal("invalid original input acquired custody", err)
			}
			requireDescriptorFutureAbsent(t, r)
		})
	}
}
