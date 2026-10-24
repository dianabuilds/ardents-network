package infrastructure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
)

// An independent byte oracle: numeric identities describe transcript fields,
// never native birth, original helper completion, Release or runtime authority.
func sourceInitialRecordVector(t *testing.T, recoverable bool) ([]byte, sourceBinding, map[string]string) {
	t.Helper()
	binding := sourceBindingVector()
	b, err := decodeSourceBinding(binding)
	if err != nil {
		t.Fatal(err)
	}
	root := "/var/lib/install"
	sealed := root + "/generations/" + strings.Repeat("a", 64)
	hashes := map[string]string{root + "/binding.json": digest(binding), "/etc/systemd/system/ardents-source.service": b.UnitSHA256}
	var lines []string
	add := func(phase, path string, inode uint64, sha string, mode, uid, gid uint32) {
		device := uint64(0)
		if inode != 0 {
			device = 1
		}
		lines = append(lines, fmt.Sprintf(`{"sequence":%d,"phase":%q,"path":%q,"device":%d,"inode":%d,"sha256":%q,"mode":%d,"uid":%d,"gid":%d}`, len(lines), phase, path, device, inode, sha, mode, uid, gid))
	}
	phase := "initial"
	if recoverable {
		phase = "initial-recoverable"
	}
	add(phase, root+".source-initial-journal", 1, "", 0600, 0, 0)
	if recoverable {
		sha := digest([]byte("ardents-source-initial-pending-v1\n" + digest(binding) + "\n"))
		add("file-intent", root+".source-initial-pending", 0, sha, 0600, 0, 0)
		add("file-born", root+".source-initial-pending", 500000, sha, 0600, 0, 0)
		add("file-ready", root+".source-initial-pending", 500000, sha, 0600, 0, 0)
	}
	add("admitted", root, 0, strings.Repeat("9", 64), 0, 999, 999)
	databases := []string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow"}
	preimages := map[string]string{}
	for i, path := range databases {
		preimages[path] = strings.Repeat(fmt.Sprint(i+1), 64)
		add("account-preimage", path, uint64(i+2), preimages[path], 0600, 0, 0)
	}
	auxiliary := []string{"/etc/.pwd.lock", "/etc/passwd-", "/etc/group-", "/etc/shadow-", "/etc/gshadow-", "/etc/passwd.lock", "/etc/group.lock", "/etc/shadow.lock", "/etc/gshadow.lock", "/etc/passwd.1", "/etc/group.1", "/etc/shadow.1", "/etc/gshadow.1"}
	for _, path := range auxiliary {
		add("account-auxiliary-preimage", path, 0, "", 0, 0, 0)
	}
	add("account-intent", "ardents-source", 0, "", 0, 999, 999)
	for i, path := range databases {
		add("account-born", path, uint64(i+6), strings.Repeat("5", 64), 0600, 0, 0)
	}
	for i, path := range auxiliary {
		if i == 0 {
			add("account-auxiliary-result", path, 10, digest(nil), 0600, 0, 0)
		} else if i < 5 {
			add("account-auxiliary-result", path, uint64(10+i), preimages[strings.TrimSuffix(path, "-")], 0600, 0, 0)
		} else {
			add("account-auxiliary-cleared", path, 0, "", 0, 0, 0)
		}
	}
	add("account-ready", "ardents-source", 0, "", 0, 999, 999)
	for i, path := range []string{root, root + "/generations", sealed, b.Request.StateRoot, b.Request.ParticipationRoot, b.Request.CredentialDirectory} {
		mode, uid, gid := uint32(0755), uint32(0), uint32(0)
		if i == 3 || i == 4 {
			mode, uid, gid = 0700, 999, 999
		}
		add("directory-intent", path, 0, "", mode, uid, gid)
		add("directory-born", path, uint64(15+i), "", mode, uid, gid)
		add("directory-ready", path, uint64(15+i), "", mode, uid, gid)
	}
	write := func(path string, inode uint64, sha string, mode, uid, gid uint32) {
		hashes[path] = sha
		add("file-intent", path, 0, sha, mode, uid, gid)
		add("file-born", path, inode, sha, mode, uid, gid)
		add("file-ready", path, inode, sha, mode, uid, gid)
	}
	for i, name := range []string{"certificate.pem", "private-key.pem", "client-ca.pem"} {
		mode, uid, gid := uint32(0444), uint32(0), uint32(0)
		if i == 1 {
			mode, uid, gid = 0600, 999, 999
		}
		write(filepath.Join(b.Request.CredentialDirectory, name), uint64(21+i), strings.Repeat("6", 64), mode, uid, gid)
	}
	for i, name := range []string{"ardents-node-linux-amd64", "ardents-source.service", "binding.json", "infrastructure-enrollment.json", "infrastructure-source.json", "rendered-source.service", "root.json", "snapshot.json", "source-plan.json", "source-request.json", "targets.json", "timestamp.json"} {
		mode := uint32(0444)
		if i == 0 {
			mode = 0555
		}
		write(filepath.Join(sealed, name), uint64(24+i), strings.Repeat("7", 64), mode, 0, 0)
	}
	add("seal-intent", sealed, 17, b.Generation, 0555, 0, 0)
	add("sealed", sealed, 17, b.Generation, 0555, 0, 0)
	write(root+"/binding.json", 36, digest(binding), 0644, 0, 0)
	add("selected", root+"/binding.json", 0, digest(binding), 0644, 0, 0)
	write("/etc/systemd/system/ardents-source.service", 37, b.UnitSHA256, 0644, 0, 0)
	add("reload-intent", "/etc/systemd/system/ardents-source.service", 0, b.UnitSHA256, 0644, 0, 0)
	add("stopped-verified", root+"/binding.json", 0, digest(binding), 0644, 0, 0)
	expected := 112
	if recoverable {
		expected = 115
	}
	if len(lines) != expected {
		t.Fatal("independent initial oracle record count", len(lines))
	}
	return []byte(strings.Join(lines, "\n") + "\n"), b, hashes
}

