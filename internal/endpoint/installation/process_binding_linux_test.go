//go:build linux

package installation

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/endpoint/worker"
)

func TestInstalledProcessBindingRefusesForeignInvocationAndExecution(t *testing.T) {
	root, files, _ := bindingBytesFixture(t)
	checked, err := readLocalBinding(root, fixtureReader(files))
	if err != nil {
		t.Fatal(err)
	}
	invocation := [16]byte{1}
	unit, service := installedProcessPropertiesFixture(t, checked, invocation)
	if err := verifyInstalledProcessVersion(255, unit, service, checked, 42, invocation); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct {
		name, key, signature string
		unit                 bool
		value                any
	}{
		{"foreign PID", "MainPID", "u", false, uint32(43)},
		{"root", "User", "s", false, "root"},
		{"foreign invocation", "InvocationID", "ay", true, [16]byte{2}},
		{"missing invocation", "InvocationID", "ay", true, nil},
		{"drop-in", "DropInPaths", "as", true, []string{"/run/override.conf"}},
		{"foreign group", "ControlGroup", "s", false, "/system.slice/other.service"},
		{"privilege", "NoNewPrivileges", "b", false, false},
		{"capability", "CapabilityBoundingSet", "t", false, uint64(1)},
		{"additional writable root", "ReadWritePaths", "as", false, []string{"/"}},
		{"extra command", "ExecStartPost", "a(sasbttttuii)", false, []any{[]any{"/bin/true"}}},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			changedUnit, changedService := cloneProperties(unit), cloneProperties(service)
			target := changedService
			if mutation.unit {
				target = changedUnit
			}
			body, err := json.Marshal(mutation.value)
			if err != nil {
				t.Fatal(err)
			}
			target[mutation.key] = worker.Value{Type: mutation.signature, Data: body}
			if err := verifyInstalledProcessVersion(255, changedUnit, changedService, checked, 42, invocation); err == nil {
				t.Fatal("foreign process binding accepted")
			}
		})
	}
	for _, change := range []struct{ from, to string }{{"start-installed", "headless"}, {"no-env-expand", "privileged"}, {",42,", ",43,"}, {",1,1,", ",1,0,"}} {
		changed := cloneProperties(service)
		value := changed["ExecStartEx"]
		value.Data = json.RawMessage(strings.Replace(string(value.Data), change.from, change.to, 1))
		changed["ExecStartEx"] = value
		if err := verifyInstalledProcessVersion(255, unit, changed, checked, 42, invocation); err == nil {
			t.Fatal("changed command accepted")
		}
	}
}

func TestInstalledProcessBindingUbuntu249ParentLifetime(t *testing.T) {
	root, files, _ := bindingBytesFixture(t)
	checked, err := readLocalBinding(root, fixtureReader(files))
	if err != nil {
		t.Fatal(err)
	}
	invocation := [16]byte{1}
	unit, service := installedProcessPropertiesFixture(t, checked, invocation)
	// The actual systemd 249 manager exposes neither selectable exit type nor
	// restart mode. Its parent lifetime still must be checked before admission.
	delete(service, "ExitType")
	delete(service, "RestartMode")
	if err := verifyInstalledProcessVersion(249, unit, service, checked, 42, invocation); err != nil {
		t.Fatalf("systemd 249 parent lifetime refused: %v", err)
	}
	for _, version := range []uint16{0, 255, 259} {
		if err := verifyInstalledProcessVersion(version, unit, service, checked, 42, invocation); err == nil {
			t.Fatalf("missing lifetime properties accepted for manager %d", version)
		}
	}
	for _, mutation := range []struct {
		key, signature string
		value          any
	}{
		{"ExitType", "s", "cgroup"},
		{"RestartMode", "s", "direct"},
		{"ExitType", "b", false},
		{"RemainAfterExit", "b", true},
		{"Restart", "s", "always"},
		{"MainPID", "u", uint32(43)},
	} {
		changed := cloneProperties(service)
		body, err := json.Marshal(mutation.value)
		if err != nil {
			t.Fatal(err)
		}
		changed[mutation.key] = worker.Value{Type: mutation.signature, Data: body}
		if err := verifyInstalledProcessVersion(249, unit, changed, checked, 42, invocation); err == nil {
			t.Fatalf("changed %s accepted on manager 249", mutation.key)
		}
	}
}

func cloneProperties(input worker.Properties) worker.Properties {
	result := worker.Properties{}
	for key, value := range input {
		result[key] = value
	}
	return result
}

func installedProcessPropertiesFixture(t *testing.T, checked checkedBinding, invocation [16]byte) (worker.Properties, worker.Properties) {
	t.Helper()
	put := func(target worker.Properties, key, signature string, value any) {
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		target[key] = worker.Value{Type: signature, Data: body}
	}
	unit, service := worker.Properties{}, worker.Properties{}
	for key, value := range map[string]string{"Id": "ardents-endpoint.service", "LoadState": "loaded", "ActiveState": "active", "SubState": "running", "FragmentPath": "/etc/systemd/system/ardents-endpoint.service"} {
		put(unit, key, "s", value)
	}
	put(unit, "DropInPaths", "as", []string{})
	put(unit, "InvocationID", "ay", invocation)
	for key, value := range map[string]string{"User": "ardents-endpoint", "Group": "ardents-endpoint", "Type": "exec", "ControlGroup": "/system.slice/ardents-endpoint.service", "WorkingDirectory": "/", "ProtectHome": "yes", "ProtectSystem": "strict", "KillMode": "control-group", "Restart": "no", "ExitType": "main", "RestartMode": "normal"} {
		put(service, key, "s", value)
	}
	put(service, "MainPID", "u", uint32(42))
	put(service, "UMask", "u", uint32(0077))
	for _, key := range []string{"CapabilityBoundingSet", "AmbientCapabilities", "LimitCORE"} {
		put(service, key, "t", uint64(0))
	}
	put(service, "SupplementaryGroups", "as", []string{})
	put(service, "ReadWritePaths", "as", writableDirectories(checked.request))
	for _, key := range []string{"NoNewPrivileges", "PrivateTmp", "ProtectControlGroups", "ProtectKernelTunables", "ProtectKernelModules", "ProtectKernelLogs", "RestrictSUIDSGID", "LockPersonality", "MemoryAccounting", "CPUAccounting", "TasksAccounting"} {
		put(service, key, "b", true)
	}
	put(service, "RemainAfterExit", "b", false)
	for _, key := range []string{"ExecCondition", "ExecStartPre", "ExecStartPost", "ExecReload", "ExecStop", "ExecStopPost"} {
		put(service, key, "a(sasbttttuii)", []any{})
	}
	program := filepath.Join(checked.directory, "ardents-linux-amd64")
	put(service, "ExecStartEx", "a(sasasttttuii)", []any{[]any{program, []string{program, "endpoint", "start-installed", checked.binding.InstallationRoot}, []string{"no-env-expand"}, 1, 1, 0, 0, 42, 0, 0}})
	put(service, "RestrictAddressFamilies", "(bas)", []any{true, []string{"AF_UNIX", "AF_INET", "AF_INET6"}})
	return unit, service
}
