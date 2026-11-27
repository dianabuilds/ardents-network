package state

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd/observation"
)

// Detached policy fixtures grant no manager/cgroup proof. The public adapter
// cannot receive these dictionaries and never accepts a zero native lifetime.
// These are the cross-role equivalence oracles for the shared node-duty native
// skeleton (W4, #711): every role binds exactly its own fixed unit, declaration
// and root count, refuses every sibling role's facts, and preserves the causal
// stopped-policy refusal matrix.

func nodeDutyRepairPolicyFixture(t *testing.T, role string) (observation.Properties, observation.Properties) {
	t.Helper()
	property := func(signature string, value any) observation.Value {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return observation.Value{Type: signature, Data: raw}
	}
	unitName := "ardents-" + role + ".service"
	unit := observation.Properties{}
	service := observation.Properties{}
	for name, value := range map[string]string{"Id": unitName, "LoadState": "loaded", "FragmentPath": "/etc/systemd/system/" + unitName, "ActiveState": "failed", "SubState": "failed"} {
		unit[name] = property("s", value)
	}
	unit["DropInPaths"], unit["TriggeredBy"] = property("as", []string{}), property("as", []string{})
	for name, value := range map[string]string{"User": "ardents-node", "Group": "ardents-node", "KillMode": "control-group", "Restart": "no", "ProtectSystem": "strict", "ControlGroup": ""} {
		service[name] = property("s", value)
	}
	service["MainPID"], service["ControlPID"] = property("u", uint32(0)), property("u", uint32(0))
	service["NoNewPrivileges"], service["ProtectControlGroups"] = property("b", true), property("b", true)
	for _, name := range []string{"SendSIGKILL", "DynamicUser", "Delegate"} {
		service[name] = property("b", false)
	}
	service["ReadWritePaths"] = property("as", []string{"state", "participation"})
	return unit, service
}

func TestNodeDutyRepairStoppedPolicyRoleEquivalence(t *testing.T) {
	roles := []nodeDutyRepairRole{forwardingRepairRole, introductionRepairRole, joinRepairRole, descriptorRepairRole}
	for _, entry := range roles {
		t.Run(entry.name, func(t *testing.T) {
			unit, service := nodeDutyRepairPolicyFixture(t, entry.name)
			if err := nodeDutyRepairStoppedPolicy(unit, service, []string{"state", "participation"}, entry); err != nil {
				t.Fatal(err)
			}
			// The exact per-unit cgroup path is admitted alongside the empty one.
			property := func(signature string, value any) observation.Value {
				raw, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				return observation.Value{Type: signature, Data: raw}
			}
			before := service["ControlGroup"]
			service["ControlGroup"] = property("s", "/system.slice/ardents-"+entry.name+".service")
			if err := nodeDutyRepairStoppedPolicy(unit, service, []string{"state", "participation"}, entry); err != nil {
				t.Fatal("exact per-unit ControlGroup refused:", err)
			}
			service["ControlGroup"] = before
			// A sibling role's genuine stopped facts are never this role's proof.
			for _, sibling := range roles {
				if sibling.name == entry.name {
					continue
				}
				otherUnit, otherService := nodeDutyRepairPolicyFixture(t, sibling.name)
				if err := nodeDutyRepairStoppedPolicy(otherUnit, otherService, []string{"state", "participation"}, entry); !errors.Is(err, entry.unavailable) {
					t.Fatal("sibling role facts admitted:", sibling.name, err)
				}
			}
			for _, change := range []struct {
				service         bool
				name, signature string
				value           any
			}{
				{false, "ActiveState", "s", "active"}, {false, "SubState", "s", "stop"},
				{false, "Id", "s", "ardents-other.service"}, {false, "FragmentPath", "s", "/etc/systemd/system/ardents-other.service"},
				{false, "DropInPaths", "as", []string{"override.conf"}}, {false, "TriggeredBy", "as", []string{"ardents-" + entry.name + ".socket"}},
				{true, "MainPID", "u", uint32(42)}, {true, "ControlPID", "u", uint32(42)}, {true, "MainPID", "s", "0"},
				{true, "ReadWritePaths", "as", []string{"another-state", "participation"}}, {true, "ControlGroup", "s", "/foreign.service"},
				{true, "User", "s", "root"}, {true, "KillMode", "s", "process"}, {true, "Restart", "s", "always"}, {true, "ProtectControlGroups", "b", false},
			} {
				t.Run(change.name+change.signature, func(t *testing.T) {
					target := unit
					if change.service {
						target = service
					}
					before := target[change.name]
					target[change.name] = property(change.signature, change.value)
					defer func() { target[change.name] = before }()
					if err := nodeDutyRepairStoppedPolicy(unit, service, []string{"state", "participation"}, entry); !errors.Is(err, entry.unavailable) {
						t.Fatal("unsafe stopped policy accepted:", err)
					}
				})
			}
		})
	}
}

