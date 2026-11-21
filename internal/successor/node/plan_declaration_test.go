package node

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
)

func testNodeNetwork() NodeNetwork {
	return NodeNetwork{Root: "/state", NetworkID: [32]byte{1}, Authorities: [][32]byte{{2}}, Threshold: 1, ProfileAuthority: [32]byte{3}, ClockObservationFile: "/clock"}
}

func testCoreMap() map[string]any {
	return map[string]any{
		"network": testNodeNetwork(), "node_id": [32]byte{9}, "process_profile": "ardents-forwarding-process-v1",
		"participation_root": "/participation", "spend_root": "/spend", "hosting_root": "/hosting",
		"certificate": "/cert.pem", "private_key": "/key.pem",
		"work": hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10},
		"retirement_wait_ms": 5000,
	}
}

func marshalTestPlan(t *testing.T, plan map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestNodePlanDecodeAcceptsFullRuntimeGrammar(t *testing.T) {
	raw := marshalTestPlan(t, testCoreMap())
	plan, err := DecodeForwardingPlan(raw)
	if err != nil {
		t.Fatal("full Forwarding plan refused:", err)
	}
	if plan.Network == nil || plan.Network.Root != "/state" || plan.NodeID != [32]byte{9} || plan.RetirementWaitMS != 5000 || plan.Work.Tx != 32<<20 {
		t.Fatalf("decoded facts diverge: %+v", plan)
	}
	if _, err := DecodeJoinPlan(raw); err != nil {
		t.Fatal("full JOIN plan refused:", err)
	}
	descriptor := testCoreMap()
	descriptor["descriptor_root"] = "/store"
	if p, err := DecodeDescriptorPlan(marshalTestPlan(t, descriptor)); err != nil || p.DescriptorRoot != "/store" {
		t.Fatal("full Descriptor plan refused:", err)
	}
	introduction := testCoreMap()
	introduction["introduction_root"] = "/registry"
	if p, err := DecodeIntroductionPlan(marshalTestPlan(t, introduction)); err != nil || p.IntroductionRoot != "/registry" {
		t.Fatal("full Introduction plan refused:", err)
	}
	issuer := testCoreMap()
	issuer["issuance"] = map[string]any{"result_root": "/results"}
	issuer["bootstrap_work"] = hosting.Traffic{Tx: 1, Rx: 2}
	if p, err := DecodeIssuerPlan(marshalTestPlan(t, issuer)); err != nil || p.BootstrapWork.Rx != 2 || len(p.Issuance) == 0 {
		t.Fatal("full Issuer plan refused:", err)
	}
}

// The existing runtime grammar admits omitted zero-valued members; the command
// policy refuses incomplete declarations before effect. The codec must keep
// accepting the same partial byte grammar the runtime accepted before.
func TestNodePlanDecodePreservesOmittedMemberGrammar(t *testing.T) {
	partial := map[string]any{
		"network": testNodeNetwork(), "process_profile": "ardents-forwarding-process-v1",
		"participation_root": "/participation", "spend_root": "/spend", "hosting_root": "/hosting", "retirement_wait_ms": 100,
	}
	plan, err := DecodeForwardingPlan(marshalTestPlan(t, partial))
	if err != nil {
		t.Fatal("partial runtime plan refused:", err)
	}
	if plan.NodeID != [32]byte{} || plan.Certificate != "" || plan.Work.Tx != 0 {
		t.Fatalf("omitted members did not decode to zero: %+v", plan)
	}
}

func TestNodePlanDecodeEnforcesExactRoleFieldSet(t *testing.T) {
	foreign := testCoreMap()
	foreign["descriptor_root"] = "/store"
	if _, err := DecodeForwardingPlan(marshalTestPlan(t, foreign)); err == nil {
		t.Fatal("Forwarding accepted a Descriptor extension field")
	}
	if _, err := DecodeJoinPlan(marshalTestPlan(t, foreign)); err == nil {
		t.Fatal("JOIN accepted a Descriptor extension field")
	}
	descriptor := testCoreMap()
	descriptor["introduction_root"] = "/registry"
	if _, err := DecodeDescriptorPlan(marshalTestPlan(t, descriptor)); err == nil {
		t.Fatal("Descriptor accepted an Introduction extension field")
	}
	issuer := testCoreMap()
	issuer["descriptor_root"] = "/store"
	if _, err := DecodeIssuerPlan(marshalTestPlan(t, issuer)); err == nil {
		t.Fatal("Issuer accepted a Descriptor extension field")
	}
}

func TestNodePlanDecodeRefusesDuplicateNullAndBadShapes(t *testing.T) {
	good := string(marshalTestPlan(t, testCoreMap()))
	for _, raw := range []string{
		strings.Replace(good, `"retirement_wait_ms":5000`, `"retirement_wait_ms":5000,"retirement_wait_ms":1`, 1),
		strings.Replace(good, `"certificate":"/cert.pem"`, `"certificate":null`, 1),
		strings.Replace(good, `"private_key":"/key.pem"`, `"private_key":"/key.pem","unknown":1`, 1),
		`{"network":null}`,
		good + ` {}`,
		``,
	} {
		if _, err := DecodeForwardingPlan([]byte(raw)); err == nil {
			t.Fatalf("refused grammar accepted: %s", raw)
		}
	}
	shortID := testCoreMap()
	shortID["node_id"] = [3]byte{1, 2, 3}
	if _, err := DecodeForwardingPlan(marshalTestPlan(t, shortID)); err == nil {
		t.Fatal("short node_id array accepted")
	}
	badTraffic := testCoreMap()
	badTraffic["work"] = map[string]any{"tx": 1, "rx": 2, "extra": 3}
	if _, err := DecodeForwardingPlan(marshalTestPlan(t, badTraffic)); err == nil {
		t.Fatal("unknown Traffic member accepted")
	}
	badNet := testCoreMap()
	badNet["network"] = map[string]any{
		"root": "/state", "network_id": [32]byte{1}, "authorities": json.RawMessage(`[[1,2]]`),
		"threshold": 1, "profile_authority": [32]byte{3}, "clock_observation_file": "/clock",
	}
	if _, err := DecodeForwardingPlan(marshalTestPlan(t, badNet)); err == nil {
		t.Fatal("short authority pin accepted")
	}
}

func TestDecodeNodeNetworkClosedGrammar(t *testing.T) {
	raw := marshalTestPlan(t, map[string]any{"root": "/state", "network_id": [32]byte{1}, "authorities": [][32]byte{{2}}, "threshold": 1, "profile_authority": [32]byte{3}, "clock_observation_file": "/clock"})
	network, err := DecodeNodeNetwork(raw)
	if err != nil || network.Root != "/state" || network.Threshold != 1 {
		t.Fatal("valid Network declaration refused:", err)
	}
	for _, bad := range []string{`{"root":"/state","root":"/other"}`, `{"root":null}`, `{"root":"/state","extra":1}`, `[]`, ``} {
		if _, err := DecodeNodeNetwork([]byte(bad)); err == nil {
			t.Fatalf("bad Network declaration accepted: %s", bad)
		}
	}
}