func TestSourceRecoveryInitialBirthVersionsStayCompleteOnly(t *testing.T) {
	legacy, b, hashes := sourceInitialRecordVector(t, false)
	current, _, currentHashes := sourceInitialRecordVector(t, true)
	oldFacts, err := decodeSourceInitialBirthRecords(legacy, b, hashes, strings.Repeat("9", 64))
	if err != nil {
		t.Fatal("legacy complete grammar", err)
	}
	facts, err := decodeSourceInitialBirthRecords(current, b, currentHashes, strings.Repeat("9", 64))
	if err != nil || !reflect.DeepEqual(facts, oldFacts) {
		t.Fatal("new initial format changed original resource facts", err)
	}
	if _, exists := facts["/var/lib/install.source-initial-pending"]; exists {
		t.Fatal("completed reader requires a removed marker as a live resource")
	}
	if _, err := decodeSourceInitialRecords(current, b, currentHashes, strings.Repeat("9", 64)); err == nil {
		t.Fatal("legacy complete-only grammar was widened")
	}
	lines := bytes.SplitAfter(current, []byte{'\n'})
	for cut := 1; cut < 115; cut++ {
		if _, err := decodeSourceInitialBirthRecords(bytes.Join(lines[:cut], nil), b, currentHashes, strings.Repeat("9", 64)); err == nil {
			t.Fatal("partial new initial prefix accepted by ordinary reader", cut)
		}
	}
}

