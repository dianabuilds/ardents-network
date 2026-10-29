package forwarding

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Independent complete byte inventory with invented native identities. It is
// only a grammar/classification oracle, never a successful recovery authority.
func recoveryInitialVector(t *testing.T) ([]initialRecord, *recoveryCandidate, transitionCutResult) {
	t.Helper()
	raw := forwardingBindingVector()
	selected, err := decodeSelection(raw)
	if err != nil {
		t.Fatal(err)
	}
	b := selected.binding
	root := b.Request.InstallationRoot
	sealed := filepath.Join(root, "generations", b.Generation)
	journal := root + ".forwarding-initial-journal"
	hashes := map[string]string{}
	for name, sha := range map[string]string{
		"ardents-node-linux-amd64": b.ProgramSHA256, "ardents-forwarding.service": b.TemplateSHA256,
		"infrastructure-forwarding.json": b.Generation, "infrastructure-enrollment.json": b.InventorySHA256,
		"targets.json": b.TargetsSHA256, "forwarding-request.json": b.RequestSHA256, "forwarding-plan.json": b.PlanSHA256,
		"rendered-forwarding.service": b.UnitSHA256, "binding.json": digest(raw),
		"root.json": digest([]byte("root")), "timestamp.json": digest([]byte("timestamp")), "snapshot.json": digest([]byte("snapshot")),
	} {
		hashes[filepath.Join(sealed, name)] = sha
	}
	hashes[filepath.Join(root, "binding.json")] = digest(raw)
	hashes["/etc/systemd/system/ardents-forwarding.service"] = b.UnitSHA256
	hashes[filepath.Join(b.Request.CredentialDirectory, "certificate.pem")] = b.CertificateSHA256
	hashes[filepath.Join(b.Request.CredentialDirectory, "private-key.pem")] = digest([]byte("private fixture hash"))
	hashes[root+".forwarding-initial-pending"] = digest([]byte("ardents-forwarding-initial-pending-v1\n" + digest(raw) + "\n"))
	var rows []initialRecord
	ino := uint64(100)
	add := func(row initialRecord) { row.Sequence = len(rows); rows = append(rows, row) }
	native := func(phase, path, sha string, mode, uid, gid uint32) initialRecord {
		ino++
		return initialRecord{Phase: phase, Path: path, SHA256: sha, Device: 1, Inode: ino, Mode: mode, UID: uid, GID: gid}
	}
	write := func(path string, mode, uid, gid uint32) {
		add(initialRecord{Phase: "file-intent", Path: path, SHA256: hashes[path], Mode: mode, UID: uid, GID: gid})
		r := native("file-born", path, "", 0600, 0, 0)
		add(r)
		r.Phase, r.SHA256, r.Mode, r.UID, r.GID = "file-ready", hashes[path], mode, uid, gid
		add(r)
	}
	add(native("forwarding-initial-v2", journal, "", 0600, 0, 0))
	context := nativeContextByteVector()
	add(initialRecord{Phase: "native-context", Path: root, SHA256: digest(context)})
	write(root+".forwarding-initial-pending", 0600, 0, 0)
	add(native("admitted", b.History, digest(raw), 0700, 0, 0))
	accounts := []string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow"}
	aux := []string{"/etc/.pwd.lock", "/etc/passwd-", "/etc/group-", "/etc/shadow-", "/etc/gshadow-", "/etc/passwd.lock", "/etc/group.lock", "/etc/shadow.lock", "/etc/gshadow.lock", "/etc/passwd.1", "/etc/group.1", "/etc/shadow.1", "/etc/gshadow.1"}
	for _, path := range accounts {
		add(native("account-preimage", path, digest([]byte(path)), 0600, 0, 0))
	}
	for _, path := range aux {
		add(initialRecord{Phase: "account-auxiliary-preimage", Path: path})
	}
	add(initialRecord{Phase: "account-intent", Path: "ardents-node", UID: b.UID, GID: b.GID})
	for _, path := range accounts {
		add(native("account-born", path, digest([]byte("new"+path)), 0600, 0, 0))
	}
	for _, path := range aux {
		if path == "/etc/.pwd.lock" {
			add(native("account-auxiliary-result", path, digest(nil), 0600, 0, 0))
		} else if strings.HasSuffix(path, "-") {
			add(native("account-auxiliary-result", path, digest([]byte(strings.TrimSuffix(path, "-"))), 0600, 0, 0))
		} else {
			add(initialRecord{Phase: "account-auxiliary-cleared", Path: path})
		}
	}
	add(initialRecord{Phase: "account-ready", Path: "ardents-node", UID: b.UID, GID: b.GID})
	var sealedBirth initialRecord
	for _, path := range []string{root, filepath.Dir(sealed), sealed, b.Request.StateRoot, b.Request.ParticipationRoot, b.Request.SpendRoot, b.Request.HostingRoot, b.Request.CredentialDirectory} {
		mode, uid, gid := uint32(0755), uint32(0), uint32(0)
		if path == b.Request.StateRoot || path == b.Request.ParticipationRoot || path == b.Request.SpendRoot || path == b.Request.HostingRoot {
			mode, uid, gid = 0700, b.UID, b.GID
		}
		add(initialRecord{Phase: "directory-intent", Path: path, Mode: mode, UID: uid, GID: gid})
		r := native("directory-born", path, "", 0700, 0, 0)
		add(r)
		r.Phase, r.Mode, r.UID, r.GID = "directory-ready", mode, uid, gid
		add(r)
		if path == sealed {
			sealedBirth = r
		}
	}
	write(filepath.Join(b.Request.CredentialDirectory, "certificate.pem"), 0444, 0, 0)
	write(filepath.Join(b.Request.CredentialDirectory, "private-key.pem"), 0640, 0, b.GID)
	names := []string{"ardents-node-linux-amd64", "ardents-forwarding.service", "binding.json", "forwarding-plan.json", "forwarding-request.json", "infrastructure-enrollment.json", "infrastructure-forwarding.json", "rendered-forwarding.service", "root.json", "snapshot.json", "targets.json", "timestamp.json"}
	slices.Sort(names)
	for _, name := range names {
		mode := uint32(0444)
		if name == "ardents-node-linux-amd64" {
			mode = 0555
		}
		write(filepath.Join(sealed, name), mode, 0, 0)
	}
	sealedBirth.Mode, sealedBirth.SHA256 = 0555, b.Generation
	for _, phase := range []string{"seal-intent", "sealed"} {
		sealedBirth.Phase = phase
		add(sealedBirth)
	}
	write(filepath.Join(root, "binding.json"), 0644, 0, 0)
	add(initialRecord{Phase: "selected", Path: filepath.Join(root, "binding.json"), SHA256: digest(raw), Mode: 0644})
	write("/etc/systemd/system/ardents-forwarding.service", 0644, 0, 0)
	add(initialRecord{Phase: "reload-intent", Path: "/etc/systemd/system/ardents-forwarding.service", SHA256: b.UnitSHA256, Mode: 0644})
	add(initialRecord{Phase: "stopped-verified", Path: filepath.Join(root, "binding.json"), SHA256: digest(raw), Mode: 0644})
	add(initialRecord{Phase: "completed", Path: journal, SHA256: digest(raw), Mode: 0600})
	if len(rows) != 120 {
		t.Fatal("independent full grammar", len(rows))
	}
	full := encodeReplacementVector(t, rows)
	if _, err := decodeInitialRecords(full, b, hashes); err != nil {
		t.Fatal("independent complete initial grammar", err)
	}
	cut := transitionCutResult{Boundary: transitionMessage{Operation: "initial", Journal: journal, Device: 1, Inode: rows[0].Inode, Generation: b.Generation, Binding: raw, BindingSHA256: digest(raw), OriginalContext: context, NativeContext: context, Manager: "physical-test-only"}, Origin: transitionOrigin{ProgramSHA256: b.ProgramSHA256}}
	return rows, &recoveryCandidate{selection: selected, hashes: hashes}, cut
}

