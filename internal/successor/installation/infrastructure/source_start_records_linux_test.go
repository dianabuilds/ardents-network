package infrastructure

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// Independently stated persistence vector. No writer, native Start, installed
// entrypoint, Release proof or live State is constructed by this codec control.
func sourceStartTranscriptVector(t *testing.T) []byte {
	t.Helper()
	const rows = `first-start /installed.source-first-start-journal 1 10 a 384 0
directory-intent /installed/source-startup 0 0 - 0 0
directory-born /installed/source-startup 1 11 - 448 0
directory-ready /installed/source-startup 1 11 - 488 321
file-intent /installed/source-startup/start-guard.json 0 0 b 0 0
file-born /installed/source-startup/start-guard.json 1 12 b 384 0
file-ready /installed/source-startup/start-guard.json 1 12 b 384 0
socket-intent /installed/source-startup/start-completion.socket 0 0 - 0 0
socket-born /installed/source-startup/start-completion.socket 1 13 - 493 0
socket-ready /installed/source-startup/start-completion.socket 1 13 - 432 321
file-intent /installed/source-startup/start-socket.json 0 0 c 0 0
file-born /installed/source-startup/start-socket.json 1 14 c 384 0
file-ready /installed/source-startup/start-socket.json 1 14 c 384 0
start-attempt /installed/source-startup 0 0 b 0 0
invocation-observed /installed/source-startup 0 0 b 0 0
completion-received /installed/source-startup 0 0 e 0 0
file-intent /installed/source-startup/start-archived.json 0 0 d 0 0
file-born /installed/source-startup/start-archived.json 1 15 d 384 0
file-ready /installed/source-startup/start-archived.json 1 15 d 384 0
archived /installed/source-startup/start-archived.json 1 15 d 384 0
ack-attempt /installed/source-startup 0 0 e 0 0
ack-written /installed/source-startup 0 0 e 0 0`
	var out strings.Builder
	for index, line := range strings.Split(rows, "\n") {
		f := strings.Fields(line)
		if len(f) != 7 {
			t.Fatal("invalid independent vector", line)
		}
		hash := ""
		if f[4] != "-" {
			hash = strings.Repeat(f[4], 64)
		}
		fmt.Fprintf(&out, "{\"schema\":\"ardents-source-first-start-record-v1\",\"sequence\":%d,\"phase\":%q,\"path\":%q,\"device\":%s,\"inode\":%s,\"sha256\":%q,\"mode\":%s,\"uid\":0,\"gid\":%s}\n", index, f[0], f[1], f[2], f[3], hash, f[5], f[6])
	}
	return []byte(out.String())
}

func TestSourceStartTranscriptRequiresCompleteOriginalOrderAndNativeContinuity(t *testing.T) {
	raw := sourceStartTranscriptVector(t)
	decode := func(body []byte) (map[string]sourceStartEntry, error) {
		return decodeSourceStartRecords(body, 0, "/installed.source-first-start-journal", "/installed/source-startup",
			strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("d", 64), strings.Repeat("e", 64), 321)
	}
	owned, err := decode(raw)
	if err != nil || len(owned) != 6 || owned["/installed/source-startup/start-archived.json"].Inode != 15 {
		t.Fatalf("independent complete transcript refused: %v %v", owned, err)
	}
	// Every proper prefix, including archive-before-ACK and ACK-attempt, refuses.
	for i := range raw {
		if _, err := decode(raw[:i]); err == nil {
			t.Fatalf("partial transcript accepted at byte %d", i)
		}
	}
	lines := bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'})
	for index := range lines {
		for _, change := range []struct{ old, new string }{
			{`"sequence":` + strconv.Itoa(index), `"sequence":99`},
			{`"uid":0`, `"uid":321`},
			{`"schema":"ardents-source-first-start-record-v1"`, `"schema":"other"`},
			{`"phase":"`, `"phase":"foreign-`},
			{`"path":"`, `"path":"/foreign`},
			{`"inode":`, `"inode":9`},
			{`"mode":`, `"mode":9`},
			{`"gid":`, `"gid":9`},
		} {
			// A birth's detached inode is checked against the real descriptor by
			// the consumer. Only repeated identities can be compared by a codec.
			if index == 0 && change.old == `"inode":` {
				continue
			}
			altered := append([][]byte(nil), lines...)
			altered[index] = bytes.Replace(lines[index], []byte(change.old), []byte(change.new), 1)
			body := append(bytes.Join(altered, []byte{'\n'}), '\n')
			if _, err := decode(body); err == nil {
				t.Fatalf("changed original record accepted: record %d field %s", index, change.old)
			}
		}
	}
	for _, body := range [][]byte{
		append(bytes.Clone(raw), raw...),
		bytes.Replace(raw, []byte(`"sequence":0`), []byte(`"sequence":0,"sequence":0`), 1),
		bytes.Replace(raw, []byte(`"sequence":0`), []byte(`"sequence": 0`), 1),
		bytes.Replace(raw, []byte(strings.Repeat("d", 64)), []byte(strings.Repeat("f", 64)), 1),
	} {
		if _, err := decode(body); err == nil {
			t.Fatal("noncanonical, replayed or substituted transcript accepted")
		}
	}
}
