package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestForwardingStartupReportsNetworkCleanupFailure(t *testing.T) {
	var out bytes.Buffer
	if code := nodeStartupRefusal(&out, nodeForwardingOperation, errors.New("private startup error"), errors.New("private cleanup error")); code != 1 {
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
