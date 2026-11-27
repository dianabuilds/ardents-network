//go:build linux

package command

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint"
)

func TestPermissionProgressHasSafeIdentityWithoutLosingLocalExchange(t *testing.T) {
	for _, phase := range []string{"permission-pending", "permission-accepted"} {
		var out bytes.Buffer
		if err := reportInstalledProgress(&out, endpoint.Progress{Phase: phase, Role: "private-role", Digest: [32]byte{1}}); err != nil {
			t.Fatal(err)
		}
		var local map[string]string
		if err := json.Unmarshal(out.Bytes(), &local); err != nil || local["request_digest"] == "" || local["role"] != "private-role" {
			t.Fatal("permission workflow changed", err)
		}
		record, present, err := runtimeevent.Decode(out.Bytes())
		if err != nil || !present || record.Kind != "endpoint.start-installed" || record.Phase != phase {
			t.Fatal("progress not recognized", record, err)
		}
		safe, _ := json.Marshal(record)
		if strings.Contains(string(safe), "private-role") || strings.Contains(string(safe), local["request_digest"]) {
			t.Fatal("permission identity escaped projection")
		}
	}
	var out bytes.Buffer
	if err := reportInstalledProgress(&out, endpoint.Progress{Phase: "PRIVATE-unknown", Role: "private-role"}); err == nil || out.Len() != 0 {
		t.Fatal("unknown progress exported")
	}
}
