package forwarding

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Literal persisted phases and invented identities exercise classification
// only. No original process cut, native authority or successful recovery is
// supplied by this vector.
func neverStartedVector(t *testing.T) (generationSelection, transitionCutResult, []startEntry) {
	t.Helper()
	b, raw, _ := selectedGenerationVector(t)
	selected := generationSelection{binding: b, raw: raw}
	intent, err := encodeHistoryStartIntent(b, digest(raw), digest([]byte("initial")), digest([]byte("original targets")), 0, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	native := nativeContextByteVector()
	context, err := decodeNativeContext(native)
	if err != nil {
		t.Fatal(err)
	}
	cut := transitionCutResult{Boundary: transitionMessage{Schema: "ardents-forwarding-transition-boundary-v2", Operation: "start", Journal: attemptNumber(0).journal(b.Request.InstallationRoot), Generation: b.Generation, BindingSHA256: digest(raw), Binding: raw, Manager: context.Manager, OriginalContext: native, NativeContext: native, StartIntent: intent}, Origin: transitionOrigin{ProgramSHA256: b.ProgramSHA256}}
	dir := attemptNumber(0).directory(b.Request.InstallationRoot)
	socket := filepath.Join(dir, "start-completion.socket")
	socketRaw, err := json.Marshal(startSocketRecord{"ardents-forwarding-start-socket-v1", digest(intent), 5, 14, b.GID})
	if err != nil {
		t.Fatal(err)
	}
	rows := []startEntry{
		{Phase: "first-start", Path: cut.Boundary.Journal, Device: 5, Inode: 10, SHA256: digest(raw), Mode: 0600},
		{Phase: "native-context", Path: b.Request.InstallationRoot, SHA256: digest(native)},
		{Phase: "directory-intent", Path: dir}, {Phase: "directory-born", Path: dir, Device: 5, Inode: 11, Mode: 0700}, {Phase: "directory-ready", Path: dir, Device: 5, Inode: 11, Mode: 0750, GID: b.GID},
	}
	file := func(name string, body []byte, inode uint64) {
		path := filepath.Join(dir, name)
		rows = append(rows, startEntry{Phase: "file-intent", Path: path, SHA256: digest(body)}, startEntry{Phase: "file-born", Path: path, Device: 5, Inode: inode, SHA256: digest(body), Mode: 0600}, startEntry{Phase: "file-ready", Path: path, Device: 5, Inode: inode, SHA256: digest(body), Mode: 0600})
	}
	file("forwarding-start-pending", []byte("ardents-forwarding-start-pending-v1\n"+digest(raw)+"\n"), 12)
	file("start-guard.json", intent, 13)
	rows = append(rows, startEntry{Phase: "socket-intent", Path: socket}, startEntry{Phase: "socket-born", Path: socket, Device: 5, Inode: 14, Mode: 0755}, startEntry{Phase: "socket-ready", Path: socket, Device: 5, Inode: 14, Mode: 0660, GID: b.GID})
	file("start-socket.json", append(socketRaw, '\n'), 15)
	rows = append(rows, startEntry{Phase: "start-attempt", Path: dir, SHA256: digest(intent)}, startEntry{Phase: "invocation-observed", Path: dir, SHA256: digest(intent)})
	return selected, cut, rows
}

// Synthetic native values are codec inputs only, never independent file or
// original-cut custody. Actual native proof belongs to the recovery consumer.
type startDirectoryVector struct {
	mode  os.FileMode
	gid   uint32
	inode uint64
}

func (i startDirectoryVector) Name() string       { return "codec-directory" }
func (i startDirectoryVector) Size() int64        { return 4096 }
func (i startDirectoryVector) Mode() os.FileMode  { return os.ModeDir | i.mode }
func (i startDirectoryVector) ModTime() time.Time { return time.Time{} }
func (i startDirectoryVector) IsDir() bool        { return true }
func (i startDirectoryVector) Sys() any           { return &syscall.Stat_t{Dev: 5, Ino: i.inode, Gid: i.gid} }

func TestForwardingNeverStartedCompletionRetainsOriginalPrefix(t *testing.T) {
	selected, cut, rows := neverStartedVector(t)
	for count := 1; count <= 18; count++ {
		prefix := encodeNeverStartedVector(t, rows[:count])
		cut.Boundary.Records = uint32(count)
		cut.Boundary.Device = 5
		cut.Boundary.Inode = 10
		cut.Boundary.PrefixSHA256 = digest(prefix)
		classified, err := classifyNeverStarted(prefix, cut, selected, nil)
		if err != nil {
			t.Fatal(err)
		}
		original := &recoveryOriginal{cut: cut, prefix: &transitionPrefix{journal: &initialFile{body: bytes.Clone(prefix)}}, result: &initialFile{body: []byte("detached cut codec vector\n")}}
		h := &startTerminalSnapshot{prefix: classified, original: original, selection: selected, path: attemptNumber(0).directory(selected.binding.Request.InstallationRoot)}
		directory := startDirectoryVector{mode: 0750, gid: selected.binding.GID, inode: 41}
		wantRecords := 4
		if born, ok := classified.owned[h.path]; ok {
			directory.mode = os.FileMode(born.Mode)
			directory.gid = born.GID
			directory.inode = born.Inode
			wantRecords = 1
		}
		raw, err := stoppedStartCompletion(original, h, directory)
		if err != nil {
			t.Fatal(count, err)
		}
		envelope, full, err := decodeRecoveryCompletion(raw, prefix, original.result.body, cut)
		if err != nil || len(envelope.Records) != wantRecords || envelope.Outcome != "stopped-before-start" || !bytes.HasPrefix(full, prefix) || !bytes.Equal(original.prefix.journal.body, prefix) {
			t.Fatal("prefix or terminal outcome changed", count, err)
		}
		var final startEntry
		if json.Unmarshal(envelope.Records[len(envelope.Records)-1], &final) != nil || final.Phase != "stopped-before-start" || final.Sequence != uint32(count+wantRecords-1) || final.SHA256 != digest(cut.Boundary.StartIntent) || final.Inode != directory.inode {
			t.Fatal("terminal commitment", count)
		}
		for _, phase := range []string{"archived", "ack-written", "retired", "invocation-observed"} {
			bad := envelope
			bad.Records = append([]json.RawMessage(nil), envelope.Records...)
			changed := final
			changed.Phase = phase
			bad.Records[len(bad.Records)-1], _ = json.Marshal(changed)
			logical := bytes.Clone(prefix)
			for _, row := range bad.Records {
				logical = append(logical, row...)
				logical = append(logical, '\n')
			}
			bad.TranscriptSHA256 = digest(logical)
			encoded, _ := json.Marshal(bad)
			if _, _, err := decodeRecoveryCompletion(append(encoded, '\n'), prefix, original.result.body, cut); err == nil {
				t.Fatal("recovery invented runtime completion", count, phase)
			}
		}
		classified.clear()
	}
}

func TestForwardingNeverStartedSpendObservationNeverCreatesHistory(t *testing.T) {
	for _, fault := range []string{"empty", "nonempty", "replaced"} {
		t.Run(fault, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "spend")
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			info, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			s := &replacementInspections{emptySpend: file, emptySpendInfo: info, emptySpendPath: path}
			if fault == "nonempty" {
				if err := os.WriteFile(filepath.Join(path, "unknown-history"), []byte("preserve\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if fault == "replaced" {
				if err := os.Rename(path, path+"-original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			err = s.checkEmptySpend()
			if (err == nil) != (fault == "empty") {
				t.Fatal("empty/native identity boundary", err)
			}
			entries, err := os.ReadDir(path)
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if fault == "nonempty" {
				want = 1
			}
			if len(entries) != want {
				t.Fatal("inspection created or removed history")
			}
		})
	}
	if publishNeverStartedAbsentPending(&recoveryPublication{}) == nil {
		t.Fatal("nil pending waived publication")
	}
}

func encodeNeverStartedVector(t *testing.T, rows []startEntry) []byte {
	t.Helper()
	var raw []byte
	for n, row := range rows {
		row.Schema = "ardents-forwarding-start-journal-v1"
		row.Sequence = uint32(n)
		body, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, body...)
		raw = append(raw, '\n')
	}
	return raw
}

func TestForwardingNeverStartedPrefixStopsBeforeOriginalStart(t *testing.T) {
	selected, cut, rows := neverStartedVector(t)
	for count := 1; count <= 18; count++ {
		prefix := encodeNeverStartedVector(t, rows[:count])
		read, err := classifyNeverStarted(prefix, cut, selected, nil)
		if err != nil || !bytes.Equal(read.intent, cut.Boundary.StartIntent) {
			t.Fatal("safe original byte boundary", count, err)
		}
		read.clear()
	}
	if _, err := classifyNeverStarted(encodeNeverStartedVector(t, rows), cut, selected, nil); err == nil {
		t.Fatal("invocation observation admitted as never started")
	}
	for n := range rows[:18] {
		bad := append([]startEntry(nil), rows[:18]...)
		bad[n].Path += "-foreign"
		if _, err := classifyNeverStarted(encodeNeverStartedVector(t, bad), cut, selected, nil); err == nil {
			t.Fatal("foreign original path", n)
		}
	}
	for _, index := range []int{4, 7, 10, 13, 16} {
		bad := append([]startEntry(nil), rows[:18]...)
		bad[index].Inode++
		if _, err := classifyNeverStarted(encodeNeverStartedVector(t, bad), cut, selected, nil); err == nil {
			t.Fatal("replaced original inode", index)
		}
	}
	legacy := cut
	legacy.Boundary.Schema = "ardents-forwarding-transition-boundary-v1"
	legacy.Boundary.StartIntent = nil
	for count := 1; count <= 18; count++ {
		_, err := classifyNeverStarted(encodeNeverStartedVector(t, rows[:count]), legacy, selected, cut.Boundary.StartIntent)
		if (err == nil) != (count >= 11) {
			t.Fatal("legacy guard preimage boundary", count, err)
		}
	}
}

func TestForwardingStartRecoverySidecarsCannotFallBack(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "journal")
	if present, err := startRecoveryPresent(journal); err != nil || present {
		t.Fatal(present, err)
	}
	a, b, c := transitionCutPaths(journal)
	d, e, f := recoveryPaths(journal)
	for _, path := range []string{a, b, c, d, e, f, journal + ".recovery-native-context"} {
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
		if present, err := startRecoveryPresent(journal); err != nil || !present {
			t.Fatal("partial proof fell back", path, err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
}
