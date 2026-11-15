package journal

import "testing"

func TestInvocationPrefixRejectsForeignGapsAndAmbiguity(t *testing.T) {
	for _, names := range [][]string{nil, {"join-invocation-00000001"}, {"join-startup", "join-startup"}, {"join-startup", "join-invocation-00000002"}, {"join-startup", "descriptor-invocation-00000001"}, {"join-startup", "join-invocation-1"}, {"join-startup", "join-invocation-00000064"}} {
		if _, err := InvocationPrefix(names, "join", 64); err == nil {
			t.Fatalf("accepted invalid original namespace %q", names)
		}
	}
	if got, err := InvocationPrefix([]string{"descriptor-invocation-00000001", "descriptor-startup"}, "descriptor", 64); err != nil || got != 1 {
		t.Fatalf("valid reordered original namespace: %d %v", got, err)
	}
	if _, err := InvocationPrefix([]string{"source-startup"}, "source", 64); err == nil {
		t.Fatal("unadmitted purpose accepted")
	}
}

func TestInvocationPrefixKeepsEachPurposeSeparate(t *testing.T) {
	for _, purpose := range []string{"forwarding", "introduction"} {
		got, err := InvocationPrefix([]string{purpose + "-startup", purpose + "-invocation-00000001"}, purpose, 64)
		if err != nil || got != 1 {
			t.Fatalf("%s valid prefix: %d %v", purpose, got, err)
		}
		if _, err := InvocationPrefix([]string{purpose + "-startup", "join-invocation-00000001"}, purpose, 64); err == nil {
			t.Fatalf("%s accepted foreign invocation", purpose)
		}
	}
}
