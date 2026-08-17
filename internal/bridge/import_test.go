package bridge_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/bridge"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestOwnerImportsIdempotentlyAndRestarts(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	owner := fixture.open(t)
	invite := fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter)

	accepted, err := owner.Import(invite)
	if err != nil || accepted.Class != "accepted" || accepted.Slot != 0 || accepted.Generation != 1 {
		t.Fatalf("first import = %+v, %v", accepted, err)
	}
	already, err := owner.Import(invite)
	if err != nil || already.Class != "already-present" || already.InviteID != accepted.InviteID {
		t.Fatalf("idempotent import = %+v, %v", already, err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}

	owner = fixture.open(t)
	t.Cleanup(func() { _ = owner.Close() })
	restarted, err := owner.Import(invite)
	if err != nil || restarted.Class != "already-present" || restarted.InviteID != accepted.InviteID {
		t.Fatalf("restart import = %+v, %v", restarted, err)
	}
	identity, candidate, err := owner.Contact()
	if err != nil || identity != fixture.members[0].identity || !bytes.Equal(candidate, fixture.candidate) {
		t.Fatalf("restarted contact = %x %x, %v", identity, candidate, err)
	}
}

func TestBridgeInviteGoldenEncoding(t *testing.T) {
	t.Parallel()
	encoded, err := os.ReadFile(filepath.Join("testdata", "invite.hex"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := hex.DecodeString(string(bytes.TrimSpace(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	fixture := newFixture(t)
	got := fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter)
	if !bytes.Equal(got, want) {
		t.Fatal("canonical Bridge Invite differs from its frozen golden bytes")
	}
}

func TestOwnerRejectsBeforeChangingLogicalState(t *testing.T) {
	t.Parallel()
	tests := map[string]func(*inviteFields){
		"wrong network":    func(f *inviteFields) { f.network[0] ^= 1 },
		"wrong epoch":      func(f *inviteFields) { f.epoch++ },
		"wrong digest":     func(f *inviteFields) { f.epochDigest[0] ^= 1 },
		"wrong profile":    func(f *inviteFields) { f.profile = "other" },
		"wrong domain":     func(f *inviteFields) { f.roleDomain = 2 },
		"wrong identity":   func(f *inviteFields) { f.identity[0] ^= 1 },
		"wrong family":     func(f *inviteFields) { f.family[0] ^= 1 },
		"wrong record":     func(f *inviteFields) { f.recordDigest[0] ^= 1 },
		"wrong proof":      func(f *inviteFields) { f.domainProof[0] ^= 1 },
		"wrong assignment": func(f *inviteFields) { f.assignmentNotAfter-- },
		"future":           func(f *inviteFields) { f.notBefore += 120 },
		"expired":          func(f *inviteFields) { f.notAfter = f.notBefore - 1 },
		"bad signature":    func(f *inviteFields) { f.corruptSignature = true },
		"trailing":         func(f *inviteFields) { f.trailing = true },
	}
	expected := map[string]string{
		"wrong network": "incompatible", "wrong epoch": "incompatible",
		"wrong digest": "incompatible", "wrong profile": "incompatible",
		"wrong domain": "wrong-domain", "wrong identity": "wrong-domain",
		"wrong family": "wrong-domain", "wrong record": "wrong-domain",
		"wrong proof": "wrong-domain", "wrong assignment": "wrong-domain",
		"future": "incompatible", "expired": "expired", "bad signature": "invalid", "trailing": "invalid",
	}
	for name, mutate := range tests {
		name, mutate := name, mutate
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fixture := newFixture(t)
			owner := fixture.open(t)
			defer owner.Close()
			fields := fixture.fields(0, 1, nil, fixture.notBefore, fixture.notAfter)
			mutate(&fields)
			result, err := owner.Import(fixture.encode(t, fields))
			if err != nil || string(result.Class) != expected[name] {
				t.Fatalf("Import() = %+v, %v, want %s", result, err, expected[name])
			}
			valid, err := owner.Import(fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter))
			if err != nil || valid.Class != "accepted" {
				t.Fatalf("valid import after rejection = %+v, %v", valid, err)
			}
		})
	}
}

func TestOwnerEnforcesTwoSlotsAndOneReplacement(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	owner := fixture.open(t)
	defer owner.Close()

	first0 := fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter)
	result0, _ := owner.Import(first0)
	first1 := fixture.inviteFor(t, 1, 1, 1, nil, fixture.notBefore, fixture.notAfter)
	result1, _ := owner.Import(first1)
	if result0.Class != "accepted" || result1.Class != "accepted" {
		t.Fatalf("initial slots = %q, %q", result0.Class, result1.Class)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	owner = fixture.open(t)
	defer owner.Close()
	for _, invite := range [][]byte{first0, first1} {
		if result, err := owner.Import(invite); err != nil || result.Class != "already-present" {
			t.Fatalf("distinct member restart = %+v, %v", result, err)
		}
	}

	wrongID := [32]byte{9}
	rejected, err := owner.Import(fixture.inviteFor(t, 2, 0, 2, &wrongID, fixture.notBefore, fixture.notAfter))
	if err != nil || rejected.Class != "replacement-rejected" {
		t.Fatalf("wrong replacement = %+v, %v", rejected, err)
	}
	replacement, err := owner.Import(fixture.inviteFor(t, 2, 0, 2, &result0.InviteID, fixture.notBefore, fixture.notAfter))
	if err != nil || replacement.Class != "accepted" || replacement.Generation != 2 {
		t.Fatalf("replacement = %+v, %v", replacement, err)
	}
	replay, err := owner.Import(first0)
	if err != nil || replay.Class != "replay" {
		t.Fatalf("retired replay = %+v, %v", replay, err)
	}
	third, err := owner.Import(fixture.inviteFor(t, 3, 0, 2, &replacement.InviteID, fixture.notBefore, fixture.notAfter))
	if err != nil || third.Class != "replacement-rejected" {
		t.Fatalf("second replacement = %+v, %v", third, err)
	}
}

func TestFailedImportChangesNoDurableBytes(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	owner := fixture.open(t)
	defer owner.Close()
	if result, err := owner.Import(fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter)); err != nil || result.Class != "accepted" {
		t.Fatalf("initial import = %+v, %v", result, err)
	}
	before := durableFiles(t, fixture.root)
	fields := fixture.fields(1, 1, nil, fixture.notBefore, fixture.notAfter)
	fields.corruptSignature = true
	if result, err := owner.Import(fixture.encode(t, fields)); err != nil || result.Class != "invalid" {
		t.Fatalf("invalid import = %+v, %v", result, err)
	}
	after := durableFiles(t, fixture.root)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("durable bytes changed after rejection\nbefore: %v\nafter: %v", before, after)
	}
}

