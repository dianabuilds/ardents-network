package networkdraft

import (
	"fmt"
	"strings"
	"testing"
)

func TestPublicRecordPlanDoesNotAdmitSigningPurpose(t *testing.T) {
	raw := []byte(fmt.Sprintf(`{"schema":"ardents-initial-node-record-plan-v1",
		"network_id":"01%s","node_id":"02%s","valid_from":"2033-05-22T05:20:00Z",
		"valid_until":"2033-05-22T06:20:00Z","family":"family-one","endpoint":"127.0.0.1:4401",
		"carrier":"ardents-carrier-tcp-tls-v2","capacity":3,"public_key":"%s"}`,
		strings.Repeat("00", 31), strings.Repeat("00", 31), strings.Repeat("03", 32)))
	name, unsigned, err := Prepare("prepare-initial-node-record", raw)
	if err != nil || name != "node-record.signing-input" || len(unsigned) == 0 {
		t.Fatalf("valid public plan: name=%q bytes=%d err=%v", name, len(unsigned), err)
	}
	if name, signed, err := Prepare("sign-initial-node-record", raw); err == nil || name != "" || signed != nil {
		t.Fatal("public preparation accepted the separate signing purpose")
	}
}
