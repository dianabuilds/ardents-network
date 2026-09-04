//go:build ignore

package main

import "testing"

func TestActionInputOwnsNoExecutionParameters(t *testing.T) {
	t.Parallel()
	request, err := decodeAction([]byte(`{"schema":"ardents-agent-action-v1","action":"refresh"}`))
	if err != nil || request.Action != "refresh" {
		t.Fatalf("decode refresh = %#v, %v", request, err)
	}
	if _, err := decodeAction([]byte(`{"schema":"ardents-agent-action-v1","action":"refresh","state_root":"tick-2"}`)); err == nil {
		t.Fatal("action input was allowed to override state_root")
	}
	if _, err := decodeAction([]byte(`{"schema":"ardents-agent-action-v1","action":"shell"}`)); err == nil {
		t.Fatal("arbitrary action was accepted")
	}
	if _, err := decodeAction([]byte(`{"schema":"ardents-agent-action-v1","action":"noop"} trailing`)); err == nil {
		t.Fatal("trailing action input was accepted")
	}
}
