package command

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/domain/node"
	"testing"
)

func TestForwardingStartupReportsNetworkCleanupFailure(t *testing.T) {
	var out bytes.Buffer
	if code := startupRefusal(&out, forwardingOperation, errors.New("private startup error"), errors.New("private cleanup error")); code != 1 {
		t.Fatal("startup refusal returned wrong result", code)
	}
	var event map[string]string
	if err := json.Unmarshal(out.Bytes(), &event); err != nil || event["outcome"] != "startup-unavailable" || event["cleanup_owner"] != "network" {
		t.Fatal("cleanup failure not retained in public diagnostic", event, err)
	}
	if bytes.Contains(out.Bytes(), []byte("private")) {
		t.Fatal("diagnostic disclosed private error text")
	}
}

func TestForwardingWaitExpiryKeepsJoinedOutcome(t *testing.T) {
	var out bytes.Buffer
	firstFailure := func() *node.FailureDiagnostic { return nil }
	reportWaitExpired(&out, forwardingOperation, firstFailure)
	var event map[string]string
	if err := json.Unmarshal(out.Bytes(), &event); err != nil || event["phase"] != "retiring" || event["outcome"] != "completion-unavailable" {
		t.Fatalf("invalid wait expiry report: %v, %v", event, err)
	}

	out.Reset()
	result := node.InvocationCompletion{Outcome: "canceled", TimedOut: true}
	if code := reportCompletion(&out, forwardingOperation, result, firstFailure); code != 1 || out.Len() != 0 {
		t.Fatalf("timed-out Forwarding projected another report: %d, %q", code, out.String())
	}
	if code := reportCompletion(&out, "node.join", result, firstFailure); code != 1 || !bytes.Contains(out.Bytes(), []byte(`"phase":"joined"`)) {
		t.Fatalf("timed-out JOIN lost the joined report: %d, %q", code, out.String())
	}
}