func TestSourceRecoveryInitialPendingGrammarRejectsUnprovenBirth(t *testing.T) {
	full, b, hashes := sourceInitialRecordVector(t, true)
	for _, change := range []struct{ from, to string }{
		{`"phase":"initial-recoverable"`, `"phase":"initial"`},
		{`"inode":500000`, `"inode":0`},
		{`"sequence":3,"phase":"file-ready"`, `"sequence":3,"phase":"file-born"`},
		{`/var/lib/install.source-initial-pending`, `/var/lib/copied.source-initial-pending`},
		{`"sequence":1,`, `"sequence":2,`},
	} {
		changed := bytes.Replace(full, []byte(change.from), []byte(change.to), 1)
		if bytes.Equal(changed, full) {
			t.Fatal("inactive grammar control", change.from)
		}
		if _, err := decodeSourceInitialBirthRecords(changed, b, hashes, strings.Repeat("9", 64)); err == nil {
			t.Fatal("unproven initial marker accepted", change.from)
		}
	}
}

func TestSourceRecoveryInitialCompletionKeepsAll115Boundaries(t *testing.T) {
	full, b, hashes := sourceInitialRecordVector(t, true)
	selected := sourceBindingVector()
	records, err := decodeSourceRecoveryPrefix(full, 115)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.SplitAfter(full, []byte{'\n'})
	// Explicit independent allowed-count oracle. Full codec/semantic completion
	// below is intentionally possible even when continuation is unsupported.
	supported := map[int]bool{}
	for _, count := range []int{68, 70, 71, 73, 74, 76, 77, 79, 80, 82, 83, 85, 86, 88, 89, 91, 92, 94, 95, 97, 98, 100, 101, 103, 104, 105, 106, 108, 109, 110, 113, 114, 115} {
		supported[count] = true
	}
	if sourceInitialRecoveryPrefixSupported(nil) {
		t.Fatal("empty initial prefix supported")
	}
	for cut := 1; cut <= 115; cut++ {
		prefix := bytes.Join(lines[:cut], nil)
		completion := sourceRecoveryCompletion{Schema: "ardents-source-recovery-completion-v1", Operation: "initial", Journal: records[0].Path,
			Device: 1, Inode: 1, PrefixSHA256: digest(prefix), PrefixRecords: cut, Generation: b.Generation,
			BindingSHA256: digest(selected), Records: append(make(sourceRecoveryRecords, 0), records[cut:]...), TranscriptSHA256: digest(full)}
		info := sourceRecoveryCodecInfo{size: int64(len(prefix)), mode: 0600, stat: syscall.Stat_t{Dev: 1, Ino: 1, Nlink: 1}}
		transcript, err := decodeSourceRecoveryCompletion(marshalSourceRecoveryCodec(t, completion), prefix, info, completion.Journal, "initial", b.Generation, digest(selected))
		if err != nil {
			t.Fatalf("initial boundary%d: %v", cut, err)
		}
		if _, err := decodeSourceRecoveredRecords(transcript, selected, "initial", hashes, strings.Repeat("9", 64), ""); err != nil {
			t.Fatalf("initial full grammar boundary%d: %v", cut, err)
		}
		if sourceInitialRecoveryPrefixSupported(records[:cut]) != supported[cut] {
			t.Fatalf("full suffix widened initial continuation at prefix%d", cut)
		}
	}
}

// Keep the oracle's exact record fields independently visible when a regression
// needs a malformed marker. JSON decoding is not used to construct its bytes.
func TestSourceRecoveryInitialPendingReadyRetainsBornIdentity(t *testing.T) {
	full, b, hashes := sourceInitialRecordVector(t, true)
	lines := bytes.Split(full[:len(full)-1], []byte{'\n'})
	var ready sourceRecord
	if json.Unmarshal(lines[3], &ready) != nil {
		t.Fatal("oracle ready record")
	}
	ready.Inode++
	changed, err := json.Marshal(ready)
	if err != nil {
		t.Fatal(err)
	}
	lines[3] = changed
	if _, err := decodeSourceInitialBirthRecords(append(bytes.Join(lines, []byte{'\n'}), '\n'), b, hashes, strings.Repeat("9", 64)); err == nil {
		t.Fatal("marker ready record replaced original born inode")
	}
}
