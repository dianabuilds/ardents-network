package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

type retiredNameInputs struct {
	resolvePath   string
	controlPath   string
	operationPath string
	stateRoot     string
	namespaceRoot string
	isolation     string
}

type retiredNamePlan struct {
	Schema                     string               `json:"schema"`
	StateRoot                  string               `json:"state_root"`
	NetworkID                  string               `json:"network_id"`
	AuthorityPublic            []string             `json:"authority_public"`
	AuthorityThreshold         int                  `json:"authority_threshold"`
	AcceptedProfile            string               `json:"accepted_profile"`
	SelectionAt                string               `json:"selection_at"`
	Deadline                   string               `json:"deadline"`
	RelayNodeID                string               `json:"relay_node_id"`
	GatewayNodeID              string               `json:"gateway_node_id"`
	ConnectionRendezvousNodeID string               `json:"connection_rendezvous_node_id"`
	AdmissionChallenge         retiredNameChallenge `json:"admission_challenge"`
}

type retiredNameRecordOperation struct {
	Kind               string   `json:"kind"`
	SigningMode        string   `json:"signing_mode,omitempty"`
	OperationDigest    [32]byte `json:"operation_digest"`
	Network            [32]byte `json:"network,omitempty"`
	Nonce              [32]byte `json:"nonce,omitempty"`
	Deadline           int64    `json:"deadline,omitempty"`
	Name               string   `json:"name"`
	ParentName         string   `json:"parent_name,omitempty"`
	Generation         uint64   `json:"generation,omitempty"`
	ExpectedRevision   uint64   `json:"expected_revision,omitempty"`
	ParentGeneration   uint64   `json:"parent_generation,omitempty"`
	ParentRevision     uint64   `json:"parent_revision,omitempty"`
	ChildGeneration    uint64   `json:"child_generation,omitempty"`
	Authority          [32]byte `json:"authority,omitempty"`
	SuccessorAuthority [32]byte `json:"successor_authority,omitempty"`
	Target             [32]byte `json:"target,omitempty"`
	LeaseNotAfter      int64    `json:"lease_not_after,omitempty"`
	RecordNotAfter     int64    `json:"record_not_after,omitempty"`
	PolicyNotBefore    int64    `json:"policy_not_before,omitempty"`
	RecoveryNotBefore  int64    `json:"recovery_not_before,omitempty"`
	PolicyID           [32]byte `json:"policy_id,omitempty"`
	RecoveryStep       string   `json:"recovery_step,omitempty"`
	OrderingProof      []byte   `json:"ordering_proof,omitempty"`
	AuthorityProof     []byte   `json:"authority_proof,omitempty"`
	RecoveryPolicy     []byte   `json:"recovery_policy,omitempty"`
	RecoveryProof      []byte   `json:"recovery_proof,omitempty"`
	SuccessorRecord    []byte   `json:"successor_record,omitempty"`
}

// prepareRetiredNameInputs materializes one authenticated State root, one
// synthetic root shaped like the retired Namespace store, and plan/operation
// files shaped like the former name resolve / name control adapter inputs
// (ADR-0090). ADR-0105 deleted the whole Namespace control subsystem: no
// working-tree code can read, convert, or delete a real
// Namespace root, so the fixture no longer composes one, and the synthetic
// bytes stand in as the untouched durable data of an old root. The fixture
// composes no Gateway, Relay, or Resolver and starts no server; the retired
// commands must refuse before reading any of these files, and
// TestNameNetworkCommandsRetireBeforeEffects proves the refusal leaves
// both durable roots byte-for-byte unchanged.
func prepareRetiredNameInputs(t *testing.T) retiredNameInputs {
	t.Helper()
	root := t.TempDir()
	now := time.Unix(1_800_000_000, 0).UTC()
	deadline := now.Add(15 * time.Second)
	stateFixture := prepareRetiredNameState(t, root, now,
		[3]string{"127.0.0.1:7441", "127.0.0.1:7442", "127.0.0.1:7443"})
	namespaceRoot := retiredNameSyntheticNamespaceRoot(t, root)
	isolation := [32]byte{1}
	resolveDigest := retiredNameResolutionDigest(t, stateFixture.network, "alice", deadline.UnixNano())
	resolveChallenge := newRetiredNameChallenge(now, "resolution", resolveDigest, isolation, deadline, 1)
	operation := retiredNameRecordOperation{Kind: "record", Name: "alice", Generation: 1,
		ExpectedRevision: 1, Target: [32]byte{2}, RecordNotAfter: now.Add(time.Hour).Unix(),
		AuthorityProof: []byte("proof")}
	operationRaw, err := json.Marshal(operation)
	if err != nil {
		t.Fatal(err)
	}
	controlDigest := sha256.Sum256(append([]byte("ardents-name-control-operation-v1\x00"), operationRaw...))
	operation.OperationDigest = controlDigest
	controlChallenge := newRetiredNameChallenge(now, "renewal-update", controlDigest, isolation, deadline, 2)
	public := stateFixture.authorities[0].Public().(ed25519.PublicKey)
	secondPublic := stateFixture.authorities[1].Public().(ed25519.PublicKey)
	authorities := []string{hex.EncodeToString(public), hex.EncodeToString(secondPublic)}
	sort.Strings(authorities)
	base := retiredNamePlan{StateRoot: stateFixture.root, NetworkID: hex.EncodeToString(stateFixture.network[:]),
		AuthorityPublic: authorities, AuthorityThreshold: 2,
		AcceptedProfile: "h3-role-probe-v1", SelectionAt: now.Format(time.RFC3339Nano),
		Deadline: deadline.Format(time.RFC3339Nano), RelayNodeID: retiredNameNodeID(1),
		GatewayNodeID: retiredNameNodeID(2)}
	resolveInput := base
	resolveInput.Schema = "ardents-private-resolution-input-v1"
	resolveInput.ConnectionRendezvousNodeID = retiredNameNodeID(3)
	resolveInput.AdmissionChallenge = resolveChallenge
	controlInput := base
	controlInput.Schema = "ardents-private-name-control-input-v1"
	controlInput.ConnectionRendezvousNodeID = retiredNameNodeID(0)
	controlInput.AdmissionChallenge = controlChallenge
	return retiredNameInputs{
		resolvePath:   retiredNameWriteJSON(t, root, "resolve.json", resolveInput),
		controlPath:   retiredNameWriteJSON(t, root, "control.json", controlInput),
		operationPath: retiredNameWriteJSON(t, root, "operation.json", operation),
		stateRoot:     stateFixture.root, namespaceRoot: namespaceRoot,
		isolation: hex.EncodeToString(isolation[:]),
	}
}

