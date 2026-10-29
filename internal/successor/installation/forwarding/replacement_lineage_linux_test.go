package forwarding

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"
)

// Invented native identities test closed bytes and lineage relations only.
// No actual reload, native custody, Release or installed authority is supplied.
func replacementContextVector(t *testing.T) ([]initialRecord, generationSelection, generationSelection, map[string]string, []byte, []byte) {
	t.Helper()
	old, selected, previous, hashes := replacementRecordsVector(t)
	before := nativeContextByteVector()
	after := bytes.Replace(before, []byte(`"inode":101,`), []byte(`"inode":102,`), 1)
	journal := old[0].Path
	trip := func(path string, raw []byte, inode uint64) []initialRecord {
		return []initialRecord{
			{Phase: "file-intent", Path: path, SHA256: digest(raw), Mode: 0600},
			{Phase: "file-born", Path: path, Device: 1, Inode: inode, Mode: 0600},
			{Phase: "file-ready", Path: path, Device: 1, Inode: inode, SHA256: digest(raw), Mode: 0600},
		}
	}
	// Literal v2 placements: after the original pending triple, and after the
	// original reload intent but before replacement verification.
	rows := append([]initialRecord(nil), old[:9]...)
	rows = append(rows, trip(journal+".native-before", before, 1000)...)
	rows = append(rows, old[9:55]...)
	rows = append(rows, trip(journal+".native-after", after, 1001)...)
	rows = append(rows, old[55])
	rows[0].Phase = "forwarding-replacement-v2"
	rows[2].SHA256 = digest(before)
	for n := range rows {
		rows[n].Sequence = n
	}
	return rows, selected, previous, hashes, before, after
}

func TestForwardingReplacementContextRecordsRequireBothOriginalSnapshotBirths(t *testing.T) {
	rows, selected, previous, hashes, before, after := replacementContextVector(t)
	raw := encodeReplacementVector(t, rows)
	owned, err := decodeReplacementContextRecords(raw, selected, previous, hashes, before, after)
	if err != nil || len(rows) != 62 || len(owned) != 21 {
		t.Fatal("independent v2 complete grammar", len(rows), len(owned), err)
	}
	if _, err := decodeReplacementContextRecords(raw, selected, previous, hashes, nil, nil); err == nil {
		t.Fatal("missing snapshot bytes supplied v2 proof")
	}
	for n := 0; n < len(rows); n++ {
		if _, err := decodeReplacementContextRecords(encodeReplacementVector(t, rows[:n]), selected, previous, hashes, before, after); err == nil {
			t.Fatal("partial context publication", n)
		}
	}
	for _, fault := range []string{"before-hash", "after-hash", "path", "alias", "ready-inode", "mode", "phase", "manager"} {
		bad := append([]initialRecord(nil), rows...)
		a, b := before, after
		switch fault {
		case "before-hash":
			bad[2].SHA256 = digest(after)
		case "after-hash":
			bad[58].SHA256 = digest(before)
		case "path":
			bad[9].Path = filepath.Join(selected.binding.Request.InstallationRoot, "different-context")
		case "alias":
			bad[59].Device, bad[60].Device = bad[10].Device, bad[10].Device
			bad[59].Inode, bad[60].Inode = bad[10].Inode, bad[10].Inode
		case "ready-inode":
			bad[60].Inode++
		case "mode":
			bad[60].Mode = 0644
		case "phase":
			bad[58].Phase = "replacement-verified"
		case "manager":
			b = bytes.Replace(after, []byte("physical-test-only"), []byte("different-manager"), 1)
		}
		if _, err := decodeReplacementContextRecords(encodeReplacementVector(t, bad), selected, previous, hashes, a, b); err == nil {
			t.Fatal("foreign/partial reload proof", fault)
		}
	}
}

