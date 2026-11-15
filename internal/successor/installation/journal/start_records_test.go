package journal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Fixture encodes the retained 25-record transcript independently of InvocationRecord.
func startRecordFixture(t *testing.T, role string) ([]byte, string) {
	t.Helper()
	h := strings.Repeat("a", 64)
	sum := sha256.Sum256([]byte("ardents-" + role + "-start-pending-v1\n" + h + "\n"))
	pending := hex.EncodeToString(sum[:])
	type record struct {
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
	var rows []record
	add := func(phase, path, hash string, mode, gid uint32, native bool) {
		v := record{Schema: "ardents-" + role + "-start-journal-v1", Sequence: uint32(len(rows)), Phase: phase, Path: path, SHA256: hash, Mode: mode, GID: gid}
		if native {
			v.Device, v.Inode = 1, 2
		}
		rows = append(rows, v)
	}
	add("first-start", "/j", h, 0600, 0, true)
	add("directory-intent", "/d", "", 0, 0, false)
	add("directory-born", "/d", "", 0700, 0, true)
	add("directory-ready", "/d", "", 0750, 988, true)
	file := func(name, digest string) {
		path := "/d/" + name
		add("file-intent", path, digest, 0, 0, false)
		add("file-born", path, digest, 0600, 0, true)
		add("file-ready", path, digest, 0600, 0, true)
	}
	file(role+"-start-pending", pending)
	file("start-guard.json", h)
	add("socket-intent", "/d/start-completion.socket", "", 0, 0, false)
	add("socket-born", "/d/start-completion.socket", "", 0700, 0, true)
	add("socket-ready", "/d/start-completion.socket", "", 0660, 988, true)
	file("start-socket.json", h)
	for _, phase := range []string{"start-attempt", "invocation-observed", "completion-received"} {
		add(phase, "/d", h, 0, 0, false)
	}
	file("start-archived.json", h)
	add("archived", "/d/start-archived.json", h, 0600, 0, true)
	add("ack-attempt", "/d", h, 0, 0, false)
	add("ack-written", "/d", h, 0, 0, false)
	if len(rows) != 25 {
		t.Fatal("independent fixture count", len(rows))
	}
	var out []byte
	for _, row := range rows {
		raw, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, raw...)
		out = append(out, '\n')
	}
	return out, h
}

func TestStartRecordsPreserveRoleBytesAndOrdering(t *testing.T) {
	for _, role := range []string{"join", "descriptor"} {
		raw, h := startRecordFixture(t, role)
		decode := func(input []byte, purpose string) error {
			_, err := DecodeStartRecords(input, purpose, 0, "/j", "/d", h, h, h, h, h, h, 988)
			return err
		}
		if err := decode(raw, role); err != nil {
			t.Fatal("original transcript rejected", role, err)
		}
		foreign := "join"
		if role == "join" {
			foreign = "descriptor"
		}
		if decode(raw, foreign) == nil {
			t.Fatal("foreign role accepted", role)
		}
		if decode(bytes.Replace(raw, []byte(`"sequence":1`), []byte(`"sequence":9`), 1), role) == nil {
			t.Fatal("out of order transcript accepted", role)
		}
		if decode(append(bytes.Clone(raw), '\n'), role) == nil {
			t.Fatal("noncanonical extra line accepted", role)
		}
	}
}

func TestStartRecordsRejectIncompleteAndForeignPurpose(t *testing.T) {
	digest := strings.Repeat("a", 64)
	for _, purpose := range []string{"join", "descriptor", "source", ""} {
		if _, err := DecodeStartRecords([]byte("{}\n"), purpose, 0, "/root/journal", "/root/start", digest, digest, digest, digest, digest, digest, 988); !errors.Is(err, ErrBinding) {
			t.Fatal("partial or foreign transcript accepted", purpose, err)
		}
	}
	if _, err := DecodeStartRecords(nil, "join", 64, "/root/journal", "/root/start", digest, digest, digest, digest, digest, digest, 988); !errors.Is(err, ErrBinding) {
		t.Fatal("exhausted attempt accepted", err)
	}
}

func TestIntroductionStartRequiresContextAndInvocationSchema(t *testing.T) {
	raw, h := startRecordFixture(t, "introduction")
	decode := func(body []byte, number uint32) error {
		_, err := DecodeStartRecords(body, "introduction", number, "/j", "/d", h, h, h, h, h, h, 988)
		return err
	}
	if decode(raw, 0) == nil {
		t.Fatal("missing native context admitted")
	}
	lines := bytes.Split(bytes.TrimSuffix(raw, []byte("\n")), []byte("\n"))
	for i := 1; i < len(lines); i++ {
		lines[i] = bytes.Replace(lines[i], []byte(fmt.Sprintf("\"sequence\":%d,", i)), []byte(fmt.Sprintf("\"sequence\":%d,", i+1)), 1)
	}
	context := []byte(fmt.Sprintf("{\"schema\":\"ardents-introduction-start-journal-v1\",\"sequence\":1,\"phase\":\"native-context\",\"path\":\"/\",\"device\":0,\"inode\":0,\"sha256\":%q,\"mode\":0,\"uid\":0,\"gid\":0}", h))
	lines = append(lines[:1], append([][]byte{context}, lines[1:]...)...)
	complete := append(bytes.Join(lines, []byte("\n")), '\n')
	if err := decode(complete, 0); err != nil {
		t.Fatal("first start refused", err)
	}
	invocation := bytes.ReplaceAll(complete, []byte("ardents-introduction-start-journal-v1"), []byte("ardents-introduction-invocation-journal-v1"))
	invocation = bytes.Replace(invocation, []byte("\"phase\":\"first-start\""), []byte("\"phase\":\"invocation\""), 1)
	if err := decode(invocation, 1); err != nil {
		t.Fatal("invocation refused", err)
	}
	if decode(invocation, 0) == nil || decode(complete, 1) == nil {
		t.Fatal("invocation schema confused with first start")
	}
}
