package stoppedunit

import (
	"encoding/json"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/platform/systemd/observation"
)

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
	roles := []struct{ name string }{{"forwarding"}, {"introduction"}, {"join"}, {"descriptor"}}
	for _, entry := range roles {
		t.Run(entry.name, func(t *testing.T) {
			unit, service := nodeDutyRepairPolicyFixture(t, entry.name)
			if !NodeStopped(unit, service, []string{"state", "participation"}, entry.name) {
				t.Fatal("valid stopped policy refused")
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
			if !NodeStopped(unit, service, []string{"state", "participation"}, entry.name) {
				t.Fatal("exact per-unit ControlGroup refused")
			}
			service["ControlGroup"] = before
			// A sibling role's genuine stopped facts are never this role's proof.
			for _, sibling := range roles {
				if sibling.name == entry.name {
					continue
				}
				otherUnit, otherService := nodeDutyRepairPolicyFixture(t, sibling.name)
				if NodeStopped(otherUnit, otherService, []string{"state", "participation"}, entry.name) {
					t.Fatal("sibling role facts admitted:", sibling.name)
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
					if NodeStopped(unit, service, []string{"state", "participation"}, entry.name) {
						t.Fatal("unsafe stopped policy accepted")
					}
				})
			}
		})
	}
}

func TestNodeDutyRepairDeclarationRoleEquivalence(t *testing.T) {
	const program = "/usr/lib/ardents/runtime/ardents-next"
	const plan = "/etc/ardents/node-role.json"
	roles := []struct{ name string }{{"forwarding"}, {"introduction"}, {"join"}, {"descriptor"}}
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
				{"traversing-program", "/usr/lib/ardents/../foreign", plan, entry.name, []string{"no-env-expand"}, false},
				{"noncanonical-program", "/usr/lib/ardents//worker", plan, entry.name, []string{"no-env-expand"}, false},
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
					if NodeDeclaration(facts, plan, entry.name) != test.accepted {
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
						if NodeDeclaration(siblingFacts, plan, entry.name) {
							t.Fatal("sibling role declaration admitted:", sibling.name)
						}
					}
				})
			}
		})
	}
}
