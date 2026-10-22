package infrastructure

import (
	"strings"
	"testing"
)

func TestSourcePrincipalCurrentRowsDoNotFreezeForeignHostAccounts(t *testing.T) {
	const passwd = "root:x:0:0:root:/root:/bin/bash\nardents-source:x:123:123::/nonexistent:/usr/sbin/nologin\n"
	const group = "root:x:0:\nardents-source:x:123:\n"
	const nss = "passwd: files systemd\ngroup: files systemd\n"
	for _, p := range []string{passwd, passwd + "other:x:124:124::/home/other:/bin/bash\n"} {
		g := group
		if p != passwd {
			g += "other:x:124:\n"
		}
		rows, err := validateSourcePublicPrincipal([]byte(p), []byte(g), []byte(nss), 123, 123)
		if err != nil || rows["passwd"] != "ardents-source:x:123:123::/nonexistent:/usr/sbin/nologin" || rows["group"] != "ardents-source:x:123:" {
			t.Fatal("current fixed principal refused after unrelated completed host update", err)
		}
	}
	for name, mutate := range map[string]func() (string, string, string){
		"uid-alias": func() (string, string, string) {
			return passwd + "other:x:123:124::/home/other:/bin/bash\n", group, nss
		},
		"noncanonical-uid-alias": func() (string, string, string) {
			return passwd + "other:x:00123:124::/home/other:/bin/bash\n", group, nss
		},
		"foreign-primary-group": func() (string, string, string) {
			return passwd + "other:x:124:123::/home/other:/bin/bash\n", group, nss
		},
		"gid-alias":           func() (string, string, string) { return passwd, group + "other:x:123:\n", nss },
		"supplementary-group": func() (string, string, string) { return passwd, group + "other:x:124:ardents-source\n", nss },
		"group-members": func() (string, string, string) {
			return passwd, strings.Replace(group, "source:x:123:", "source:x:123:other", 1), nss
		},
		"duplicate-principal": func() (string, string, string) {
			return passwd + "ardents-source:x:123:123::/nonexistent:/usr/sbin/nologin\n", group, nss
		},
		"login-shell": func() (string, string, string) {
			return strings.Replace(passwd, "/usr/sbin/nologin", "/bin/bash", 1), group, nss
		},
		"foreign-nss": func() (string, string, string) {
			return passwd, group, strings.Replace(nss, "files systemd", "files ldap", 1)
		},
		"duplicate-nss": func() (string, string, string) { return passwd, group, nss + "passwd: files\n" },
	} {
		t.Run(name, func(t *testing.T) {
			p, g, n := mutate()
			if _, err := validateSourcePublicPrincipal([]byte(p), []byte(g), []byte(n), 123, 123); err == nil {
				t.Fatal("foreign current principal accepted")
			}
		})
	}
}

func TestSourcePrincipalPrivateRowsRetainFixedLockedContract(t *testing.T) {
	if err := validateSourcePrincipalRow("/etc/shadow", "ardents-source:!:20000::::::", 123, 123); err != nil {
		t.Fatal(err)
	}
	if err := validateSourcePrincipalRow("/etc/gshadow", "ardents-source:!::", 123, 123); err != nil {
		t.Fatal(err)
	}
	for _, row := range []string{"ardents-source::20000::::::", "ardents-source:$6$foreign:20000::::::", "ardents-source:!:20000:1:::::", "other:!:20000::::::"} {
		if err := validateSourcePrincipalRow("/etc/shadow", row, 123, 123); err == nil {
			t.Fatal("foreign or unlocked current private row accepted", row)
		}
	}
}
