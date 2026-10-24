//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// Native files only, with controlled account-row text. This fixture runs no
// useradd, Source, manager, Release or accepted installed operation.
func TestSourceCanonicalPhysicalEarlyImageNative(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-source-stopped-native-stand")
	if err != nil || string(marker) != "ardents-source-stopped-native-v1\n" || os.Geteuid() != 0 {
		t.Fatal("isolated root native stand required", err)
	}
	raw, _, _ := sourceInitialRecordVector(t, true)
	vector, err := decodeSourceRecoveryPrefix(raw, sourceRecoverableInitialRecords)
	if err != nil {
		t.Fatal(err)
	}
	account := t.TempDir()
	var fs syscall.Statfs_t
	if err := syscall.Statfs(account, &fs); err != nil || fs.Type != 0xef53 {
		t.Fatal("actual ext4 native stand required", err)
	}
	old := []byte("foreign-original-row\n")
	rows := map[string]string{"passwd": "ardents-source:x:999:999::/nonexistent:/usr/sbin/nologin\n", "group": "ardents-source:x:999:\n",
		"shadow": "ardents-source:!:20000::::::\n", "gshadow": "ardents-source:!::\n"}
	for name, row := range rows {
		for leaf, body := range map[string][]byte{name: append(bytes.Clone(old), []byte(row)...), name + "-": old} {
			if err := os.WriteFile(filepath.Join(account, leaf), body, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(account, ".pwd.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	// Bind the independent grammar vector's preimage commitments and recorded
	// results to actual private native files. The facts grant no account custody.
	for i := range vector {
		record := &vector[i]
		if record.Phase == "account-preimage" {
			record.SHA256 = digest(old)
		}
		if record.Phase != "account-born" && record.Phase != "account-auxiliary-result" {
			continue
		}
		file, err := readSourceAccountFile(filepath.Join(account, filepath.Base(record.Path)))
		if err != nil {
			t.Fatal(err)
		}
		entry := sourcePhysicalEarlyEntryFor(record.Path, file)
		entry.Record.Sequence, entry.Record.Phase = record.Sequence, record.Phase
		*record = entry.Record
		if err := file.file.Close(); err != nil {
			t.Fatal(err)
		}
		clear(file.raw)
	}
	for _, count := range []int{24, 42} {
		first, _, err := sourcePhysicalEarlyAccountImage(vector[:count], account)
		if err != nil {
			t.Fatal("actual native early image", count, err)
		}
		second, _, err := sourcePhysicalEarlyAccountImage(vector[:count], account)
		if err != nil || !bytes.Equal(first, second) {
			t.Fatal("stable unchanged native account image", count, err)
		}
		var entries []sourcePhysicalEarlyEntry
		if json.Unmarshal(first, &entries) != nil || len(entries) != 17 || entries[1].Path != "/etc/group" {
			t.Fatal("closed native account inventory")
		}
	}
	for name, change := range map[string]func([]sourceRecord){
		"wrong-preimage": func(v []sourceRecord) { v[5].SHA256 = strings.Repeat("0", 64) },
		"copied-born":    func(v []sourceRecord) { v[23].Inode++ },
		"foreign-uid":    func(v []sourceRecord) { v[22].UID = 998 },
		"foreign-path":   func(v []sourceRecord) { v[23].Path = "/etc/foreign" },
	} {
		v := append([]sourceRecord(nil), vector[:24]...)
		change(v)
		if _, _, err := sourcePhysicalEarlyAccountImage(v, account); err == nil {
			t.Fatal("foreign early image accepted", name)
		}
	}
	foreign := filepath.Join(account, "passwd.lock")
	if err := os.WriteFile(foreign, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := sourcePhysicalEarlyAccountImage(vector[:24], account); err == nil {
		t.Fatal("unexpected original account auxiliary adopted")
	}
	root := filepath.Join(t.TempDir(), "root-birth")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	before, info, err := sourcePhysicalEarlyDirectory(root)
	if err != nil || info == nil || !before.Present {
		t.Fatal("actual empty root0700 directory birth", err)
	}
	if err := os.Chmod(root, 0750); err != nil {
		t.Fatal(err)
	}
	if _, _, err := sourcePhysicalEarlyDirectory(root); err == nil {
		t.Fatal("unrecorded ready access accepted")
	}
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "unrecorded-dependent-file"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := sourcePhysicalEarlyDirectory(root); err == nil {
		t.Fatal("later dependent birth accepted")
	}
}
