//go:build linux && network_signing_native

package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
)

// The non-root child is an explicit numeric system-UID fixture, not an
// installed ardents-source principal/unit. Actual signatures, State acceptance,
// original files and CLI are genuine; no startup/Guard/serving is supplied.
func TestSourcePreparationNativeInitialNetworkAndNonrootState(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	command, commandBytes := sourceNativePinnedProgram(t, "ARDENTS_SOURCE_CONTROL_PROGRAM")
	run := func(args ...string) []byte {
		t.Helper()
		child := exec.CommandContext(t.Context(), command, append([]string{"source"}, args...)...)
		raw, err := child.CombinedOutput()
		if err != nil {
			t.Fatal("actual initial preparation command", args[0], string(raw), err)
		}
		return raw
	}
	keys := filepath.Join(parent, "keys")
	keyReceipt := run("prepare-initial-keys", keys)
	var pins struct {
		State string `json:"state_public_key"`
		Node  string `json:"node_public_key"`
	}
	if json.Unmarshal(keyReceipt, &pins) != nil || len(pins.State) != 64 || len(pins.Node) != 64 || pins.State == pins.Node {
		t.Fatal("distinct public key receipt", string(keyReceipt))
	}
	var p sourceInitialNetworkPlan
	p.Schema = "ardents-source-initial-network-plan-v1"
	p.Reference = time.Now().UTC().Truncate(time.Second)
	p.From = p.Reference.Add(-time.Minute)
	p.Until = p.Reference.Add(time.Hour)
	for _, id := range []*sourceInitialID{&p.Network, &p.Node, &p.Seed} {
		if _, err := rand.Read(id[:]); err != nil {
			t.Fatal(err)
		}
	}
	decode := func(text string, destination *sourceInitialID) {
		raw, err := hex.DecodeString(text)
		if err != nil || len(raw) != 32 {
			t.Fatal(err)
		}
		copy(destination[:], raw)
	}
	decode(pins.State, &p.StateKey)
	decode(pins.Node, &p.NodeKey)
	p.Family = "one-effective-controller"
	p.Endpoint = "127.0.0.1:4100"
	p.Domains = []string{"domain-a"}
	planDirectory := filepath.Join(parent, "public-plan")
	idHex := func(id sourceInitialID) string { return hex.EncodeToString(id[:]) }
	run("prepare-initial-plan", "--network", idHex(p.Network), "--node", idHex(p.Node), "--node-key", pins.Node, "--state-key", pins.State, "--family", p.Family, "--endpoint", p.Endpoint, "--valid-from", p.From.Format(time.RFC3339), "--valid-until", p.Until.Format(time.RFC3339), "--assignment-seed", idHex(p.Seed), "--domains", "domain-a", "--output", planDirectory)
	plan := filepath.Join(planDirectory, "source-initial-network.json")
	raw, err := os.ReadFile(plan)
	if err != nil {
		t.Fatal(err)
	}
	var prepared sourceInitialNetworkPlan
	if json.Unmarshal(raw, &prepared) != nil || prepared.Network != p.Network || prepared.Node != p.Node || prepared.NodeKey != p.NodeKey || prepared.StateKey != p.StateKey || prepared.Seed != p.Seed || prepared.Reference.Before(p.Reference) || prepared.Reference.After(time.Now().UTC()) {
		t.Fatal("public flag preparation differs", err)
	}
	unsignedRecord := filepath.Join(parent, "unsigned-record")
	run("prepare-initial-record", plan, unsignedRecord)
	preparedRecord := filepath.Join(unsignedRecord, "node-record.signing-input")
	digestFile := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(raw)
		return hex.EncodeToString(digest[:])
	}
	signedRecord := filepath.Join(parent, "signed-record")
	run("sign-initial-record", plan, preparedRecord, digestFile(preparedRecord), filepath.Join(keys, "source-node-key.pem"), signedRecord)
	record := filepath.Join(signedRecord, "node-record.bin")
	unsignedEpoch := filepath.Join(parent, "unsigned-epoch")
	run("prepare-initial-epoch", plan, record, unsignedEpoch)
	preparedEpoch := filepath.Join(unsignedEpoch, "epoch.unsigned")
	signedEpoch := filepath.Join(parent, "signed-epoch")
	run("sign-initial-epoch", plan, record, preparedEpoch, digestFile(preparedEpoch), filepath.Join(keys, "source-state-key.pem"), signedEpoch)
	bundleRaw, err := os.ReadFile(filepath.Join(signedEpoch, "source-bundle.bin"))
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := source.DecodeBundle(bundleRaw)
	if err != nil || len(bundle.Inputs) != 1 || len(bundle.Materials) != 1 {
		t.Fatal("finite signed Source bundle", err)
	}
	// A different reviewed commitment cannot authorize private-key consumption.
	if result, err := prepareSourceInitialNetwork(t.Context(), []string{"sign-initial-record", plan, preparedRecord, hex.EncodeToString(make([]byte, 32)), "/unopened-private-key", filepath.Join(parent, "bad-pin")}, nil); err == nil || result != nil {
		t.Fatal("wrong reviewed prepared commitment admitted")
	}
	publicParent := os.Getenv("ARDENTS_SOURCE_STATE_FIXTURE_PARENT")
	info, err := os.Lstat(publicParent)
	if err != nil || !filepath.IsAbs(publicParent) || info.Mode() != os.ModeDir|0755 || info.Sys().(*syscall.Stat_t).Uid != 0 {
		t.Fatal("invalid environment: separate protected cross-UID fixture parent required", err)
	}
	fixture, err := os.MkdirTemp(publicParent, "state-preparation-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(fixture); err != nil {
			t.Error(err)
		}
	})
	if err := os.Chmod(fixture, 0755); err != nil {
		t.Fatal(err)
	}
	client := filepath.Join(fixture, "ardents-control")
	if err := os.WriteFile(client, commandBytes, 0555); err != nil {
		t.Fatal(err)
	}
	stateRoot := filepath.Join(fixture, "state")
	if err := os.Mkdir(stateRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(stateRoot, 199, 199); err != nil {
		t.Fatal(err)
	}
	clock := filepath.Join(fixture, "clock")
	if err := os.WriteFile(clock, []byte("clock observation\n"), 0644); err != nil {
		t.Fatal(err)
	}
	serve := source.ServingDeclaration{Network: &source.ServingNetwork{Root: stateRoot, NetworkID: p.Network, Authorities: [][32]byte{p.StateKey}, Threshold: 1, ProfileAuthority: p.StateKey, ClockObservationFile: clock},
		ProcessProfile: "ardents-source-process-v1", ParticipationRoot: filepath.Join(fixture, "unopened-participation"), Address: p.Endpoint, Certificate: "/unopened-source-certificate", PrivateKey: "/unopened-source-private-key", ClientRoot: "/unopened-client-ca", ClientPins: [][32]byte{{1}}}
	serveRaw, err := json.Marshal(serve)
	if err != nil {
		t.Fatal(err)
	}
	servePath, bundlePath := filepath.Join(fixture, "serve.json"), filepath.Join(fixture, "bundle.bin")
	for path, raw := range map[string][]byte{servePath: serveRaw, bundlePath: bundleRaw} {
		if err := os.WriteFile(path, raw, 0644); err != nil {
			t.Fatal(err)
		}
	}
	child := exec.CommandContext(t.Context(), client, "source", "prepare-state", servePath, bundlePath)
	child.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 199, Gid: 199, Groups: []uint32{}}, Pdeathsig: syscall.SIGKILL}
	if out, err := child.CombinedOutput(); err != nil || !bytes.Contains(out, []byte(`"outcome":"state-prepared"`)) {
		t.Fatal("actual non-root State producer", string(out), err)
	}
	child = exec.CommandContext(t.Context(), client, "source", "prepare-state", servePath, bundlePath)
	child.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 199, Gid: 199, Groups: []uint32{}}, Pdeathsig: syscall.SIGKILL}
	if out, err := child.CombinedOutput(); err == nil || bytes.Contains(out, []byte("state-prepared")) {
		t.Fatal("prepared root automatically adopted", string(out), err)
	}
	if _, err := os.Lstat(serve.ParticipationRoot); !os.IsNotExist(err) {
		t.Fatal("offline preparation opened participation", err)
	}
}
