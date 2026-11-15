package introduction

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// The oracle uses an independent row DTO and an explicit persisted transcript.
// No successful native custody or authority is manufactured by these bytes.
func TestIntroductionFirstStartTranscriptRejectsPartialOrForeignHistory(t *testing.T) {
	const root = "/var/lib/intro/node"
	const directory = root + "/introduction-startup"
	const journal = root + ".introduction-first-start-journal"
	h := strings.Repeat("a", 64)
	binding, guard, socket, archive, frame, ack := h, strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("d", 64), strings.Repeat("e", 64), strings.Repeat("f", 64)
	type row struct {
		Schema   string `json:"schema"`
		Sequence uint32 `json:"sequence"`
		Phase    string `json:"phase"`
		Path     string `json:"path"`
		Device   uint64 `json:"device"`
		Inode    uint64 `json:"inode"`
		SHA256   string `json:"sha256"`
		Mode     uint32 `json:"mode"`
		UID      uint32 `json:"uid"`
		GID      uint32 `json:"gid"`
	}
	var rows []row
	inode := map[string]uint64{}
	add := func(phase, path, sha string, mode, gid uint32, native bool) {
		r := row{Schema: "ardents-introduction-start-journal-v1", Sequence: uint32(len(rows)), Phase: phase, Path: path, SHA256: sha, Mode: mode, GID: gid}
		if native {
			if inode[path] == 0 {
				inode[path] = uint64(len(inode) + 10)
			}
			r.Device = 7
			r.Inode = inode[path]
		}
		rows = append(rows, r)
	}
	add("first-start", journal, binding, 0600, 0, true)
	add("native-context", root, h, 0, 0, false)
	add("directory-intent", directory, "", 0, 0, false)
	add("directory-born", directory, "", 0700, 0, true)
	add("directory-ready", directory, "", 0750, 991, true)
	file := func(name, sha string) {
		path := directory + "/" + name
		add("file-intent", path, sha, 0, 0, false)
		add("file-born", path, sha, 0600, 0, true)
		add("file-ready", path, sha, 0600, 0, true)
	}
	file("introduction-start-pending", digest([]byte("ardents-introduction-start-pending-v1\n"+binding+"\n")))
	file("start-guard.json", guard)
	add("socket-intent", directory+"/start-completion.socket", "", 0, 0, false)
	add("socket-born", directory+"/start-completion.socket", "", 0755, 0, true)
	add("socket-ready", directory+"/start-completion.socket", "", 0660, 991, true)
	file("start-socket.json", socket)
	add("start-attempt", directory, guard, 0, 0, false)
	add("invocation-observed", directory, guard, 0, 0, false)
	add("completion-received", directory, frame, 0, 0, false)
	file("start-archived.json", archive)
	add("archived", directory+"/start-archived.json", archive, 0600, 0, true)
	add("ack-attempt", directory, ack, 0, 0, false)
	add("ack-written", directory, ack, 0, 0, false)
	encode := func(items []row) []byte {
		var raw []byte
		for _, item := range items {
			b, err := json.Marshal(item)
			if err != nil {
				t.Fatal(err)
			}
			raw = append(raw, append(b, '\n')...)
		}
		return raw
	}
	decode := func(raw []byte) error {
		_, err := decodeStartRecordsFrames(raw, journal, directory, binding, guard, socket, archive, frame, ack, 991)
		return err
	}
	raw := encode(rows)
	if len(rows) != 26 || decode(raw) != nil {
		t.Fatal("independent complete transcript refused", len(rows), decode(raw))
	}
	for count := 0; count < len(rows); count++ {
		if err := decode(encode(rows[:count])); !errors.Is(err, ErrBinding) {
			t.Fatal("incomplete transcript accepted", count, err)
		}
	}
	for index := range rows {
		bad := append([]row(nil), rows...)
		bad[index].Sequence++
		if decode(encode(bad)) == nil {
			t.Fatal("foreign sequence accepted", index)
		}
	}
	for _, index := range []int{4, 7, 10, 13, 16, 22, 23} {
		bad := append([]row(nil), rows...)
		bad[index].Inode++
		if decode(encode(bad)) == nil {
			t.Fatal("same-path substituted inode accepted", index)
		}
	}
	for _, bad := range [][]byte{raw[:len(raw)-1], append(bytes.Clone(raw), '\n'), bytes.Replace(raw, []byte("first-start"), []byte("invocation"), 1), bytes.Replace(raw, []byte("introduction-start-journal"), []byte("descriptor-start-journal"), 1)} {
		if decode(bad) == nil {
			t.Fatal("foreign or noncanonical transcript accepted")
		}
	}
}