func TestForwardingRecoveredContextLineagePreservesEveryOriginalNonGenerator(t *testing.T) {
	rows, selected, previous, _, before, after := replacementContextVector(t)
	proof := &replacementNativeProof{before: &initialFile{path: rows[9].Path, body: before}, after: &initialFile{path: rows[58].Path, body: after}}
	c := &completedInitial{files: &completedFiles{recovered: &recoveredInitial{nativeRaw: before}}, selectionOrder: []generationSelection{selected, previous}, reloads: map[string]*replacementNativeProof{selected.binding.Generation: proof}}
	raw, err := c.recoveredContextBytes()
	if err != nil || !bytes.Equal(raw, after) {
		t.Fatal("checked forward native lineage unavailable", err)
	}
	delete(c.reloads, selected.binding.Generation)
	if _, err := c.recoveredContextBytes(); err == nil {
		t.Fatal("v1 or absent reload evidence supplied current context")
	}
	c.reloads[selected.binding.Generation] = proof
	for _, change := range [][2]string{
		{`"inode":8,`, `"inode":88,`},
		{`"started":1`, `"started":2`},
		{`"physical-test-only"`, `"another-manager"`},
		{`"path":"/run/systemd","device":1,"inode":9`, `"path":"/run/systemd","device":1,"inode":99`},
	} {
		changed := bytes.Replace(before, []byte(change[0]), []byte(change[1]), 1)
		if bytes.Equal(changed, before) {
			t.Fatal("missing causal mutation", change)
		}
		proof.before.body = changed
		proof.after.body = bytes.Replace(changed, []byte(`"inode":101,`), []byte(`"inode":102,`), 1)
		if _, err := c.recoveredContextBytes(); err == nil {
			t.Fatal("internally consistent later snapshots replaced original context", change)
		}
	}
	proof.before.body, proof.after.body = before, after
	var extra nativeContextSnapshot
	if json.Unmarshal(before, &extra) != nil {
		t.Fatal("fixture")
	}
	extra.Ancestors = append(extra.Ancestors, nativeObject{Path: "/var", Device: 1, Inode: 200, Mode: 2147484141})
	added, err := json.Marshal(extra)
	if err != nil {
		t.Fatal(err)
	}
	added = append(added, '\n')
	if err := compareContextContinuation(before, added); err != nil {
		t.Fatal("additional independently observed ancestor unavailable", err)
	}
	if err := compareContextContinuation(added, before); err == nil {
		t.Fatal("earlier retained ancestor dropped")
	}
}

func TestForwardingChangedContextAtV2AfterFileHasExactOriginalIntent(t *testing.T) {
	rows, selected, _, _, before, after := replacementContextVector(t)
	message := transitionMessage{Operation: "replacement", Journal: rows[0].Path, OriginalContext: before, NativeContext: after, Binding: selected.raw}
	for _, count := range []int{59, 60, 61, 62} {
		if err := checkTransitionContexts(message, rows[:count]); err != nil {
			t.Fatal("owned after-reload snapshot boundary", count, err)
		}
	}
	for _, count := range []int{1, 3, 9, 10, 11, 12, 57, 58} {
		if err := checkTransitionContexts(message, rows[:count]); err == nil {
			t.Fatal("context changed before own reload proof", count)
		}
	}
	bad := append([]initialRecord(nil), rows...)
	bad[58].SHA256 = digest(before)
	if checkTransitionContexts(message, bad[:61]) == nil {
		t.Fatal("after snapshot intent did not bind checked current context")
	}
	bad = append([]initialRecord(nil), rows...)
	bad[58].Path += "-foreign"
	if checkTransitionContexts(message, bad[:59]) == nil {
		t.Fatal("foreign after snapshot path accepted")
	}
}

