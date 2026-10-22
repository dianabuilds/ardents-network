package infrastructure

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func sourceRetirementTranscriptVector(t *testing.T) []byte {
	t.Helper()
	const rows = `stop source-retirement-journal 1 30 a 384 0
socket-intent stop-completion.socket 0 0 - 0 0
socket-born stop-completion.socket 1 31 - 493 0
socket-ready stop-completion.socket 1 31 - 432 321
interrupt-attempt . 0 0 b 0 0
domain-joined . 0 0 c 0 0
native-joined . 0 0 d 0 0
native-closed . 0 0 d 0 0
file-intent source-retirement.json 0 0 d 0 0
file-born source-retirement.json 1 32 d 384 0
file-ready source-retirement.json 1 32 d 384 0
retired source-retirement.json 1 32 d 384 0`
	var out strings.Builder
	for i, line := range strings.Split(rows, "\n") {
		f := strings.Fields(line)
		if len(f) != 7 {
			t.Fatal("invalid independent vector")
		}
		path := "/installed/source-startup"
		if f[1] != "." {
			path += "/" + f[1]
		}
		hash := ""
		if f[4] != "-" {
			hash = strings.Repeat(f[4], 64)
		}
		fmt.Fprintf(&out, "{\"schema\":\"ardents-source-retirement-record-v1\",\"sequence\":%d,\"phase\":%q,\"path\":%q,\"device\":%s,\"inode\":%s,\"sha256\":%q,\"mode\":%s,\"uid\":0,\"gid\":%s}\n", i, f[0], path, f[2], f[3], hash, f[5], f[6])
	}
	return []byte(out.String())
}

func TestSourceRetirementTranscriptRequiresDomainNativeJoinCloseAndExactCommit(t *testing.T) {
	raw := sourceRetirementTranscriptVector(t)
	decode := func(body []byte) (map[string]sourceStartEntry, error) {
		return decodeSourceRetirementRecords(body, "/installed/source-startup", strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("d", 64), 321)
	}
	owned, err := decode(raw)
	if err != nil || len(owned) != 3 || owned["/installed/source-startup/source-retirement.json"].Inode != 32 {
		t.Fatal("independent complete retirement vector refused", owned, err)
	}
	for i := range raw {
		if _, err := decode(raw[:i]); err == nil {
			t.Fatal("torn or unfinished retirement accepted", i)
		}
	}
	for _, change := range []struct{ old, new string }{
		{`"domain-joined"`, `"ack-written"`},
		{`"native-joined"`, `"process-absent"`},
		{`"native-closed"`, `"inactive"`},
		{`"retired"`, `"completion-unavailable"`},
		{`"inode":32`, `"inode":33`},
		{`"uid":0`, `"uid":321`},
		{`"gid":321`, `"gid":322`},
		{`"sequence":5`, `"sequence":6`},
		{`"sequence":0`, `"sequence":0,"sequence":0`},
		{strings.Repeat("a", 64), strings.Repeat("f", 64)},
		{strings.Repeat("b", 64), strings.Repeat("f", 64)},
		{strings.Repeat("c", 64), strings.Repeat("f", 64)},
		{strings.Repeat("d", 64), strings.Repeat("f", 64)},
	} {
		body := bytes.Replace(raw, []byte(change.old), []byte(change.new), 1)
		if bytes.Equal(body, raw) {
			t.Fatal("negative control did not alter independent vector", change.old)
		}
		if _, err := decode(body); err == nil {
			t.Fatal("substituted original retirement evidence accepted", change.old)
		}
	}
	if _, err := decode(append(bytes.Clone(raw), raw...)); err == nil {
		t.Fatal("replayed retirement accepted")
	}
}

func TestSourceRetirementChallengeHasOneExactNonzeroEncoding(t *testing.T) {
	text := strings.Repeat("01", 32)
	if challenge, err := sourceRetirementChallenge(text); err != nil || challenge[0] != 1 || challenge[31] != 1 {
		t.Fatal("independent challenge encoding refused", challenge, err)
	}
	for _, value := range []string{"", strings.Repeat("00", 32), strings.Repeat("ff", 31), strings.Repeat("FF", 32), text + "00"} {
		if _, err := sourceRetirementChallenge(value); err == nil {
			t.Fatal("noncanonical/zero challenge accepted", value)
		}
	}
}
