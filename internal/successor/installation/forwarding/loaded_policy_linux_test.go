package forwarding

import (
	"encoding/json"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"testing"
)

func TestForwardingStartSelectorsRequireExactInvocationAndOwnJob(t *testing.T) {
	unit := systemd.Properties{"InvocationID": {Type: "ay", Data: json.RawMessage(`[9,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`)}, "Job": {Type: "(uo)", Data: json.RawMessage(`[73,"/org/freedesktop/systemd1/job/73"]`)}}
	service := systemd.Properties{"MainPID": {Type: "u", Data: json.RawMessage(`7317`)}}
	pid, id, err := runningInvocation(unit, service)
	if err != nil || pid != 7317 || id != [16]byte{9} {
		t.Fatal("exact detached invocation grammar refused", err)
	}
	for _, raw := range []string{`[]`, `[9]`, `null`, `"CQAAAAAAAAAAAAAAAAAAAA=="`, `[9,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,1]`, `[256,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`, `[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`} {
		unit["InvocationID"] = systemd.Value{Type: "ay", Data: json.RawMessage(raw)}
		if _, _, err := runningInvocation(unit, service); err == nil {
			t.Fatal("truncated/foreign invocation admitted", raw)
		}
	}
	if !ownQueuedJob(unit, "/org/freedesktop/systemd1/job/73") {
		t.Fatal("matching original queued job refused")
	}
	for _, object := range []string{"", "/", "/org/freedesktop/systemd1/job/0", "/org/freedesktop/systemd1/job/073", "/org/freedesktop/systemd1/job/74", "/org/freedesktop/systemd1/job/-73"} {
		if ownQueuedJob(unit, object) {
			t.Fatal("foreign or noncanonical job admitted", object)
		}
	}
}