func TestForwardingRecoveredContextLineageRejectsUnprovedGeneratorChange(t *testing.T) {
	_, selected, previous, _, original, _ := replacementContextVector(t)
	for _, fault := range []string{"inode", "appearance", "disappearance"} {
		t.Run(fault, func(t *testing.T) {
			// The later own pair agrees internally, but its before snapshot
			// already differs from the previous proved generator observations.
			var before, after nativeContextSnapshot
			if json.Unmarshal(original, &before) != nil || json.Unmarshal(original, &after) != nil {
				t.Fatal("independent original fixture")
			}
			switch fault {
			case "inode":
				before.Ancestors[len(before.Ancestors)-1].Inode = 201
				after.Ancestors[len(after.Ancestors)-1].Inode = 202
			case "appearance":
				object := nativeObject{Path: "/run/systemd/generator.early", Device: 1, Inode: 201, Mode: 2147484141}
				before.Ancestors = append(before.Ancestors, object)
				before.Absent = slices.DeleteFunc(before.Absent, func(path string) bool { return path == object.Path })
				object.Inode = 202
				after.Ancestors = append(after.Ancestors, object)
				after.Absent = slices.DeleteFunc(after.Absent, func(path string) bool { return path == object.Path })
			case "disappearance":
				before.Ancestors = slices.DeleteFunc(before.Ancestors, func(object nativeObject) bool { return object.Path == "/run/systemd/generator" })
				before.Absent = append(before.Absent, "/run/systemd/generator")
				slices.Sort(before.Absent)
				after.Ancestors[len(after.Ancestors)-1].Inode = 202
			}
			encode := func(value nativeContextSnapshot) []byte {
				raw, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				return append(raw, '\n')
			}
			b, a := encode(before), encode(after)
			if err := compareReloadContexts(b, a); err != nil {
				t.Fatal("causal pair must otherwise be coherent", err)
			}
			proof := &replacementNativeProof{before: &initialFile{body: b}, after: &initialFile{body: a}}
			c := &completedInitial{files: &completedFiles{recovered: &recoveredInitial{nativeRaw: original}}, selectionOrder: []generationSelection{selected, previous}, reloads: map[string]*replacementNativeProof{selected.binding.Generation: proof}}
			if _, err := c.recoveredContextBytes(); err == nil {
				t.Fatal("unproved generator change before next own reload admitted")
			}
		})
	}
}

// Detached native vectors isolate chronology only; no runtime cut or reload
// authority is created by this graph.
func TestForwardingRecoveredRetirementContextPrecedesLaterOwnedReload(t *testing.T) {
	rows, selected, previous, _, before, after := replacementContextVector(t)
	proof := &replacementNativeProof{before: &initialFile{path: rows[9].Path, body: before}, after: &initialFile{path: rows[58].Path, body: after}}
	retired := &retirementSnapshot{binding: previous.binding, recovery: &retirementRecovery{nativeRaw: before}}
	current := &retirementSnapshot{binding: selected.binding, recovery: &retirementRecovery{nativeRaw: after}}
	c := &completedInitial{files: &completedFiles{invocations: &invocationChain{entries: []*invocationEntry{{selection: previous, retirement: retired}, {selection: selected, retirement: current}}}}, selectionOrder: []generationSelection{selected, previous}, reloads: map[string]*replacementNativeProof{selected.binding.Generation: proof}}
	raw, err := c.recoveredContextBytes()
	if err != nil || !bytes.Equal(raw, after) {
		t.Fatal("retirement/reload chronology lost", err)
	}
	delete(c.reloads, selected.binding.Generation)
	if _, err := c.recoveredContextBytes(); err == nil {
		t.Fatal("later retirement hid an unproved reload")
	}
	c.reloads[selected.binding.Generation] = proof
	current.recovery.nativeRaw = before
	if _, err := c.recoveredContextBytes(); err == nil {
		t.Fatal("later retirement rolled back generator proof")
	}
	current.recovery.nativeRaw = bytes.Replace(after, []byte(`"started":1`), []byte(`"started":2`), 1)
	if _, err := c.recoveredContextBytes(); err == nil {
		t.Fatal("later retirement changed original manager")
	}
}
