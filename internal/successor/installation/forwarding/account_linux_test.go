package forwarding

import (
	"bytes"
	"strings"
	"testing"
)

func TestNodeAccountDeltaPreservesForeignBytesAndLockedIdentity(t *testing.T) {
	before := []byte("root:x:0:0:root:/root:/bin/bash\n")
	for _, v := range []struct{ path, row string }{
		{"/etc/passwd", "ardents-node:x:999:999::/nonexistent:/usr/sbin/nologin"},
		{"/etc/group", "ardents-node:x:999:"},
		{"/etc/shadow", "ardents-node:!:20000::::::"},
		{"/etc/gshadow", "ardents-node:!::"},
	} {
		after := append(bytes.Clone(before), []byte(v.row+"\n")...)
		if err := nodeAccountDelta(v.path, before, after, 999, 999); err != nil {
			t.Fatal(v.path, err)
		}
		for _, bad := range [][]byte{after[:len(after)-1], append(bytes.Clone(after), []byte(v.row+"\n")...), bytes.Replace(after, []byte("root:x"), []byte("root:!"), 1), bytes.Replace(after, []byte(v.row), []byte(strings.ReplaceAll(v.row, "999", "998")), 1)} {
			if bytes.Equal(bad, after) {
				continue
			}
			if nodeAccountDelta(v.path, before, bad, 999, 999) == nil {
				t.Fatalf("accepted substituted account/preimage: %s", v.path)
			}
		}
	}
	for _, row := range []string{"ardents-node::20000::::::", "ardents-node:$6$hash:20000::::::", "ardents-node:!:20000:0:99999:7:::"} {
		if nodeAccountDelta("/etc/shadow", before, append(bytes.Clone(before), []byte(row+"\n")...), 999, 999) == nil {
			t.Fatal("unlocked/aging shadow accepted")
		}
	}
}
