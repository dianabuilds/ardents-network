//go:build network_signing_native

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/forwarding"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// Actual canonical producer outputs and State acceptance under the private stand's
// Node principal. This is a separate offline root: no installed Startup, duty,
// producer, Guard, key or Route/Hosting/Admission runtime is supplied. Fixture
// families are all one test controller, never independent operational families.
func TestForwardingStateNativeCanonicalOfflineAcceptanceAndNoReset(t *testing.T) {
	if planPath := os.Getenv("ARDENTS_FORWARDING_STATE_VERIFY"); planPath != "" {
		if _, err := forwardingStatePrincipal(); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(planPath)
		if err != nil {
			t.Fatal(err)
		}
		plan, err := forwarding.DecodePlan(raw)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := forwardingPreparationStateConfig(plan)
		if err != nil {
			t.Fatal(err)
		}
		owner, err := state.OpenRetained(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if os.Getenv("ARDENTS_FORWARDING_STATE_VERIFY_PARTIAL") == "1" {
			current, err := owner.Current()
			if err != nil || current.Epoch != 1 || current.Profile != "ardents-route-v3" {
				_ = owner.Close()
				t.Fatal("original signed Epoch not retained after profile refusal", err)
			}
			if _, err := owner.CurrentRuntime(); err == nil {
				_ = owner.Close()
				t.Fatal("partial Epoch supplied missing profile authority")
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			return
		}
		view, err := owner.CurrentRuntime()
		if err != nil {
			_ = owner.Close()
			t.Fatal(err)
		}
		member, err := view.Member(plan.NodeID, view.ObservedAt())
		expectedSubrole := uint8(1)
		if os.Getenv("ARDENTS_FORWARDING_STATE_VERIFY_INTERIOR") == "1" {
			expectedSubrole = 2
		}
		if err != nil || member.RoleDomain != 4 || member.Subrole != expectedSubrole {
			_ = owner.Close()
			t.Fatal("retained signed Forwarding membership differs", err)
		}
		if peer := os.Getenv("ARDENTS_FORWARDING_STATE_VERIFY_PEER"); peer != "" {
			raw, err := hex.DecodeString(peer)
			if err != nil || len(raw) != 32 {
				_ = owner.Close()
				t.Fatal("exact public Interior ID missing", err)
			}
			var id [32]byte
			copy(id[:], raw)
			peerMember, err := view.Member(id, view.ObservedAt())
			if err != nil || peerMember.NodeID != id || peerMember.RoleDomain != 4 || peerMember.Subrole != 2 || peerMember.PublicKey == [32]byte{} || peerMember.FamilyID == member.FamilyID {
				_ = owner.Close()
				t.Fatal("independent retained State did not bind the actual Interior", err)
			}
		}
		if err := owner.Close(); err != nil {
			t.Fatal(err)
		}
		return
	}
	if os.Geteuid() != 0 {
		t.Fatal("invalid environment: original private root driver required")
	}
	marker, err := os.Lstat("/etc/forwarding-initial-component-marker")
	if err != nil || !marker.Mode().IsRegular() || marker.Sys().(*syscall.Stat_t).Uid != 0 {
		t.Fatal("invalid environment: private initial-stopped stand required", err)
	}
	account, err := user.Lookup("ardents-node")
	if err != nil {
		t.Fatal("actual original Node account unavailable", err)
	}
	uid, err := strconv.ParseUint(account.Uid, 10, 32)
	if err != nil || uid < 100 || uid > 999 || account.Gid != account.Uid {
		t.Fatal("invalid Node principal", err)
	}
	command, commandBytes := sourceNativePinnedProgram(t, "ARDENTS_FORWARDING_CONTROL_PROGRAM")
	_ = sourceArtifactNativeParent(t)
	completed, err := forwarding.ReadCompleted(t.Context(), os.Getenv("ARDENTS_FORWARDING_INITIAL_BINDING"))
	if err != nil {
		t.Fatal("genuine private completed installation prerequisite", err)
	}
	if err := completed.Close(); err != nil {
		t.Fatal(err)
	}
	parent, err := os.MkdirTemp("/var/lib", "ardents-forwarding-state-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(parent); err != nil {
			t.Error(err)
		}
	})
	var nativeFS syscall.Statfs_t
	if err := syscall.Statfs(parent, &nativeFS); err != nil || nativeFS.Type != 0xef53 {
		t.Fatal("invalid environment: actual native ext4 required", err)
	}
	if err := os.Chmod(parent, 0755); err != nil {
		t.Fatal(err)
	}
	client := filepath.Join(parent, "ardents-control")
	if err := os.WriteFile(client, commandBytes, 0555); err != nil {
		t.Fatal(err)
	}
	testBytes, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	testProgram := filepath.Join(parent, "state-verifier.test")
	if err := os.WriteFile(testProgram, testBytes, 0555); err != nil {
		t.Fatal(err)
	}
	bundle, profile, public := prepareForwardingStateNativeInputs(t, command, parent)
	p := forwardingStatePlanFixture()
	p.NodeID = [32]byte{3} // the signed Forwarding member, distinct from the profile's Issuer
	p.RetirementWaitMS = 5000
	p.Network.Authorities = [][32]byte{public}
	p.Network.ProfileAuthority = public
	p.Network.ClockObservationFile = filepath.Join(parent, "clock")
	p.ParticipationRoot = filepath.Join(parent, "unopened-participation")
	p.SpendRoot = filepath.Join(parent, "unopened-spend")
	p.HostingRoot = filepath.Join(parent, "unopened-hosting")
	p.Certificate = filepath.Join(parent, "unopened-cert")
	p.PrivateKey = filepath.Join(parent, "unopened-key")
	bundlePath, profilePath := filepath.Join(parent, "bundle.bin"), filepath.Join(parent, "profile.bin")
	for path, raw := range map[string][]byte{bundlePath: bundle, profilePath: profile, p.Network.ClockObservationFile: []byte("independent offline observation\n")} {
		if err := os.WriteFile(path, raw, 0644); err != nil {
			t.Fatal(err)
		}
	}
	writePlan := func(root, name string) string {
		t.Helper()
		p.Network.Root = root
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chown(root, int(uid), int(uid)); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, '\n')
		path := filepath.Join(parent, name)
		if err := os.WriteFile(path, raw, 0644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	run := func(plan, profile string, principal uint32) ([]byte, error) {
		at := time.Now()
		if err := os.Chtimes(p.Network.ClockObservationFile, at, at); err != nil {
			t.Fatal(err)
		}
		child := exec.CommandContext(t.Context(), client, "forwarding", "prepare-state", plan, bundlePath, profile)
		child.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: principal, Gid: principal, Groups: []uint32{}}, Pdeathsig: syscall.SIGKILL}
		return child.CombinedOutput()
	}
	stateRoot := filepath.Join(parent, "accepted-state")
	planPath := writePlan(stateRoot, "plan.json")
	if raw, err := run(planPath, profilePath, uint32(uid)); err != nil || !bytes.Equal(raw, []byte("{\"operation\":\"forwarding.prepare-state\",\"state_accepted\":true,\"runtime_authority\":false}\n")) {
		t.Fatal("actual canonical non-root State/profile acceptance", err, string(raw))
	}
	at := time.Now()
	if err := os.Chtimes(p.Network.ClockObservationFile, at, at); err != nil {
		t.Fatal(err)
	}
	verify := exec.CommandContext(t.Context(), testProgram, "-test.run", "^TestForwardingStateNativeCanonicalOfflineAcceptanceAndNoReset$", "-test.count=1", "-test.v", "-test.timeout=30s")
	verify.Env = append(os.Environ(), "ARDENTS_FORWARDING_STATE_VERIFY="+planPath)
	verify.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(uid), Groups: []uint32{}}, Pdeathsig: syscall.SIGKILL}
	if raw, err := verify.CombinedOutput(); err != nil {
		t.Fatal("independent retained State reopen", err, string(raw))
	}
	stamp := func(root string) map[string][32]byte {
		t.Helper()
		result := map[string][32]byte{}
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type().IsRegular() {
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				result[path] = sha256.Sum256(raw)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return result
	}
	before := stamp(stateRoot)
	if raw, err := run(planPath, profilePath, uint32(uid)); err == nil || bytes.Contains(raw, []byte(`"state_accepted":true`)) {
		t.Fatal("existing State automatically initialized again", err, string(raw))
	}
	after := stamp(stateRoot)
	if len(before) != len(after) {
		t.Fatal("refused retry changed retained State inventory")
	}
	for path, hash := range before {
		if after[path] != hash {
			t.Fatal("refused retry changed retained State bytes")
		}
	}
	if raw, err := exec.CommandContext(t.Context(), command, "forwarding", "prepare-state", planPath, bundlePath, profilePath).CombinedOutput(); err == nil || bytes.Contains(raw, []byte(`"state_accepted":true`)) {
		t.Fatal("root supplied Node preparation principal", err, string(raw))
	}
	rootRefused := stamp(stateRoot)
	if len(rootRefused) != len(before) {
		t.Fatal("refused root principal changed State inventory")
	}
	for path, hash := range before {
		if rootRefused[path] != hash {
			t.Fatal("refused root principal changed State bytes")
		}
	}
	bad := append([]byte(nil), profile...)
	bad[len(bad)-1] ^= 1
	badProfile := filepath.Join(parent, "bad-profile.bin")
	if err := os.WriteFile(badProfile, bad, 0644); err != nil {
		t.Fatal(err)
	}
	partialRoot := filepath.Join(parent, "partial-state")
	partialPlan := writePlan(partialRoot, "partial-plan.json")
	if raw, err := run(partialPlan, badProfile, uint32(uid)); err == nil || bytes.Contains(raw, []byte(`"state_accepted":true`)) {
		t.Fatal("damaged profile accepted", err, string(raw))
	}
	at = time.Now()
	if err := os.Chtimes(p.Network.ClockObservationFile, at, at); err != nil {
		t.Fatal(err)
	}
	partialVerify := exec.CommandContext(t.Context(), testProgram, "-test.run", "^TestForwardingStateNativeCanonicalOfflineAcceptanceAndNoReset$", "-test.count=1", "-test.v", "-test.timeout=30s")
	partialVerify.Env = append(os.Environ(), "ARDENTS_FORWARDING_STATE_VERIFY="+partialPlan, "ARDENTS_FORWARDING_STATE_VERIFY_PARTIAL=1")
	partialVerify.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(uid), Groups: []uint32{}}, Pdeathsig: syscall.SIGKILL}
	if raw, err := partialVerify.CombinedOutput(); err != nil {
		t.Fatal("independent partial Epoch reopen", err, string(raw))
	}
	partial := stamp(partialRoot)
	if len(partial) == 0 {
		t.Fatal("partial actual State provenance disappeared")
	}
	if raw, err := run(partialPlan, profilePath, uint32(uid)); err == nil || bytes.Contains(raw, []byte(`"state_accepted":true`)) {
		t.Fatal("partial State reset on retry", err, string(raw))
	}
	partialAfter := stamp(partialRoot)
	if len(partial) != len(partialAfter) {
		t.Fatal("retry rewrote partial State inventory")
	}
	for path, hash := range partial {
		if partialAfter[path] != hash {
			t.Fatal("retry rewrote partial State bytes")
		}
	}
	for _, path := range []string{p.ParticipationRoot, p.SpendRoot, p.HostingRoot, p.Certificate, p.PrivateKey} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("offline State opened another purpose root or key", err)
		}
	}
}
