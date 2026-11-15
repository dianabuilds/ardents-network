package join

import (
	"errors"
	"fmt"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
)

func TestJoinAttemptDiscoveryIsClosedContiguousAndPurposeSpecific(t *testing.T) {
	for _, names := range [][]string{nil, {"source-startup"}, {"join-invocation-00000001"},
		{"join-startup", "join-startup"}, {"join-startup", "join-invocation-00000002"},
		{"join-startup", "join-invocation-00000000"}, {"join-startup", "join-invocation-1"},
		{"join-startup", "join-invocation-00000064"}, {"join-startup", "join-invocation--0000001"},
		{"join-startup", "source-invocation-00000001"}, {"join-startup", "../join-invocation-00000001"}} {
		if _, err := journal.InvocationPrefix(names, "join", attemptLimit); !errors.Is(err, journal.ErrBinding) {
			t.Fatal("gapped/ambiguous/foreign attempt grammar accepted", names, err)
		}
	}
	names := []string{"join-startup"}
	for n := 0; n < attemptLimit; n++ {
		if n > 0 {
			names = append(names, fmt.Sprintf("join-invocation-%08d", n))
		}
		got, err := journal.InvocationPrefix(names, "join", attemptLimit)
		if err != nil || got != uint32(n) {
			t.Fatal("complete original attempt prefix refused", n, got, err)
		}
	}
	names = append(names, "join-invocation-00000064")
	if _, err := journal.InvocationPrefix(names, "join", attemptLimit); !errors.Is(err, journal.ErrBinding) {
		t.Fatal("exhausted original history reset or grew past finite profile", err)
	}
	if attemptNumber(0).directory("/opt/node") != "/opt/node/join-startup" || attemptNumber(1).directory("/opt/node") != "/opt/node/join-invocation-00000001" || attemptNumber(0).journal("/opt/node") != "/opt/node.join-first-start-journal" {
		t.Fatal("Join attempt names reused another purpose")
	}
}
