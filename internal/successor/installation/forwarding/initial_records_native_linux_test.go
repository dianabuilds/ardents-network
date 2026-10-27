//go:build linux && network_signing_native

package forwarding

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// The checked private controller invokes this only after actual canonical
// install-initial-stopped and its separate native readback have completed. No
// supplied receipt, test record or fixture can provide installation authority.
func TestForwardingInitialNativeCompleteRecordsAndCausalRefusals(t *testing.T) {
	path := os.Getenv("ARDENTS_FORWARDING_INITIAL_BINDING")
	if os.Geteuid() != 0 || !validPath(path) {
		t.Fatal("invalid environment: genuine private initial stopped binding required")
	}
	bound, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := decodeBinding(bound)
	if err != nil || path != filepath.Join(b.Request.InstallationRoot, "binding.json") {
		t.Fatal("actual selected initial binding", err)
	}
	journal := b.Request.InstallationRoot + ".forwarding-initial-journal"
	pending := b.Request.InstallationRoot + ".forwarding-initial-pending"
	if _, err := os.Lstat(pending); !os.IsNotExist(err) {
		t.Fatal("invalid environment: original completion still blocked", err)
	}
	dir := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 12 {
		t.Fatal("actual sealed inventory", err)
	}
	hashes := map[string]string{}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		hashes[path] = digest(raw)
	}
	for _, path := range []string{filepath.Join(b.Request.CredentialDirectory, "certificate.pem"), filepath.Join(b.Request.CredentialDirectory, "private-key.pem"), filepath.Join(b.Request.InstallationRoot, "binding.json"), "/etc/systemd/system/ardents-forwarding.service"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		hashes[path] = digest(raw)
		clear(raw)
	}
	hashes[pending] = digest([]byte("ardents-forwarding-initial-pending-v1\n" + digest(bound) + "\n"))
	raw, _, err := readInitialOriginal(t.Context(), journal, 4<<20, 0600, 0, 0, false)
	if err != nil {
		t.Fatal("actual original journal", err)
	}
	owned, err := decodeInitialRecords(raw, b, hashes)
	if err != nil || len(owned) != 27 {
		t.Fatal("complete genuine ordered initial inventory", len(owned), err)
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	for index := range lines {
		var record initialRecord
		if err := json.Unmarshal(lines[index], &record); err != nil {
			t.Fatal(err)
		}
		record.Phase = "substituted"
		modified, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		changed := append([][]byte(nil), lines...)
		changed[index] = modified
		if _, err := decodeInitialRecords(append(bytes.Join(changed, []byte{'\n'}), '\n'), b, hashes); !errors.Is(err, ErrBinding) {
			t.Fatal("record was not consumed at exact phase", index, err)
		}
		changed = append(append([][]byte(nil), lines[:index]...), lines[index+1:]...)
		if _, err := decodeInitialRecords(append(bytes.Join(changed, []byte{'\n'}), '\n'), b, hashes); !errors.Is(err, ErrBinding) {
			t.Fatal("omitted original record accepted", index, err)
		}
	}
	for index := 1; index < len(lines); index++ {
		var previous, current initialRecord
		if json.Unmarshal(lines[index-1], &previous) != nil || json.Unmarshal(lines[index], &current) != nil {
			t.Fatal("original canonical journal")
		}
		previous.Sequence, current.Sequence = index, index-1
		changed := append([][]byte(nil), lines...)
		changed[index-1], _ = json.Marshal(current)
		changed[index], _ = json.Marshal(previous)
		if _, err := decodeInitialRecords(append(bytes.Join(changed, []byte{'\n'}), '\n'), b, hashes); !errors.Is(err, ErrBinding) {
			t.Fatal("reordered phases accepted after sequence repair", index, err)
		}
	}
	for _, bad := range [][]byte{raw[:len(raw)-1], append(bytes.Clone(raw), raw...), append(bytes.Clone(raw), '\n'), []byte("null\n")} {
		if _, err := decodeInitialRecords(bad, b, hashes); !errors.Is(err, ErrBinding) {
			t.Fatal("incomplete or noncanonical transcript accepted", err)
		}
	}
	// The creator-only readback requires its original still-blocking pending
	// inode. Completed bytes and detached records cannot recreate that custody
	// after publication or act as a generic Startup/recovery opener.
	if err := verifyInitialBirthReadback(t.Context(), journal, b, hashes); err == nil {
		t.Fatal("creator readback adopted already published installation")
	}
}
