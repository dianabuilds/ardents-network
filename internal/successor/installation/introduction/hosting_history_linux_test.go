package introduction

import (
	"context"
	"encoding/json"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"path/filepath"
	"testing"
)

// A separately enumerated sixteen-record oracle exercises only the finite
// transcript grammar. It supplies no native birth or preparation permission.
func TestIntroductionHostingTranscriptRequiresCompleteOrderedOriginals(t *testing.T) {
	b, _, _ := initialTranscriptOracle(t)
	bound, _ := json.Marshal(b)
	bound = append(bound, '\n')
	intent := hostingBirth{"ardents-introduction-hosting-intent-v1", digest(bound), 2, 300, digest([]byte("policy"))}
	intentRaw, _ := json.Marshal(intent)
	intentRaw = append(intentRaw, '\n')
	completion := hostingCompletion{"ardents-introduction-hosting-completed-v1", digest(intentRaw), hostingFileIdentity{2, 501}, hostingFileIdentity{2, 502}}
	resultRaw, _ := json.Marshal(completion)
	resultRaw = append(resultRaw, '\n')
	directory := b.Request.InstallationRoot + ".introduction-hosting"
	journal := directory + "-journal"
	pending := directory + "-pending"
	var records []initialRecord
	inode := uint64(400)
	add := func(r initialRecord) { r.Sequence = len(records); records = append(records, r) }
	native := func(r initialRecord) initialRecord { inode++; r.Device = 2; r.Inode = inode; return r }
	write := func(path string, raw []byte) {
		add(initialRecord{Phase: "file-intent", Path: path, SHA256: digest(raw), Mode: 0600})
		r := native(initialRecord{Phase: "file-born", Path: path, Mode: 0600})
		add(r)
		r.Phase = "file-ready"
		r.SHA256 = digest(raw)
		add(r)
	}
	add(native(initialRecord{Phase: "introduction-hosting-v1", Path: journal, Mode: 0600}))
	write(pending, []byte("ardents-introduction-hosting-pending-v1\n"+digest(intentRaw)+"\n"))
	add(initialRecord{Phase: "directory-intent", Path: directory, Mode: 0700})
	dir := native(initialRecord{Phase: "directory-born", Path: directory, Mode: 0700})
	add(dir)
	dir.Phase = "directory-ready"
	add(dir)
	write(filepath.Join(directory, "intent.json"), intentRaw)
	root := initialRecord{Phase: "admitted", Path: b.Request.HostingRoot, Device: 2, Inode: 300, SHA256: intent.Policy, Mode: 0700, UID: b.UID, GID: b.GID}
	add(root)
	root.Phase = "selected"
	root.SHA256 = digest(resultRaw)
	add(root)
	write(filepath.Join(directory, "completed.json"), resultRaw)
	dir.Phase = "completed"
	dir.SHA256 = digest(resultRaw)
	add(dir)
	raw := encodeInitialOracle(t, records)
	owned, err := decodeHostingRecords(raw, b, intentRaw, resultRaw)
	if err != nil || len(records) != 16 || len(owned) != 6 {
		t.Fatal("independent complete Hosting oracle refused", len(records), len(owned), err)
	}
	for n := range records {
		changed := append([]initialRecord(nil), records...)
		changed[n].Phase = "foreign"
		if _, err := decodeHostingRecords(encodeInitialOracle(t, changed), b, intentRaw, resultRaw); err == nil {
			t.Fatal("foreign effect accepted", n)
		}
		if _, err := decodeHostingRecords(encodeInitialOracle(t, records[:n]), b, intentRaw, resultRaw); err == nil {
			t.Fatal("incomplete original accepted", n)
		}
	}
	if _, err := decodeHostingRecords(raw[:len(raw)-1], b, intentRaw, resultRaw); err == nil {
		t.Fatal("missing final newline accepted")
	}
	if _, err := decodeHostingRecords(raw, b, append(intentRaw, ' '), resultRaw); err == nil {
		t.Fatal("changed intent commitment accepted")
	}
	if _, err := decodeHostingRecords(raw, b, intentRaw, append(resultRaw, ' ')); err == nil {
		t.Fatal("changed completion commitment accepted")
	}
}
func TestIntroductionHostingUnavailableAdmissionCreatesNothing(t *testing.T) {
	if err := PrepareHosting(nil, "/binding", hosting.Policy{}); err == nil {
		t.Fatal("nil original caller accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := PrepareHosting(ctx, "/binding", hosting.Policy{}); err == nil {
		t.Fatal("canceled original caller accepted")
	}
}
