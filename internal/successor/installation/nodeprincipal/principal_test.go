package nodeprincipal

import "testing"

func TestFixedPrincipalValidation(t *testing.T) {
	passwd := []byte("root:x:0:0:root:/root:/bin/bash\nardents-node:x:999:999::/nonexistent:/usr/sbin/nologin\n")
	group := []byte("root:x:0:\nardents-node:x:999:\n")
	nss := []byte("passwd: files systemd\ngroup: files systemd\n")
	if _, err := ValidatePublic(passwd, group, nss, 999, 999); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidatePublic(passwd, group, []byte("passwd: files ldap\ngroup: files\n"), 999, 999); err == nil {
		t.Fatal("foreign NSS admitted")
	}
	if _, err := ValidatePublic(passwd, append(group, []byte("other:x:999:\n")...), nss, 999, 999); err == nil {
		t.Fatal("duplicate group identity admitted")
	}
	if err := Delta("/etc/group", []byte("root:x:0:\n"), []byte("root:x:0:\nardents-node:x:999:\n"), 999, 999); err != nil {
		t.Fatal(err)
	}
	if err := Delta("/etc/group", []byte("root:x:0:\n"), []byte("root:x:1:\nardents-node:x:999:\n"), 999, 999); err == nil {
		t.Fatal("foreign bytes changed")
	}
}

func TestSelectIDUsesBothDatabasesAndRefusesExistingNode(t *testing.T) {
	passwd := []byte("root:x:0:0:root:/root:/bin/bash\nother:x:998:999::/tmp:/bin/false\n")
	group := []byte("root:x:0:\nother:x:997:\n")
	id, err := SelectID(passwd, group)
	if err != nil || id != 996 {
		t.Fatalf("selected %d: %v", id, err)
	}
	if _, err := SelectID(append(passwd, []byte("ardents-node:x:996:996::/nonexistent:/usr/sbin/nologin\n")...), group); err == nil {
		t.Fatal("existing account admitted")
	}
	if _, err := SelectID([]byte("bad:x:not-a-number:0::/:/bin/false\n"), group); err == nil {
		t.Fatal("malformed identity admitted")
	}
}
