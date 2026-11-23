package process

import (
	"strings"
	"testing"
)

func TestDedicatedAccountID(t *testing.T) {
	const passwd = "root:x:0:0:root:/root:/bin/sh\nardents-node:x:999:999::/nonexistent:/usr/sbin/nologin\n"
	const group = "root:x:0:\nardents-node:x:999:\n"
	if uid, ok := DedicatedAccountID([]byte(passwd), []byte(group), "ardents-node"); !ok || uid != 999 {
		t.Fatalf("fixed unprivileged principal: uid=%d admitted=%v", uid, ok)
	}
	for _, bad := range []struct {
		name   string
		passwd string
		group  string
	}{
		{"root-id", strings.ReplaceAll(passwd, "999", "0"), group},
		{"shared-uid", passwd + "alias:x:999:999::/nonexistent:/usr/sbin/nologin\n", group},
		{"interactive-shell", strings.ReplaceAll(passwd, "/usr/sbin/nologin", "/bin/sh"), group},
		{"duplicate-name", passwd + "ardents-node:x:999:999::/nonexistent:/usr/sbin/nologin\n", group},
		{"shared-gid", passwd, group + "alias:x:999:\n"},
		{"supplementary-group", passwd, group + "other:x:998:ardents-node\n"},
	} {
		t.Run(bad.name, func(t *testing.T) {
			if uid, ok := DedicatedAccountID([]byte(bad.passwd), []byte(bad.group), "ardents-node"); ok || uid != 0 {
				t.Fatalf("invalid account returned uid=%d admitted=%v", uid, ok)
			}
		})
	}
}