func TestNodeDutyRepairDeclarationRoleEquivalence(t *testing.T) {
	const program = "/usr/lib/ardents/runtime/ardents-next"
	const plan = "/etc/ardents/node-role.json"
	roles := []nodeDutyRepairRole{forwardingRepairRole, introductionRepairRole, joinRepairRole, descriptorRepairRole}
	for _, entry := range roles {
		t.Run(entry.name, func(t *testing.T) {
			for _, test := range []struct {
				name, program, plan, operation string
				flags                          []string
				accepted                       bool
			}{
				{"same", program, plan, entry.name, []string{"no-env-expand"}, true},
				{"foreign-plan", program, "/etc/ardents/other.json", entry.name, []string{"no-env-expand"}, false},
				{"fresh", program, plan, "serve-retained", []string{"no-env-expand"}, false},
				{"foreign-program", "/tmp/program", plan, entry.name, []string{"no-env-expand"}, false},
				{"privileged", program, plan, entry.name, []string{"privileged", "no-env-expand"}, false},
			} {
				t.Run(test.name, func(t *testing.T) {
					row := []any{test.program, []string{test.program, "node", test.operation, "--retained-config", test.plan}, test.flags, 0, 0, 0, 0, 0, 0, 0}
					raw, err := json.Marshal([]any{row})
					if err != nil {
						t.Fatal(err)
					}
					facts := observation.Properties{"ExecStartEx": {Type: "a(sasasttttuii)", Data: raw}}
					// The caller-supplied declaration stays the canonical plan; a
					// unit loaded with a different plan must refuse (as in the
					// Source declaration oracle).
					if nodeDutyRepairDeclaration(facts, plan, entry) != test.accepted {
						t.Fatal("wrong loaded declaration admission")
					}
					// A sibling role's genuine declaration is never this role's proof.
					for _, sibling := range roles {
						if sibling.name == entry.name {
							continue
						}
						siblingRow := []any{program, []string{program, "node", sibling.name, "--retained-config", plan}, []string{"no-env-expand"}, 0, 0, 0, 0, 0, 0, 0}
						siblingRaw, err := json.Marshal([]any{siblingRow})
						if err != nil {
							t.Fatal(err)
						}
						siblingFacts := observation.Properties{"ExecStartEx": {Type: "a(sasasttttuii)", Data: siblingRaw}}
						if nodeDutyRepairDeclaration(siblingFacts, plan, entry) {
							t.Fatal("sibling role declaration admitted:", sibling.name)
						}
					}
				})
			}
		})
	}
}

func TestNodeDutyRepairCannotManufactureNativeQuiescence(t *testing.T) {
	if RepairRetainedForwardingProducer(context.Background(), Config{}, nil, "", nil) == nil {
		t.Fatal("absent original authorized Forwarding repair")
	}
	if RepairRetainedIntroductionProducer(context.Background(), Config{}, nil, "", nil) == nil {
		t.Fatal("absent original authorized Introduction repair")
	}
	if RepairRetainedJoinProducer(context.Background(), Config{}, nil, "", nil) == nil {
		t.Fatal("absent original authorized JOIN repair")
	}
	if RepairRetainedDescriptorProducer(context.Background(), Config{}, nil, "", nil) == nil {
		t.Fatal("absent original authorized Descriptor repair")
	}
	for _, original := range []*systemd.ForwardingRuntime{nil, {}} {
		if RepairRetainedForwardingProducer(context.Background(), Config{}, original, "", nil) == nil {
			t.Fatal("absent original supplied native proof")
		}
	}
}

func TestNodeDutyRepairDescriptorRootCountRefusal(t *testing.T) {
	// The fixed five-root count is enforced before any filesystem observation,
	// so the refusal is provable without the root-native parent chain.
	for _, count := range []int{0, 4, 6} {
		roots := make([]string, count)
		for i := range roots {
			roots[i] = "/var/lib/ardents/roles/descriptor/root-" + string(rune('a'+i))
		}
		if err := checkNodeDutyRepairRoots(descriptorRepairNative, roots); !errors.Is(err, ErrDescriptorRepairUnavailable) {
			t.Fatal("wrong Descriptor writable-root count accepted:", count, err)
		}
	}
}
