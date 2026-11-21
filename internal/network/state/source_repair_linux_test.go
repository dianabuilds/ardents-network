package state

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/platform/installation/systemd"
)

// Detached policy fixtures grant no manager/cgroup proof. The public adapter
// cannot receive these dictionaries and never accepts a zero native lifetime.
func TestSourceRepairStoppedPolicyRefusesUnsafeNativeFacts(t *testing.T) {
	property := func(signature string, value any) systemd.Value {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return systemd.Value{Type: signature, Data: raw}
	}
	unit := systemd.Properties{}
	service := systemd.Properties{}
	for name, value := range map[string]string{"Id": "ardents-source.service", "LoadState": "loaded", "FragmentPath": "/etc/systemd/system/ardents-source.service", "ActiveState": "failed", "SubState": "failed"} {
		unit[name] = property("s", value)
	}
	unit["DropInPaths"], unit["TriggeredBy"] = property("as", []string{}), property("as", []string{})
	for name, value := range map[string]string{"User": "ardents-source", "Group": "ardents-source", "KillMode": "control-group", "Restart": "no", "ProtectSystem": "strict", "ControlGroup": ""} {
		service[name] = property("s", value)
	}
	service["MainPID"], service["ControlPID"] = property("u", uint32(0)), property("u", uint32(0))
	service["NoNewPrivileges"], service["ProtectControlGroups"] = property("b", true), property("b", true)
	for _, name := range []string{"SendSIGKILL", "DynamicUser", "Delegate"} {
		service[name] = property("b", false)
	}
	service["ReadWritePaths"] = property("as", []string{"state", "participation"})
	if err := sourceRepairStoppedPolicy(unit, service, "state", "participation"); err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct {
		service         bool
		name, signature string
		value           any
	}{
		{false, "ActiveState", "s", "active"}, {false, "SubState", "s", "stop"},
		{false, "DropInPaths", "as", []string{"override.conf"}}, {false, "TriggeredBy", "as", []string{"ardents-source.socket"}},
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
			if sourceRepairStoppedPolicy(unit, service, "state", "participation") == nil {
				t.Fatal("unsafe stopped policy accepted")
			}
		})
	}
}

func TestSourceRepairCannotManufactureNativeQuiescence(t *testing.T) {
	for _, original := range []*systemd.SourceRuntime{nil, {}} {
		if checkSourceRepairQuiescence(context.Background(), original, config{}, "") == nil {
			t.Fatal("absent original supplied native proof")
		}
	}
	if RepairRetainedSourceProducer(context.Background(), Config{}, nil, "") == nil {
		t.Fatal("absent original authorized repair")
	}
}

func TestSourceRepairBindsLoadedRetainedDeclaration(t *testing.T) {
	const program = "/usr/lib/ardents/runtime/ardents-next"
	const plan = "/etc/ardents/source.json"
	for _, test := range []struct {
		name, program, plan, operation string
		flags                          []string
		accepted                       bool
	}{
		{"same", program, plan, "serve-retained", []string{"no-env-expand"}, true},
		{"foreign-plan", program, "/etc/ardents/other.json", "serve-retained", []string{"no-env-expand"}, false},
		{"fresh", program, plan, "serve", []string{"no-env-expand"}, false},
		{"foreign-program", "/tmp/program", plan, "serve-retained", []string{"no-env-expand"}, false},
		{"privileged", program, plan, "serve-retained", []string{"privileged", "no-env-expand"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			row := []any{test.program, []string{test.program, "source", test.operation, test.plan}, test.flags, 0, 0, 0, 0, 0, 0, 0}
			raw, err := json.Marshal([]any{row})
			if err != nil {
				t.Fatal(err)
			}
			facts := systemd.Properties{"ExecStartEx": {Type: "a(sasasttttuii)", Data: raw}}
			if sourceRepairDeclaration(facts, plan) != test.accepted {
				t.Fatal("wrong loaded declaration admission")
			}
		})
	}
}
