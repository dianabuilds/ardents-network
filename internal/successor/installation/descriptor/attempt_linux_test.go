package descriptor

import (
	"errors"
	"fmt"
	"testing"
)

func TestDescriptorAttemptDiscoveryIsClosedContiguousAndPurposeSpecific(t *testing.T) {
	for _, names := range [][]string{nil, {"source-startup"}, {"descriptor-invocation-00000001"},
		{"descriptor-startup", "descriptor-startup"}, {"descriptor-startup", "descriptor-invocation-00000002"},
		{"descriptor-startup", "descriptor-invocation-00000000"}, {"descriptor-startup", "descriptor-invocation-1"},
		{"descriptor-startup", "descriptor-invocation-00000064"}, {"descriptor-startup", "descriptor-invocation--0000001"},
		{"descriptor-startup", "source-invocation-00000001"}, {"descriptor-startup", "../descriptor-invocation-00000001"}} {
		if _, err := attemptDirectories(names); !errors.Is(err, ErrBinding) {
			t.Fatal("gapped/ambiguous/foreign attempt grammar accepted", names, err)
		}
	}
	names := []string{"descriptor-startup"}
	for n := 0; n < attemptLimit; n++ {
		if n > 0 {
			names = append(names, fmt.Sprintf("descriptor-invocation-%08d", n))
		}
		got, err := attemptDirectories(names)
		if err != nil || got != attemptNumber(n) {
			t.Fatal("complete original attempt prefix refused", n, got, err)
		}
	}
	names = append(names, "descriptor-invocation-00000064")
	if _, err := attemptDirectories(names); !errors.Is(err, ErrBinding) {
		t.Fatal("exhausted original history reset or grew past finite profile", err)
	}
	if attemptNumber(0).directory("/opt/node") != "/opt/node/descriptor-startup" || attemptNumber(1).directory("/opt/node") != "/opt/node/descriptor-invocation-00000001" || attemptNumber(0).journal("/opt/node") != "/opt/node.descriptor-first-start-journal" {
		t.Fatal("Descriptor attempt names reused another purpose")
	}
}
