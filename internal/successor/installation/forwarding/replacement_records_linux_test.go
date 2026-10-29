package forwarding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
)

// This independently assembled byte transcript has invented inode numbers. It
// exercises only the closed grammar: it cannot establish native ownership,
// Release authorization, predecessor retirement or a safe recovery boundary.
func replacementRecordsVector(t *testing.T) ([]initialRecord, generationSelection, generationSelection, map[string]string) {
	t.Helper()
	previous, selected := replacementSelectionVector(t)
	b, r := selected.binding, selected.replacement
	root := b.Request.InstallationRoot
	sealed := filepath.Join(root, "generations", b.Generation)
	selection := filepath.Join(root, "binding.json")
	unit := "/etc/systemd/system/ardents-forwarding.service"
	hashes := map[string]string{
		"ardents-node-linux-amd64": b.ProgramSHA256, "ardents-forwarding.service": b.TemplateSHA256,
		"infrastructure-forwarding.json": b.Generation, "infrastructure-enrollment.json": b.InventorySHA256,
		"targets.json": b.TargetsSHA256, "forwarding-request.json": b.RequestSHA256,
		"forwarding-plan.json": b.PlanSHA256, "rendered-forwarding.service": b.UnitSHA256,
		"binding.json": digest(selected.raw), "root.json": digest([]byte("root")),
		"snapshot.json": digest([]byte("snapshot")), "timestamp.json": digest([]byte("timestamp")),
	}
	var records []initialRecord
	nextInode := uint64(100)
	appendRecord := func(phase, path, sha string, mode uint32, inode uint64) {
		record := initialRecord{Sequence: len(records), Phase: phase, Path: path, SHA256: sha, Mode: mode, Inode: inode}
		if inode != 0 {
			record.Device = 1
		}
		records = append(records, record)
	}
	owned := func(phase, path, sha string, mode uint32) uint64 {
		nextInode++
		appendRecord(phase, path, sha, mode, nextInode)
		return nextInode
	}
	write := func(path, sha string, mode uint32) {
		appendRecord("file-intent", path, sha, mode, 0)
		inode := owned("file-born", path, "", 0600)
		appendRecord("file-ready", path, sha, mode, inode)
	}
	owned("forwarding-replacement-v1", root+".forwarding-replacement-"+b.Generation+"-journal", "", 0600)
	owned("replacement-admitted", b.History, digest(selected.raw), 0700)
	appendRecord("native-context", root, digest([]byte("native observations")), 0, 0)
	selectedInode := owned("predecessor-binding", selection, digest(previous.raw), 0644)
	unitInode := owned("predecessor-unit", unit, previous.binding.UnitSHA256, 0644)
	owned("predecessor-retired", filepath.Join(attemptNumber(r.PredecessorAttempt).retirement(root), "retirement-result.json"), r.RetirementSHA256, 0600)
	write(filepath.Join(root, "forwarding-replacement-pending"), digest([]byte("ardents-forwarding-replacement-pending-v1\n"+digest(selected.raw)+"\n")), 0600)
	appendRecord("directory-intent", sealed, "", 0755, 0)
	directoryInode := owned("directory-born", sealed, "", 0700)
	appendRecord("directory-ready", sealed, "", 0755, directoryInode)
	// Literal inventory: the test does not borrow the implementation's names or
	// ordered phase generator.
	names := []string{"ardents-node-linux-amd64", "ardents-forwarding.service", "binding.json", "forwarding-plan.json", "forwarding-request.json", "infrastructure-enrollment.json", "infrastructure-forwarding.json", "rendered-forwarding.service", "root.json", "snapshot.json", "targets.json", "timestamp.json"}
	slices.Sort(names)
	for _, name := range names {
		mode := uint32(0444)
		if name == "ardents-node-linux-amd64" {
			mode = 0555
		}
		write(filepath.Join(sealed, name), hashes[name], mode)
	}
	appendRecord("seal-intent", sealed, b.Generation, 0555, directoryInode)
	appendRecord("sealed", sealed, b.Generation, 0555, directoryInode)
	appendRecord("selection-intent", selection, digest(selected.raw), 0644, selectedInode)
	appendRecord("selection-written", selection, digest(selected.raw), 0644, selectedInode)
	appendRecord("unit-intent", unit, b.UnitSHA256, 0644, unitInode)
	appendRecord("unit-written", unit, b.UnitSHA256, 0644, unitInode)
	appendRecord("reload-intent", unit, b.UnitSHA256, 0644, 0)
	appendRecord("replacement-verified", selection, digest(selected.raw), 0644, 0)
	if len(records) != 56 {
		t.Fatal("independent transcript inventory", len(records))
	}
	return records, selected, previous, hashes
}

