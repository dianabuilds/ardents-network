package forwarding

import (
	"bytes"
	"strings"
	"testing"
)

// Independent canonical byte fixture only: the invented native identities and
// manager string cannot supply a successful fresh observation or recovery.
func nativeContextByteVector() []byte {
	return []byte(`{"schema":"ardents-forwarding-native-context-v1","manager":"physical-test-only","process":{"path":"/proc/1","device":1,"inode":2,"mode":2147484013,"uid":0,"gid":0},"started":1,"boot_sha256":"` + strings.Repeat("a", 64) + `","namespaces":[{"path":"/proc/self/ns/mnt","device":1,"inode":3,"mode":292,"uid":0,"gid":0},{"path":"/proc/self/ns/pid","device":1,"inode":4,"mode":292,"uid":0,"gid":0},{"path":"/proc/self/ns/cgroup","device":1,"inode":5,"mode":292,"uid":0,"gid":0},{"path":"/proc/self/ns/uts","device":1,"inode":6,"mode":292,"uid":0,"gid":0},{"path":"/proc/self/ns/user","device":1,"inode":7,"mode":292,"uid":0,"gid":0}],"unit_paths":["/run/systemd/generator"],"ancestors":[{"path":"/","device":1,"inode":8,"mode":2147484141,"uid":0,"gid":0},{"path":"/run/systemd","device":1,"inode":9,"mode":2147484141,"uid":0,"gid":0},{"path":"/run/systemd/generator","device":1,"inode":101,"mode":2147484141,"uid":0,"gid":0}],"absent":["/run/systemd/generator.early","/run/systemd/generator.late"]}` + "\n")
}

func TestForwardingReloadContextPreservesEveryNonGeneratorBinding(t *testing.T) {
	original := nativeContextByteVector()
	current := bytes.Replace(original, []byte(`"inode":101,`), []byte(`"inode":102,`), 1)
	if _, err := decodeNativeContext(original); err != nil {
		t.Fatal("independent original context", err)
	}
	if err := compareReloadContexts(original, current); err != nil {
		t.Fatal("own generator inode refresh unavailable", err)
	}
	for _, substitution := range [][2]string{
		{`"manager":"physical-test-only"`, `"manager":"another-manager"`},
		{`"started":1`, `"started":2`},
		{`"inode":8,`, `"inode":88,`},
		{`"inode":9,`, `"inode":99,`},
		{`"inode":3,`, `"inode":33,`},
		{`"boot_sha256":"a`, `"boot_sha256":"b`},
		{`"/run/systemd/generator.early","/run/systemd/generator.late"`, `"/run/systemd/generator.early","/run/systemd/generator.late","/unrelated-absence"`},
		{`"inode":102,"mode":2147484141,"uid":0`, `"inode":102,"mode":2147484141,"uid":999`},
		{`"inode":102,"mode":2147484141`, `"inode":102,"mode":493`},
	} {
		bad := bytes.Replace(current, []byte(substitution[0]), []byte(substitution[1]), 1)
		if bytes.Equal(bad, current) || compareReloadContexts(original, bad) == nil {
			t.Fatal("unowned native context change accepted", substitution)
		}
	}
}

func TestForwardingChangedContextRequiresOriginalCommitAndVerifiedReloadBoundary(t *testing.T) {
	original := nativeContextByteVector()
	current := bytes.Replace(original, []byte(`"inode":101,`), []byte(`"inode":102,`), 1)
	message := transitionMessage{Operation: "initial", OriginalContext: original, NativeContext: current}
	records := []initialRecord{{Phase: "native-context", SHA256: digest(original)}, {Phase: "reload-intent"}, {Phase: "stopped-verified"}}
	if err := checkTransitionContexts(message, records); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"reload-intent", "file-ready", "ack-written", "retired"} {
		bad := append([]initialRecord(nil), records...)
		bad[2].Phase = phase
		if checkTransitionContexts(message, bad) == nil {
			t.Fatal("outstanding effect supplied checked reload", phase)
		}
	}
	for _, index := range []int{0, 1} {
		bad := append([]initialRecord(nil), records...)
		bad[index].Phase = "missing"
		if checkTransitionContexts(message, bad) == nil {
			t.Fatal("missing original context/reload commitment accepted")
		}
	}
	message.Operation = "start"
	if checkTransitionContexts(message, records) == nil {
		t.Fatal("start acquired reload context permission")
	}
}