func TestForwardingRecoveryInitialClassifiesOnlyOriginalStoppedBoundaries(t *testing.T) {
	rows, candidate, cut := recoveryInitialVector(t)
	for _, count := range []int{119, 120} {
		prefix := encodeReplacementVector(t, rows[:count])
		cut.Boundary.Records = uint32(count)
		cut.Boundary.PrefixSHA256 = digest(prefix)
		original := &recoveryOriginal{prefix: &transitionPrefix{journal: &initialFile{path: cut.Boundary.Journal, body: prefix}}, result: &initialFile{body: []byte("non-authorizing original-cut fixture\n")}, cut: cut}
		envelope, full, err := stoppedInitialCompletion(original, candidate)
		if err != nil || !bytes.Equal(full, encodeReplacementVector(t, rows)) || !bytes.Equal(original.prefix.journal.body, prefix) {
			t.Fatal("stopped logical continuation changed original", count, err)
		}
		decoded, _, err := decodeRecoveryCompletion(envelope, prefix, original.result.body, cut)
		if err != nil || len(decoded.Records) != 120-count {
			t.Fatal("separate completion", err)
		}
	}
	for count := 1; count < 119; count++ {
		if _, err := initialStoppedPrefix(encodeReplacementVector(t, rows[:count]), cut, candidate.selection); err == nil {
			t.Fatal("unproved stopped prefix", count)
		}
	}
	for _, fault := range []string{"context", "manager", "program", "boundary", "native-manager", "grammar"} {
		bad := append([]initialRecord(nil), rows...)
		changed := cut
		switch fault {
		case "context":
			bad[1].SHA256 = digest([]byte("foreign context"))
		case "manager":
			changed.Boundary.Manager = "foreign manager"
		case "program":
			changed.Origin.ProgramSHA256 = digest([]byte("another program"))
		case "boundary":
			bad[118].Phase = "reload-intent"
		case "native-manager":
			changed.Boundary.NativeContext = bytes.Replace(cut.Boundary.NativeContext, []byte("physical-test-only"), []byte("different-manager"), 1)
		case "grammar":
			bad[110].Mode ^= 1
		}
		prefix := encodeReplacementVector(t, bad)
		changed.Boundary.Records = 120
		changed.Boundary.PrefixSHA256 = digest(prefix)
		original := &recoveryOriginal{prefix: &transitionPrefix{journal: &initialFile{path: cut.Boundary.Journal, body: prefix}}, result: &initialFile{body: []byte("cut fixture")}, cut: changed}
		if _, _, err := stoppedInitialCompletion(original, candidate); err == nil {
			t.Fatal("foreign/uncertain completion", fault)
		}
	}
}