// retiredNameChallenge is the synthetic stand-in for the deleted
// admission.Challenge grammar. The retired commands refuse before parsing
// any plan file, so only the presence of the field matters, never its
// contents.
type retiredNameChallenge struct {
	Schema    string `json:"schema"`
	IssuedAt  int64  `json:"issued_at"`
	Purpose   string `json:"purpose"`
	Digest    string `json:"digest"`
	Isolation string `json:"isolation"`
	Deadline  int64  `json:"deadline"`
	Counter   string `json:"counter"`
}

func newRetiredNameChallenge(now time.Time, purpose string, digest, isolation [32]byte, deadline time.Time, counter byte) retiredNameChallenge {
	return retiredNameChallenge{Schema: "ardents-namespace-admission-challenge-v1",
		IssuedAt: now.UnixMilli(), Purpose: purpose, Digest: hex.EncodeToString(digest[:]),
		Isolation: hex.EncodeToString(isolation[:]), Deadline: deadline.UnixMilli(),
		Counter: hex.EncodeToString([]byte{counter})}
}

// retiredNameSyntheticNamespaceRoot writes opaque durable bytes shaped like
// the retired Namespace store: a legacy marker, one epoch file, and one
// signed record file. No working-tree code reads them; they exist so the
// zero-effect oracle can prove that the refusal leaves old Namespace data
// byte-for-byte untouched on disk.
func retiredNameSyntheticNamespaceRoot(t *testing.T, root string) string {
	t.Helper()
	namespaceRoot := filepath.Join(root, "namespace")
	epochDir := filepath.Join(namespaceRoot, "epochs", "00000001")
	if err := os.MkdirAll(filepath.Join(epochDir, "records"), 0o700); err != nil {
		t.Fatal(err)
	}
	marker := []byte("ardents-namespace-store-v1\n")
	if err := os.WriteFile(filepath.Join(namespaceRoot, ".ardents-namespace-store-v1"), marker, 0o600); err != nil {
		t.Fatal(err)
	}
	namePrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{76}, ed25519.SeedSize))
	payload := append([]byte("ardents-name-record-v1\x00"), []byte("alice")...)
	signed := append(payload, ed25519.Sign(namePrivate, payload)...)
	recordDigest := sha256.Sum256(signed)
	epochRaw := append([]byte("ardents-namespace-epoch-v1\x00"), recordDigest[:]...)
	if err := os.WriteFile(filepath.Join(epochDir, "epoch.bin"), epochRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(epochDir, "records", "alice.bin"), signed, 0o600); err != nil {
		t.Fatal(err)
	}
	return namespaceRoot
}

func retiredNameResolutionDigest(t *testing.T, network [32]byte, raw string, deadline int64) [32]byte {
	t.Helper()
	// The frozen Stage 6 wire bytes are inlined after ADR-0113 deleted
	// the naming package: uint16 schema version 1 (big-endian) followed by
	// one uint8-length-prefixed label. The fixture only ever encodes the
	// single canonical label "alice" ("000105616c696365").
	if raw != "alice" {
		t.Fatalf("retired name fixture encodes only \"alice\", got %q", raw)
	}
	wire := []byte{0x00, 0x01, byte(len(raw))}
	wire = append(wire, raw...)
	transcript := []byte("ardents-name-resolution-operation-v1\x00")
	transcript = append(transcript, network[:]...)
	transcript = binary.BigEndian.AppendUint64(transcript, uint64(deadline))
	transcript = binary.BigEndian.AppendUint16(transcript, uint16(len(wire)))
	return sha256.Sum256(append(transcript, wire...))
}

func retiredNameNodeID(marker byte) string {
	return hex.EncodeToString(append([]byte{marker}, make([]byte, 31)...))
}

func retiredNameWriteJSON(t *testing.T, root, name string, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func retiredNameTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = content
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func retiredNameTreeUnchanged(t *testing.T, root string, before map[string][]byte) {
	t.Helper()
	if after := retiredNameTree(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("retired Name command changed durable root %s: before=%d files after=%d files", root, len(before), len(after))
	}
}
