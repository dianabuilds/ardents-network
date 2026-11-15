package introduction

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/deployment"
	releaseartifact "github.com/dianabuilds/ardents-network/internal/successor/release/infrastructure"
)

// This independent transcript oracle enumerates the accepted order and lexical
// artifact inventory without calling a creator or its journal-writing helpers.
// It supplies parser bytes only, never native provenance or successful authority.
func initialTranscriptOracle(t *testing.T) (binding, map[string]string, []initialRecord) {
	t.Helper()
	hash := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	var request Request
	if err := json.Unmarshal([]byte(independentRequest), &request); err != nil {
		t.Fatal(err)
	}
	b := binding{Schema: "ardents-introduction-binding-v1", Invocation: "introduction", Generation: hash([]byte("artifact")), ProgramTarget: "ardents/linux-amd64/node", GenerationTarget: "ardents/linux-amd64/infrastructure/introduction", ProgramSHA256: hash([]byte("program")), TemplateSHA256: hash([]byte(deployment.IntroductionTemplate)), InventorySHA256: hash([]byte("inventory")), TargetsSHA256: hash([]byte("targets")), RequestSHA256: hash([]byte(independentRequest)), PlanSHA256: hash([]byte("plan")), CertificateSHA256: hash([]byte("certificate")), Account: "ardents-node", UID: 999, GID: 999, History: "/var/lib/release-history", Request: request}
	unit, err := renderUnit([]byte(deployment.IntroductionTemplate), request, b.Generation)
	if err != nil {
		t.Fatal(err)
	}
	b.UnitSHA256 = hash(unit)
	bound, _ := json.Marshal(b)
	bound = append(bound, '\n')
	boundSHA := hash(bound)
	base := filepath.Join(request.InstallationRoot, "generations")
	sealed := filepath.Join(base, b.Generation)
	hashes := map[string]string{}
	names := []string{"ardents-introduction.service", "ardents-node-linux-amd64", "binding.json", "infrastructure-enrollment.json", "infrastructure-introduction.json", "introduction-plan.json", "introduction-request.json", "rendered-introduction.service", "root.json", "snapshot.json", "targets.json", "timestamp.json"}
	for _, name := range names {
		hashes[filepath.Join(sealed, name)] = hash([]byte(name))
	}
	for name, value := range map[string]string{"ardents-introduction.service": b.TemplateSHA256, "ardents-node-linux-amd64": b.ProgramSHA256, "binding.json": boundSHA, "infrastructure-enrollment.json": b.InventorySHA256, "infrastructure-introduction.json": b.Generation, "introduction-plan.json": b.PlanSHA256, "introduction-request.json": b.RequestSHA256, "rendered-introduction.service": b.UnitSHA256, "targets.json": b.TargetsSHA256} {
		hashes[filepath.Join(sealed, name)] = value
	}
	pending := request.InstallationRoot + ".introduction-initial-pending"
	journal := request.InstallationRoot + ".introduction-initial-journal"
	selection := filepath.Join(request.InstallationRoot, "binding.json")
	unitPath := "/etc/systemd/system/ardents-introduction.service"
	hashes[pending] = hash([]byte("ardents-introduction-initial-pending-v1\n" + boundSHA + "\n"))
	hashes[selection] = boundSHA
	hashes[unitPath] = b.UnitSHA256
	hashes[filepath.Join(request.CredentialDirectory, "certificate.pem")] = b.CertificateSHA256
	hashes[filepath.Join(request.CredentialDirectory, "private-key.pem")] = hash([]byte("private fixture commitment"))
	records := []initialRecord{}
	inode := uint64(100)
	add := func(r initialRecord) { r.Sequence = len(records); records = append(records, r) }
	native := func(r initialRecord) initialRecord { inode++; r.Device = 1; r.Inode = inode; return r }
	write := func(path string, mode, uid, gid uint32) {
		add(initialRecord{Phase: "file-intent", Path: path, SHA256: hashes[path], Mode: mode, UID: uid, GID: gid})
		r := native(initialRecord{Phase: "file-born", Path: path, Mode: 0600})
		add(r)
		r.Phase = "file-ready"
		r.SHA256 = hashes[path]
		r.Mode = mode
		r.UID = uid
		r.GID = gid
		add(r)
	}
	add(native(initialRecord{Phase: "introduction-initial-v1", Path: journal, Mode: 0600}))
	add(initialRecord{Phase: "native-context", Path: request.InstallationRoot, SHA256: hash([]byte("native context"))})
	write(pending, 0600, 0, 0)
	add(native(initialRecord{Phase: "admitted", Path: b.History, SHA256: boundSHA, Mode: 0700}))
	accounts := []string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow"}
	for _, p := range accounts {
		add(native(initialRecord{Phase: "account-preimage", Path: p, SHA256: hash([]byte(p)), Mode: 0600}))
	}
	auxiliary := []string{"/etc/.pwd.lock", "/etc/passwd-", "/etc/group-", "/etc/shadow-", "/etc/gshadow-", "/etc/passwd.lock", "/etc/group.lock", "/etc/shadow.lock", "/etc/gshadow.lock", "/etc/passwd.1", "/etc/group.1", "/etc/shadow.1", "/etc/gshadow.1"}
	for _, p := range auxiliary {
		add(initialRecord{Phase: "account-auxiliary-preimage", Path: p})
	}
	add(initialRecord{Phase: "account-intent", Path: "ardents-node", UID: 999, GID: 999})
	for _, p := range accounts {
		add(native(initialRecord{Phase: "account-born", Path: p, SHA256: hash([]byte("new " + p)), Mode: 0600}))
	}
	for i, p := range auxiliary {
		if i < 5 {
			body := []byte(strings.TrimSuffix(p, "-"))
			if i == 0 {
				body = nil
			}
			add(native(initialRecord{Phase: "account-auxiliary-result", Path: p, SHA256: hash(body), Mode: 0600}))
		} else {
			add(initialRecord{Phase: "account-auxiliary-cleared", Path: p})
		}
	}
	add(initialRecord{Phase: "account-ready", Path: "ardents-node", UID: 999, GID: 999})
	for _, p := range []string{request.InstallationRoot, base, sealed, request.StateRoot, request.ParticipationRoot, request.SpendRoot, request.HostingRoot, request.IntroductionRoot, request.CredentialDirectory} {
		mode, uid := uint32(0755), uint32(0)
		if p == request.StateRoot || p == request.ParticipationRoot || p == request.SpendRoot || p == request.HostingRoot || p == request.IntroductionRoot {
			mode, uid = 0700, 999
		}
		add(initialRecord{Phase: "directory-intent", Path: p, Mode: mode, UID: uid, GID: uid})
		r := native(initialRecord{Phase: "directory-born", Path: p, Mode: 0700})
		add(r)
		r.Phase = "directory-ready"
		r.Mode = mode
		r.UID = uid
		r.GID = uid
		add(r)
	}
	write(filepath.Join(request.CredentialDirectory, "certificate.pem"), 0444, 0, 0)
	write(filepath.Join(request.CredentialDirectory, "private-key.pem"), 0640, 0, 999)
	for _, name := range names {
		mode := uint32(0444)
		if name == "ardents-node-linux-amd64" {
			mode = 0555
		}
		write(filepath.Join(sealed, name), mode, 0, 0)
	}
	var directory initialRecord
	for _, r := range records {
		if r.Path == sealed && r.Phase == "directory-ready" {
			directory = r
		}
	}
	directory.Mode = 0555
	directory.SHA256 = b.Generation
	for _, phase := range []string{"seal-intent", "sealed"} {
		directory.Phase = phase
		add(directory)
	}
	write(selection, 0644, 0, 0)
	add(initialRecord{Phase: "selected", Path: selection, SHA256: boundSHA, Mode: 0644})
	write(unitPath, 0644, 0, 0)
	add(initialRecord{Phase: "reload-intent", Path: unitPath, SHA256: b.UnitSHA256, Mode: 0644})
	add(initialRecord{Phase: "stopped-verified", Path: selection, SHA256: boundSHA, Mode: 0644})
	add(initialRecord{Phase: "completed", Path: journal, SHA256: boundSHA, Mode: 0600})
	return b, hashes, records
}
func encodeInitialOracle(t *testing.T, records []initialRecord) []byte {
	t.Helper()
	var raw []byte
	for _, r := range records {
		line, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, line...)
		raw = append(raw, '\n')
	}
	return raw
}
func TestIntroductionInitialTranscriptRequiresWholeIndependentOracle(t *testing.T) {
	b, hashes, records := initialTranscriptOracle(t)
	raw := encodeInitialOracle(t, records)
	if len(records) != 123 || len(hashes) != 17 {
		t.Fatal("oracle closure differs", len(records), len(hashes))
	}
	owned, err := decodeInitialRecords(raw, b, hashes)
	if err != nil || len(owned) != 28 {
		t.Fatal("independent complete oracle refused", len(owned), err)
	}
	for i := range records {
		changed := append([]initialRecord(nil), records...)
		changed[i].Phase = "foreign-effect"
		if _, err := decodeInitialRecords(encodeInitialOracle(t, changed), b, hashes); err == nil {
			t.Fatal("foreign phase accepted", i)
		}
		if _, err := decodeInitialRecords(encodeInitialOracle(t, records[:i]), b, hashes); err == nil {
			t.Fatal("incomplete prefix accepted", i)
		}
	}
	for _, bad := range [][]byte{append(bytes.Clone(raw), '\n'), raw[:len(raw)-1], bytes.Replace(raw, []byte("introduction-initial-v1"), []byte("descriptor-initial-v1"), 1)} {
		if _, err := decodeInitialRecords(bad, b, hashes); err == nil {
			t.Fatal("foreign/noncanonical transcript accepted")
		}
	}
	for path, sum := range hashes {
		hashes[path] = strings.Repeat("0", 64)
		if _, err := decodeInitialRecords(raw, b, hashes); err == nil {
			t.Fatal("changed commitment accepted", path)
		}
		hashes[path] = sum
	}
}
func TestIntroductionInitialEmptyAuthorityAndCompletedCustodyRefuse(t *testing.T) {
	if InstallInitialStopped(nil, releaseartifact.IntroductionAuthorization{}, Inputs{}, "/distribution", "/history") == nil {
		t.Fatal("zero authority created installation")
	}
	if _, err := ReadCompleted(nil, "/binding"); err == nil {
		t.Fatal("nil caller admitted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := ReadCompleted(ctx, "/binding"); err == nil {
		t.Fatal("ended caller admitted")
	}
	h := Completed{}
	if h.Check() == nil || h.CheckAuthorization(nil, releaseartifact.IntroductionAuthorization{}) == nil || h.IndependentPaths("/distribution", "/history") == nil {
		t.Fatal("empty completed authority accepted")
	}
	if h.Close() != nil {
		t.Fatal("empty close failed")
	}
}
