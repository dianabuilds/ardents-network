package introduction

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestIntroductionInvocationRequiresContiguousHistory(t *testing.T) {
	for _, names := range [][]string{nil, {"introduction-invocation-00000001"}, {"introduction-startup", "introduction-invocation-00000002"}, {"introduction-startup", "introduction-startup"}, {"introduction-startup", "introduction-invocation-1"}, {"introduction-startup", "introduction-invocation-00000064"}, {"descriptor-startup"}} {
		if _, err := attemptDirectories(names); err == nil {
			t.Fatal("foreign, duplicate or incomplete history accepted", names)
		}
	}
	names := []string{"introduction-startup"}
	for n := 1; n < attemptLimit; n++ {
		names = append(names, fmt.Sprintf("introduction-invocation-%08d", n))
	}
	if n, err := attemptDirectories(names); err != nil || n != 63 {
		t.Fatal("finite complete namespace refused", n, err)
	}
	if attemptNumber(attemptLimit).directory("/opt/node") != "" {
		t.Fatal("exhausted namespace recycled")
	}
}

func TestIntroductionInvocationIntentBindsRetiredPredecessor(t *testing.T) {
	b := binding{UID: 991, GID: 991}
	bindingSHA, initial, targets, previous := strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("d", 64)
	raw, err := encodeStartIntent(b, bindingSHA, initial, targets, 1, previous)
	want := fmt.Sprintf(`{"schema":"ardents-introduction-invocation-intent-v1","binding_sha256":"%s","initial_history_sha256":"%s","targets_sha256":"%s","uid":991,"gid":991,"attempt":1,"predecessor_sha256":"%s"}`, bindingSHA, initial, targets, previous) + "\n"
	if err != nil || string(raw) != want || checkStartIntent(raw, b, bindingSHA, initial, 1, previous) != nil {
		t.Fatal("independent invocation intent differs", err)
	}
	for _, bad := range [][]byte{bytes.Replace(raw, []byte(previous), []byte(targets), 1), bytes.Replace(raw, []byte(`"attempt":1`), []byte(`"attempt":2`), 1), bytes.Replace(raw, []byte("introduction-invocation"), []byte("descriptor-invocation"), 1), append(bytes.Clone(raw), '\n')} {
		if checkStartIntent(bad, b, bindingSHA, initial, 1, previous) == nil {
			t.Fatal("foreign or noncanonical intent accepted")
		}
	}
	for _, n := range []attemptNumber{0, 1, 64} {
		predecessor := ""
		if n == 0 {
			predecessor = previous
		}
		if _, err := encodeStartIntent(b, bindingSHA, initial, targets, n, predecessor); err == nil {
			t.Fatal("missing retirement or invalid first intent accepted", n)
		}
	}
	first, err := encodeStartIntent(b, bindingSHA, initial, targets, 0, "")
	if err != nil || checkStartIntent(first, b, bindingSHA, initial, 0, "") != nil || checkStartIntent(first, b, bindingSHA, initial, 1, previous) == nil {
		t.Fatal("first history reclassified as restart", err)
	}
}
