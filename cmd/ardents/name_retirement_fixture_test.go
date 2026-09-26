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

	"github.com/dianabuilds/ardents-network/internal/naming"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/admission"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/epoch"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
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
	Schema                     string              `json:"schema"`
	StateRoot                  string              `json:"state_root"`
	NetworkID                  string              `json:"network_id"`
	AuthorityPublic            []string            `json:"authority_public"`
	AuthorityThreshold         int                 `json:"authority_threshold"`
	AcceptedProfile            string              `json:"accepted_profile"`
	SelectionAt                string              `json:"selection_at"`
	Deadline                   string              `json:"deadline"`
	RelayNodeID                string              `json:"relay_node_id"`
	GatewayNodeID              string              `json:"gateway_node_id"`
	ConnectionRendezvousNodeID string              `json:"connection_rendezvous_node_id"`
	AdmissionChallenge         admission.Challenge `json:"admission_challenge"`
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
// committed Namespace root carrying a signed Record, and plan/operation files
// shaped like the former name resolve / name control adapter inputs
// (ADR-0090). Since ADR-0100 removed the uncomposed private-resolution
// transport, the fixture composes no Gateway, Relay, or Resolver and starts no
// server; the retired commands must refuse before reading any of these files,
// and TestNameNetworkCommandsRetireBeforeEffects proves the refusal leaves
// both durable roots byte-for-byte unchanged.
func prepareRetiredNameInputs(t *testing.T) retiredNameInputs {
	t.Helper()
	root := t.TempDir()
	now := time.Unix(1_800_000_000, 0).UTC()
	deadline := now.Add(15 * time.Second)
	stateFixture := prepareRetiredNameState(t, root, now,
		[3]string{"127.0.0.1:7441", "127.0.0.1:7442", "127.0.0.1:7443"})

	namePrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{76}, ed25519.SeedSize))
	namePublic := namePrivate.Public().(ed25519.PublicKey)
	signed, err := record.SignRecord(stateFixture.network, record.Record{
		Name: "alice", Generation: 1, Revision: 1, Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: hex.EncodeToString(namePublic), Target: [32]byte{1},
		LeaseExpiresAt: now.Add(time.Hour).Unix(), GraceExpiresAt: now.Add(2 * time.Hour).Unix(),
		RecordNotAfter: now.Add(30 * time.Minute).UnixMilli(),
	}, namePrivate)
	if err != nil {
		t.Fatal(err)
	}
	public := stateFixture.authorities[0].Public().(ed25519.PublicKey)
	secondPublic := stateFixture.authorities[1].Public().(ed25519.PublicKey)
	firstID, secondID := sha256.Sum256(public), sha256.Sum256(secondPublic)
	policy := epoch.MaterializationPolicy{Network: stateFixture.network,
		Rule:        "ardents-namespace-materialization-v1",
		Authorities: map[[32]byte]ed25519.PublicKey{firstID: public, secondID: secondPublic}, Threshold: 2}
	namespaceRoot := filepath.Join(root, "namespace")
	store, err := epoch.Open(namespaceRoot, policy)
	if err != nil {
		t.Fatal(err)
	}
	storeClosed := false
	closeStore := func() error {
		if storeClosed {
			return nil
		}
		storeClosed = true
		return store.Close()
	}
	t.Cleanup(func() {
		if err := closeStore(); err != nil {
			t.Errorf("close Name fixture Namespace Store: %v", err)
		}
	})
	materialization := epoch.Epoch{Number: 1, Digest: stateFixture.digest, CutoffOffset: 1,
		TransitionRoot: sha256.Sum256([]byte("transitions")), TransitionLength: 1,
		RejectionRoot: sha256.Sum256([]byte("rejections"))}
	if err := store.CommitLegacy(materialization, [][]byte{signed}, func(transcript []byte) ([][32]byte, [][]byte, error) {
		ids := [][32]byte{firstID, secondID}
		signatures := [][]byte{ed25519.Sign(stateFixture.authorities[0], transcript),
			ed25519.Sign(stateFixture.authorities[1], transcript)}
		if bytes.Compare(ids[0][:], ids[1][:]) > 0 {
			ids[0], ids[1] = ids[1], ids[0]
			signatures[0], signatures[1] = signatures[1], signatures[0]
		}
		return ids, signatures, nil
	}); err != nil {
		t.Fatal(err)
	}
	gate, err := admission.NewAdmission([32]byte{2}, stateFixture.network, 1, [32]byte{77})
	if err != nil {
		t.Fatal(err)
	}
	isolation := [32]byte{1}
	resolveDigest := retiredNameResolutionDigest(t, stateFixture.network, "alice", deadline.UnixNano())
	resolveChallenge, err := gate.Issue(now.UnixMilli(), "resolution", resolveDigest, isolation,
		deadline.UnixMilli(), [16]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	operation := retiredNameRecordOperation{Kind: "record", Name: "alice", Generation: 1,
		ExpectedRevision: 1, Target: [32]byte{2}, RecordNotAfter: now.Add(time.Hour).Unix(),
		AuthorityProof: []byte("proof")}
	operationRaw, err := json.Marshal(operation)
	if err != nil {
		t.Fatal(err)
	}
	controlDigest := sha256.Sum256(append([]byte("ardents-name-control-operation-v1\x00"), operationRaw...))
	operation.OperationDigest = controlDigest
	controlChallenge, err := gate.Issue(now.UnixMilli(), "renewal-update", controlDigest, isolation,
		deadline.UnixMilli(), [16]byte{2})
	if err != nil {
		t.Fatal(err)
	}
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

	if err := closeStore(); err != nil {
		t.Fatal(err)
	}
	return retiredNameInputs{
		resolvePath:   retiredNameWriteJSON(t, root, "resolve.json", resolveInput),
		controlPath:   retiredNameWriteJSON(t, root, "control.json", controlInput),
		operationPath: retiredNameWriteJSON(t, root, "operation.json", operation),
		stateRoot:     stateFixture.root, namespaceRoot: namespaceRoot,
		isolation: hex.EncodeToString(isolation[:]),
	}
}

func retiredNameResolutionDigest(t *testing.T, network [32]byte, raw string, deadline int64) [32]byte {
	t.Helper()
	name, err := naming.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := naming.EncodeWire(name)
	if err != nil {
		t.Fatal(err)
	}
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
