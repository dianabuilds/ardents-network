//go:build linux && network_signing_native

package provisioning

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

	"github.com/dianabuilds/ardents-network/internal/domain/network/source"
	"github.com/dianabuilds/ardents-network/internal/domain/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/tests/networkfixture"
)

// The non-root child is an explicit numeric system-UID fixture, not an
// installed ardents-source principal/unit. Actual signatures, State acceptance,
// original files and CLI are genuine; no startup/Guard/serving is supplied.
func TestSourcePreparationNativeInitialNetworkAndNonrootState(t *testing.T) {
	parent := networkfixture.ProtectedExt4Parent(t)
	command, commandBytes := networkfixture.PinnedProvisionProgram(t, "ARDENTS_INFRASTRUCTURE_PROVISION_PROGRAM")
	run := func(args ...string) []byte {
		t.Helper()
		child := networkfixture.ProvisionCommand(t, command, append([]string{"source"}, args...)...)
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
	reference := time.Now().UTC().Truncate(time.Second)
	from, until := reference.Add(-time.Minute), reference.Add(time.Hour)
	var networkID, nodeID, assignmentSeed [32]byte
	for _, id := range []*[32]byte{&networkID, &nodeID, &assignmentSeed} {
		if _, err := rand.Read(id[:]); err != nil {
			t.Fatal(err)
		}
	}
	stateKeyRaw, err := hex.DecodeString(pins.State)
	if err != nil || len(stateKeyRaw) != 32 {
		t.Fatal("invalid State public key receipt", err)
	}
	var stateKey [32]byte
	copy(stateKey[:], stateKeyRaw)
	const endpoint = "127.0.0.1:4100"
	idHex := func(id [32]byte) string { return hex.EncodeToString(id[:]) }
	planDirectory := filepath.Join(parent, "public-plan")
	run("prepare-initial-plan", "--network", idHex(networkID), "--node", idHex(nodeID), "--node-key", pins.Node, "--state-key", pins.State, "--family", "one-effective-controller", "--endpoint", endpoint, "--valid-from", from.Format(time.RFC3339), "--valid-until", until.Format(time.RFC3339), "--assignment-seed", idHex(assignmentSeed), "--domains", "domain-a", "--output", planDirectory)
	plan := filepath.Join(planDirectory, "source-initial-network.json")
	raw, err := os.ReadFile(plan)
	if err != nil {
		t.Fatal(err)
	}
	var prepared struct {
		Network   string    `json:"network_id"`
		Node      string    `json:"node_id"`
		NodeKey   string    `json:"node_public_key"`
		StateKey  string    `json:"state_public_key"`
		Seed      string    `json:"assignment_seed"`
		Reference time.Time `json:"reference_time"`
	}
	if json.Unmarshal(raw, &prepared) != nil || prepared.Network != idHex(networkID) || prepared.Node != idHex(nodeID) || prepared.NodeKey != pins.Node || prepared.StateKey != pins.State || prepared.Seed != idHex(assignmentSeed) || prepared.Reference.Before(reference) || prepared.Reference.After(time.Now().UTC()) {
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
	// Keep every other real input and the valid private-key path unchanged so
	// a fresh output cannot be refused for a missing file instead of the pin.
	badPin := filepath.Join(parent, "bad-pin")
	child := networkfixture.ProvisionCommand(t, command, "source", "sign-initial-record", plan, preparedRecord, hex.EncodeToString(make([]byte, 32)), filepath.Join(keys, "source-node-key.pem"), badPin)
	if out, err := child.CombinedOutput(); err == nil || bytes.Contains(out, []byte("artifact_sha256")) {
		t.Fatal("wrong reviewed prepared commitment admitted", string(out), err)
	}
	if _, err := os.Lstat(badPin); !os.IsNotExist(err) {
		t.Fatal("wrong reviewed commitment created an output directory", err)
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
	client := filepath.Join(fixture, "ardents")
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
	participationRoot := filepath.Join(fixture, "participation")
	if err := os.Mkdir(participationRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(participationRoot, 199, 199); err != nil {
		t.Fatal(err)
	}
	serve := servingdeclaration.ServingDeclaration{Network: &servingdeclaration.ServingNetwork{Root: stateRoot, NetworkID: networkID, Authorities: [][32]byte{stateKey}, Threshold: 1, ProfileAuthority: stateKey, ClockObservationFile: clock},
		ProcessProfile: "ardents-source-process-v1", ParticipationRoot: participationRoot, Address: endpoint, Certificate: "/unopened-source-certificate", PrivateKey: "/unopened-source-private-key", ClientRoot: "/unopened-client-ca", ClientPins: [][32]byte{{1}}}
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
	child = exec.CommandContext(t.Context(), client, "source", "prepare-state", servePath, bundlePath)
	child.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 199, Gid: 199, Groups: []uint32{}}, Pdeathsig: syscall.SIGKILL}
	if out, err := child.CombinedOutput(); err != nil || !bytes.Contains(out, []byte(`"outcome":"state-prepared"`)) {
		t.Fatal("actual non-root State producer", string(out), err)
	}
	child = exec.CommandContext(t.Context(), client, "source", "prepare-state", servePath, bundlePath)
	child.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 199, Gid: 199, Groups: []uint32{}}, Pdeathsig: syscall.SIGKILL}
	if out, err := child.CombinedOutput(); err == nil || bytes.Contains(out, []byte("state-prepared")) {
		t.Fatal("prepared root automatically adopted", string(out), err)
	}
	if raw, err := os.ReadFile(filepath.Join(serve.ParticipationRoot, "current")); err != nil || len(raw) != 65 {
		t.Fatal("offline preparation did not retain initial participation", err)
	}
}