func TestOwnerErasesExpiredSecretsAndRetainsReplayFloor(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	invite := fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter)
	owner := fixture.open(t)
	if result, err := owner.Import(invite); err != nil || result.Class != "accepted" {
		t.Fatalf("initial import = %+v, %v", result, err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	fixture.now = fixture.notAfter.Add(time.Second)
	owner = fixture.open(t)
	if result, err := owner.Import(invite); err != nil || result.Class != "replay" {
		t.Fatalf("expired replay = %+v, %v", result, err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	for name, raw := range durableFiles(t, fixture.root) {
		if bytes.Contains(raw, fixture.candidate) {
			t.Fatalf("expired candidate secret remains in %s", name)
		}
	}
}

func TestOwnerLeaseAndInterruptedStagingRecovery(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	owner := fixture.open(t)
	if second, err := bridge.Open(fixture.config()); err == nil {
		_ = second.Close()
		t.Fatal("second owner acquired the same state root")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(fixture.root, ".stage-interrupted")
	pointer := filepath.Join(fixture.root, ".current-interrupted")
	orphanRaw := []byte("orphan generation")
	orphan := filepath.Join(fixture.root, "state-"+hexDigest(orphanRaw))
	for path, raw := range map[string][]byte{stage: {1}, pointer: {2}, orphan: orphanRaw} {
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	owner = fixture.open(t)
	defer owner.Close()
	for _, path := range []string{stage, pointer, orphan} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("interrupted artifact remains: %s (%v)", path, err)
		}
	}
}

func TestOwnerRepairsCurrentPointerRollbackFromWatermark(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	owner := fixture.open(t)
	first := fixture.invite(t, 0, 1, nil, fixture.notBefore, fixture.notAfter)
	second := fixture.inviteFor(t, 1, 1, 1, nil, fixture.notBefore, fixture.notAfter)
	if result, err := owner.Import(first); err != nil || result.Class != "accepted" {
		t.Fatalf("first import = %+v, %v", result, err)
	}
	oldPointer, err := os.ReadFile(filepath.Join(fixture.root, "current"))
	if err != nil {
		t.Fatal(err)
	}
	if result, err := owner.Import(second); err != nil || result.Class != "accepted" {
		t.Fatalf("second import = %+v, %v", result, err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.root, "current"), oldPointer, 0o600); err != nil {
		t.Fatal(err)
	}
	owner = fixture.open(t)
	defer owner.Close()
	if result, err := owner.Import(second); err != nil || result.Class != "already-present" {
		t.Fatalf("watermark recovery = %+v, %v", result, err)
	}
}

type fixture struct {
	root      string
	now       time.Time
	notBefore time.Time
	notAfter  time.Time
	snapshot  state.Snapshot
	members   [4]memberFixture
	candidate []byte
}

type memberFixture struct {
	private      ed25519.PrivateKey
	issuerID     [32]byte
	identity     [32]byte
	family       [32]byte
	recordDigest [32]byte
	domainProof  []byte
}

type inviteFields struct {
	network, epochDigest, identity, family, recordDigest [32]byte
	epoch                                                uint64
	profile                                              string
	roleDomain                                           byte
	domainProof                                          []byte
	assignmentNotAfter, notBefore, notAfter              int64
	slotGeneration, slot                                 byte
	replaces                                             *[32]byte
	candidate                                            []byte
	issuerID                                             [32]byte
	private                                              ed25519.PrivateKey
	corruptSignature, trailing                           bool
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	now := time.Unix(1_800_000_000, 0).UTC()
	value := fixture{
		root: filepath.Join(t.TempDir(), "bridge-state"), now: now,
		notBefore: now.Add(-time.Minute), notAfter: now.Add(30 * time.Minute),
		candidate: candidateEnvelope(),
	}
	value.snapshot = state.Snapshot{
		Generation: "authenticated-generation", NetworkID: [32]byte{1}, Epoch: 7, Digest: [32]byte{2},
		EpochValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), Freshness: "fresh",
		CandidateCount: 4,
	}
	for index := range value.members {
		label := "ardents-stage-5-bridge-fixture-key"
		if index > 0 {
			label += string(rune('0' + index))
		}
		seed := sha256.Sum256([]byte(label))
		private := ed25519.NewKeyFromSeed(seed[:])
		public := private.Public().(ed25519.PublicKey)
		member := memberFixture{
			private: private, issuerID: sha256.Sum256(public), identity: [32]byte{byte(3 + index*3)},
			family: [32]byte{byte(4 + index*3)}, recordDigest: [32]byte{byte(5 + index*3)},
			domainProof: []byte("authenticated-initiator-proof" + string(rune('0'+index))),
		}
		if index == 0 {
			member.domainProof = []byte("authenticated-initiator-proof")
		}
		value.members[index] = member
		candidate := &value.snapshot.Candidates[index]
		candidate.NodeID = member.identity
		candidate.KeyID = member.issuerID
		copy(candidate.PublicKey[:], public)
		candidate.FamilyID = member.family
		candidate.RecordDigest = member.recordDigest
		candidate.DomainProofDigest = sha256.Sum256(member.domainProof)
		candidate.Domain = "initiator"
		candidate.ValidFrom = now.Add(-time.Hour)
		candidate.ValidUntil = now.Add(time.Hour)
		candidate.AssignmentNotAfter = now.Add(time.Hour)
	}
	return value
}

type bridgeOwner interface {
	Import([]byte) (bridge.Result, error)
	Contact() ([32]byte, []byte, error)
	BeginContact([]byte, [32]byte, time.Time) ([32]byte, []byte, byte, uint64, time.Time, error)
	NextContact(context.Context) ([32]byte, []byte, byte, error)
	FinishContact(byte, uint64, bool, bool) error
	Acquire(context.Context, []byte, [32]byte, time.Time,
		func(context.Context, [32]byte, []byte, time.Time) (net.Conn, func() error, bool, error),
	) (net.Conn, func() error, error)
	Evidence() (bridge.AttemptEvidence, error)
	Close() error
}

func (f fixture) open(t *testing.T) bridgeOwner {
	t.Helper()
	owner, err := bridge.Open(f.config())
	if err != nil {
		t.Fatalf("open bridge owner: %v", err)
	}
	return owner
}

func (f fixture) config() bridge.Config {
	return bridge.Config{
		Root: f.root, RouteProfile: "h3-interactive-v1", Clock: func() time.Time { return f.now },
		TimeConfidence: func() bool { return true },
		CurrentNetwork: func() (state.Snapshot, error) { return f.snapshot, nil },
		RoleConflict:   func([32]byte, [32]byte) (bool, error) { return false, nil },
		ValidateCandidate: func(raw []byte, identity [32]byte) ([32]byte, string, error) {
			input := append(bytes.Clone(raw), identity[:]...)
			return sha256.Sum256(input), "test-adapter-v1", nil
		},
	}
}

func (f fixture) invite(t *testing.T, slot, generation byte, replaces *[32]byte, notBefore, notAfter time.Time) []byte {
	t.Helper()
	return f.inviteFor(t, 0, slot, generation, replaces, notBefore, notAfter)
}

func (f fixture) fields(slot, generation byte, replaces *[32]byte, notBefore, notAfter time.Time) inviteFields {
	return f.fieldsFor(0, slot, generation, replaces, notBefore, notAfter)
}

func (f fixture) inviteFor(t *testing.T, member int, slot, generation byte, replaces *[32]byte, notBefore, notAfter time.Time) []byte {
	t.Helper()
	return f.encode(t, f.fieldsFor(member, slot, generation, replaces, notBefore, notAfter))
}

func (f fixture) fieldsFor(memberIndex int, slot, generation byte, replaces *[32]byte, notBefore, notAfter time.Time) inviteFields {
	member := f.members[memberIndex]
	return inviteFields{
		network: f.snapshot.NetworkID, epoch: f.snapshot.Epoch, epochDigest: f.snapshot.Digest,
		profile: "h3-interactive-v1", roleDomain: 1, identity: member.identity,
		family: member.family, recordDigest: member.recordDigest,
		domainProof: bytes.Clone(member.domainProof), assignmentNotAfter: f.snapshot.Candidates[memberIndex].AssignmentNotAfter.Unix(),
		notBefore: notBefore.Unix(), notAfter: notAfter.Unix(), slotGeneration: generation,
		slot: slot, replaces: replaces, candidate: bytes.Clone(f.candidate), issuerID: member.issuerID, private: member.private,
	}
}

func (f fixture) encode(t *testing.T, fields inviteFields) []byte {
	t.Helper()
	var body bytes.Buffer
	_ = binary.Write(&body, binary.BigEndian, uint16(1))
	body.Write(fields.network[:])
	_ = binary.Write(&body, binary.BigEndian, fields.epoch)
	body.Write(fields.epochDigest[:])
	writeBytes(&body, []byte(fields.profile), 1)
	body.WriteByte(fields.roleDomain)
	body.Write(fields.identity[:])
	body.Write(fields.family[:])
	body.Write(fields.recordDigest[:])
	writeBytes(&body, fields.domainProof, 2)
	_ = binary.Write(&body, binary.BigEndian, fields.assignmentNotAfter)
	_ = binary.Write(&body, binary.BigEndian, fields.notBefore)
	_ = binary.Write(&body, binary.BigEndian, fields.notAfter)
	body.WriteByte(fields.slotGeneration)
	body.WriteByte(fields.slot)
	if fields.replaces == nil {
		body.WriteByte(0)
	} else {
		body.WriteByte(1)
		body.Write(fields.replaces[:])
	}
	writeBytes(&body, fields.candidate, 2)
	body.Write(fields.issuerID[:])

	var raw bytes.Buffer
	raw.WriteString("ardents-h3-bi1")
	_ = binary.Write(&raw, binary.BigEndian, uint16(body.Len()))
	raw.Write(body.Bytes())
	signatureInput := append([]byte("ardents-h3-bridge-invite-signature-v1\x00"), body.Bytes()...)
	signature := ed25519.Sign(fields.private, signatureInput)
	if fields.corruptSignature {
		signature[0] ^= 1
	}
	raw.Write(signature)
	if fields.trailing {
		raw.WriteByte(0)
	}
	return raw.Bytes()
}

func writeBytes(raw *bytes.Buffer, value []byte, width int) {
	if width == 1 {
		raw.WriteByte(byte(len(value)))
	} else {
		_ = binary.Write(raw, binary.BigEndian, uint16(len(value)))
	}
	raw.Write(value)
}

func candidateEnvelope() []byte {
	var raw bytes.Buffer
	raw.WriteString("ardents-h3-wt1")
	raw.WriteByte(1)
	writeBytes(&raw, []byte("webtunnel-v0.0.6"), 1)
	raw.Write([]byte{203, 0, 113, 7})
	_ = binary.Write(&raw, binary.BigEndian, uint16(443))
	writeBytes(&raw, []byte("/entry"), 2)
	writeBytes(&raw, []byte("front.example"), 1)
	raw.Write(bytes.Repeat([]byte{0x5a}, 32))
	return raw.Bytes()
}

func durableFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string][]byte)
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == ".ardents-bridge-state-lock" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		result[entry.Name()] = raw
	}
	return result
}

func hexDigest(raw []byte) string {
	digest := sha256.Sum256(raw)
	const alphabet = "0123456789abcdef"
	encoded := make([]byte, 64)
	for index, value := range digest {
		encoded[index*2] = alphabet[value>>4]
		encoded[index*2+1] = alphabet[value&15]
	}
	return string(encoded)
}
