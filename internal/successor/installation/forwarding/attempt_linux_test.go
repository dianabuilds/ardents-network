package forwarding

import (
	"errors"
	"fmt"
	"testing"
)

func TestForwardingAttemptDiscoveryIsClosedContiguousAndPurposeSpecific(t *testing.T) {
	for _, names := range [][]string{nil, {"source-startup"}, {"forwarding-invocation-00000001"},
		{"forwarding-startup", "forwarding-startup"}, {"forwarding-startup", "forwarding-invocation-00000002"},
		{"forwarding-startup", "forwarding-invocation-00000000"}, {"forwarding-startup", "forwarding-invocation-1"},
		{"forwarding-startup", "forwarding-invocation-00000064"}, {"forwarding-startup", "forwarding-invocation--0000001"},
		{"forwarding-startup", "source-invocation-00000001"}, {"forwarding-startup", "../forwarding-invocation-00000001"}} {
		if _, err := attemptDirectories(names); !errors.Is(err, ErrBinding) {
			t.Fatal("gapped/ambiguous/foreign attempt grammar accepted", names, err)
		}
	}
	names := []string{"forwarding-startup"}
	for n := 0; n < attemptLimit; n++ {
		if n > 0 {
			names = append(names, fmt.Sprintf("forwarding-invocation-%08d", n))
		}
		got, err := attemptDirectories(names)
		if err != nil || got != attemptNumber(n) {
			t.Fatal("complete original attempt prefix refused", n, got, err)
		}
	}
	names = append(names, "forwarding-invocation-00000064")
	if _, err := attemptDirectories(names); !errors.Is(err, ErrBinding) {
		t.Fatal("exhausted original history reset or grew past finite profile", err)
	}
	if attemptNumber(0).directory("/opt/node") != "/opt/node/forwarding-startup" || attemptNumber(1).directory("/opt/node") != "/opt/node/forwarding-invocation-00000001" || attemptNumber(0).journal("/opt/node") != "/opt/node.forwarding-first-start-journal" {
		t.Fatal("Forwarding attempt names reused another purpose")
	}
}