func encodeReplacementVector(t *testing.T, records []initialRecord) []byte {
	t.Helper()
	var raw []byte
	for _, record := range records {
		line, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(append(raw, line...), '\n')
	}
	return raw
}

func TestForwardingReplacementRecordsIndependentCompleteVector(t *testing.T) {
	records, selected, previous, hashes := replacementRecordsVector(t)
	raw := encodeReplacementVector(t, records)
	owned, err := decodeReplacementContextRecords(raw, selected, previous, hashes, nil, nil)
	if err != nil || len(owned) != 19 {
		t.Fatal("complete byte grammar", len(owned), err)
	}
	root := selected.binding.Request.InstallationRoot
	if owned[filepath.Join(root, "binding.json")].Phase != "selection-written" || owned[filepath.Join(root, "generations", selected.binding.Generation)].Phase != "sealed" {
		t.Fatal("publication/seal commitments lost")
	}
	for n := 0; n < len(records); n++ {
		if _, err := decodeReplacementContextRecords(encodeReplacementVector(t, records[:n]), selected, previous, hashes, nil, nil); err == nil {
			t.Fatal("prefix supplied completed replacement", n)
		}
	}
	for _, bad := range [][]byte{bytes.TrimSuffix(raw, []byte{'\n'}), append(bytes.Clone(raw), '\n'), append(bytes.Clone(raw), raw...), bytes.Replace(raw, []byte("\"sequence\":0"), []byte("\"sequence\":0,\"unknown\":true"), 1), bytes.Replace(raw, []byte("\"sequence\":0"), []byte("\"sequence\":1,\"sequence\":0"), 1)} {
		if _, err := decodeReplacementContextRecords(bad, selected, previous, hashes, nil, nil); err == nil {
			t.Fatal("noncanonical transcript accepted")
		}
	}
}

func TestForwardingReplacementRecordsRejectChangedOriginalCommitments(t *testing.T) {
	original, selected, previous, hashes := replacementRecordsVector(t)
	for n := range original {
		for _, field := range []string{"sequence", "phase", "path", "sha256", "mode", "uid", "gid", "device", "inode"} {
			// SHA and native numbers of original observations are intentionally
			// detached. Only malformed hashes/zero native numbers are rejected by
			// this parser; genuine readback must establish their actual values.
			t.Run(fmt.Sprintf("record-%02d/%s", n, field), func(t *testing.T) {
				records := slices.Clone(original)
				r := &records[n]
				switch field {
				case "sequence":
					r.Sequence++
				case "phase":
					r.Phase += "-foreign"
				case "path":
					r.Path += "-foreign"
				case "sha256":
					r.SHA256 = "malformed"
				case "mode":
					r.Mode ^= 2
				case "uid":
					r.UID++
				case "gid":
					r.GID++
				case "device":
					if r.Device == 0 {
						r.Device = 1
					} else {
						r.Device = 0
					}
				case "inode":
					if r.Inode == 0 {
						r.Inode = 1
					} else {
						r.Inode = 0
					}
				}
				if _, err := decodeReplacementContextRecords(encodeReplacementVector(t, records), selected, previous, hashes, nil, nil); err == nil {
					t.Fatal("changed original commitment accepted")
				}
			})
		}
	}
	// Keeping both birth and ready internally consistent must not permit two
	// different objects (here the pending file and first generation file) to
	// claim one inode. A complete-looking publication cannot erase that alias.
	aliased := slices.Clone(original)
	aliased[13].Device, aliased[13].Inode = aliased[7].Device, aliased[7].Inode
	aliased[14].Device, aliased[14].Inode = aliased[7].Device, aliased[7].Inode
	if _, err := decodeReplacementContextRecords(encodeReplacementVector(t, aliased), selected, previous, hashes, nil, nil); err == nil {
		t.Fatal("shared native inode accepted")
	}
}
