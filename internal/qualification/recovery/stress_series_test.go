package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestVerifiedProgressRejectsTerminalDeliveryStall(t *testing.T) {
	values := make([]progressSample, 61)
	for index := range values {
		values[index] = progressSample{AtNanos: int64(index) * int64(time.Second), Delivered: uint32(index) * 1024}
	}
	if _, result := verifiedProgress(values, int64(time.Minute)); result.Verdict != "pass" {
		t.Fatalf("valid progress rejected: %+v", result)
	}
	for index := 54; index < len(values); index++ {
		values[index].Delivered = values[53].Delivered
	}
	if _, result := verifiedProgress(values, int64(time.Minute)); result.Verdict != "fail" {
		t.Fatalf("terminal stall verdict=%+v; want fail", result)
	}
}

func TestVerifyShapersBindsIndependentConfigsAndConfinement(t *testing.T) {
	const image = "sha256:tool"
	values := make([]shaperEvidence, 2)
	for index, role := range []string{"shape-client", "shape-publisher"} {
		config := json.RawMessage(fmt.Sprintf(`{"schema_version":"carrier-lab-native-tool-role/v1","run_id":"s43-%d","role":"%s","mode":"shape","profile":"h3-s43-impaired-v1","seed":%d}`,
			index, role, index+1))
		digest := sha256.Sum256(config)
		result := json.RawMessage(fmt.Sprintf(`{"schema_version":"carrier-lab-native-tool-role/v1","run_id":"s43-%d","role":"%s","status":"passed","effective_capabilities":"0000000000001000","tool_lock_sha256":"%s","tools":{"tc":{"version":"1","path":"/usr/sbin/tc","sha256":"%s"}},"qdisc":{"eth0":"qdisc netem limit 1000 delay 150ms loss 5%% rate 25mbit"}}`,
			index, role, hex.EncodeToString(make([]byte, 32)), hex.EncodeToString(make([]byte, 32))))
		target := fmt.Sprintf("target-%d", index)
		values[index] = shaperEvidence{Role: role, ContainerID: fmt.Sprintf("shape-%d", index),
			TargetContainer: target, ToolImageID: image, ConfigDigest: hex.EncodeToString(digest[:]),
			ReadyObservedAtNanos: 1, CompletedAtNanos: 61, Config: config, Result: result, Removed: true,
			Observer: ObserverProcess{ContainerID: fmt.Sprintf("shape-%d", index), ImageID: image,
				NetworkMode: "container:" + target, User: "0:0", IPCMode: "private", ReadOnly: true,
				CapAdd: []string{"NET_ADMIN"}, CapDrop: []string{"ALL"},
				SecurityOpt: []string{"no-new-privileges:true"}, MountCount: 4, PidsLimit: 16,
				MemoryLimit: 32 << 20, NanoCPUs: 250_000_000}}
	}
	if result := verifyShapers(values, image, hostScopeEvidence{Adapter: "docker-compose-v1"}, 1, 60); result.Verdict != "pass" {
		t.Fatalf("valid shapers rejected: %+v", result)
	}
	values[1].Config = values[0].Config
	if result := verifyShapers(values, image, hostScopeEvidence{Adapter: "docker-compose-v1"}, 1, 60); result.Verdict != "invalid" {
		t.Fatalf("reused shaping seed verdict=%+v; want invalid", result)
	}
}
