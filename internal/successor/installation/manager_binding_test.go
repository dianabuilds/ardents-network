package installation

import (
	"encoding/json"
	"errors"
	"path"
	"strings"
	"testing"
)

// Independently spelled required configuration. These observations exercise
// refusal/coherence only; they do not supply a successful native manager.
func stoppedEndpointFixture(t *testing.T) (managerProperties, managerProperties, installationRequest, string) {
	t.Helper()
	request, err := decodeInstallationRequest(requestFixture())
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("01", 32)
	unit, service := managerProperties{}, managerProperties{}
	put := func(properties managerProperties, name, signature string, data any) {
		raw, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		properties[name] = managerValue{Type: signature, Data: raw}
	}
	for name, value := range map[string]string{"Id": "ardents-endpoint.service", "LoadState": "loaded", "ActiveState": "inactive", "SubState": "dead", "FragmentPath": "/etc/systemd/system/ardents-endpoint.service"} {
		put(unit, name, "s", value)
	}
	put(unit, "DropInPaths", "as", []string{})
	put(unit, "Requires", "as", []string{"sysinit.target", "ardents-text-publisher.socket", "ardents-text-reader.socket", "system.slice"})
	for name, value := range map[string]string{"User": "ardents-endpoint", "Group": "ardents-endpoint", "Type": "exec", "WorkingDirectory": "/", "ProtectHome": "yes", "ProtectSystem": "strict", "KillMode": "control-group", "Restart": "no", "RootDirectory": "", "RootImage": "", "ExitType": "main", "RestartMode": "normal"} {
		put(service, name, "s", value)
	}
	put(service, "MainPID", "u", 0)
	put(service, "UMask", "u", 63)
	put(service, "TimeoutStopUSec", "t", uint64(30_000_000))
	put(service, "ReadWritePaths", "as", []string{"/entry", "/permissions", "/roles", "/socket", "/state", "/tokens"})
	put(service, "SupplementaryGroups", "as", []string{})
	for _, name := range []string{"CapabilityBoundingSet", "AmbientCapabilities", "LimitCORE"} {
		put(service, name, "t", uint64(0))
	}
	for _, name := range []string{"NoNewPrivileges", "PrivateTmp", "ProtectControlGroups", "ProtectKernelTunables", "ProtectKernelModules", "ProtectKernelLogs", "RestrictSUIDSGID", "LockPersonality", "MemoryAccounting", "CPUAccounting", "TasksAccounting"} {
		put(service, name, "b", true)
	}
	put(service, "RemainAfterExit", "b", false)
	for _, name := range []string{"ExecCondition", "ExecStartPre", "ExecStartPost", "ExecReload", "ExecStop", "ExecStopPost"} {
		put(service, name, "a(sasbttttuii)", []any{})
	}
	put(service, "RestrictAddressFamilies", "(bas)", []any{true, []string{"AF_UNIX", "AF_INET6", "AF_INET"}})
	program := path.Join(request.InstallationRoot, "generations", digest, "ardents-linux-amd64")
	put(service, "ExecStartEx", "a(sasasttttuii)", []any{[]any{program, []string{program, "endpoint", "start-installed", request.InstallationRoot}, []string{"no-env-expand"}, 0, 0, 0, 0, 0, 0, 0}})
	return unit, service, request, digest
}

func TestStoppedEndpointPropertiesPreserveKnownProfiles(t *testing.T) {
	unit, service, request, digest := stoppedEndpointFixture(t)
	if err := verifyStoppedEndpointProperties("255.4-1ubuntu8.17", unit, service, request, digest); err != nil {
		t.Fatal(err)
	}
	delete(service, "ExitType")
	delete(service, "RestartMode")
	if err := verifyStoppedEndpointProperties("249.11-0ubuntu3.22", unit, service, request, digest); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"255.4-1ubuntu8.17", "259.5-0ubuntu3.4", "", "0"} {
		if err := verifyStoppedEndpointProperties(version, unit, service, request, digest); err == nil {
			t.Fatal("missing lifetime policy/new profile accepted", version)
		}
	}
	service["RemainAfterExit"] = managerValue{Type: "b", Data: json.RawMessage("true")}
	if err := verifyStoppedEndpointProperties("249.11", unit, service, request, digest); !errors.Is(err, ErrBinding) {
		t.Fatal("older manager waived parent lifetime", err)
	}
}

func TestStoppedEndpointPropertiesRefuseMissingNullWrongTypeAndWeakerProtection(t *testing.T) {
	_, baseline, _, _ := stoppedEndpointFixture(t)
	for name := range baseline {
		for _, kind := range []string{"missing", "null", "wrong-type"} {
			t.Run(name+"/"+kind, func(t *testing.T) {
				unit, service, request, digest := stoppedEndpointFixture(t)
				value := service[name]
				switch kind {
				case "missing":
					delete(service, name)
				case "null":
					value.Data = json.RawMessage("null")
					service[name] = value
				case "wrong-type":
					value.Type = "v"
					service[name] = value
				}
				if err := verifyStoppedEndpointProperties("255.4", unit, service, request, digest); !errors.Is(err, ErrBinding) {
					t.Fatal("required observation lost", err)
				}
			})
		}
	}
	for name, value := range map[string]managerValue{
		"NoNewPrivileges": {Type: "b", Data: json.RawMessage("false")}, "ProtectSystem": {Type: "s", Data: json.RawMessage(`"full"`)},
		"MainPID": {Type: "u", Data: json.RawMessage("42")}, "CapabilityBoundingSet": {Type: "t", Data: json.RawMessage("18446744073709551615")},
		"ReadWritePaths": {Type: "as", Data: json.RawMessage(`["/"]`)}, "ExecStartPost": {Type: "a(sasbttttuii)", Data: json.RawMessage(`[["/bin/true",["/bin/true"],false,0,0,0,0,0,0,0]]`)},
	} {
		t.Run(name+"/weaker", func(t *testing.T) {
			unit, service, request, digest := stoppedEndpointFixture(t)
			service[name] = value
			if err := verifyStoppedEndpointProperties("255.4", unit, service, request, digest); !errors.Is(err, ErrBinding) {
				t.Fatal("weaker configuration accepted", err)
			}
		})
	}
}

func TestStoppedEndpointUnitPropertiesRefuseMissingOrForeignIdentity(t *testing.T) {
	baseline, _, _, _ := stoppedEndpointFixture(t)
	for name := range baseline {
		for _, kind := range []string{"missing", "null", "wrong-type"} {
			t.Run(name+"/"+kind, func(t *testing.T) {
				unit, service, request, digest := stoppedEndpointFixture(t)
				value := unit[name]
				switch kind {
				case "missing":
					delete(unit, name)
				case "null":
					value.Data = json.RawMessage("null")
					unit[name] = value
				case "wrong-type":
					value.Type = "v"
					unit[name] = value
				}
				if err := verifyStoppedEndpointProperties("255.4", unit, service, request, digest); !errors.Is(err, ErrBinding) {
					t.Fatal("required unit identity lost", err)
				}
			})
		}
	}
}

func TestStoppedEndpointExecRefusesForeignArgumentsFlagsAndProcessHistory(t *testing.T) {
	for index := 0; index < 10; index++ {
		t.Run(string(rune('a'+index)), func(t *testing.T) {
			unit, service, request, digest := stoppedEndpointFixture(t)
			var entries [][]json.RawMessage
			if err := json.Unmarshal(service["ExecStartEx"].Data, &entries); err != nil {
				t.Fatal(err)
			}
			switch index {
			case 0:
				entries[0][0] = json.RawMessage(`"/foreign"`)
			case 1:
				entries[0][1] = json.RawMessage(`["/foreign"]`)
			case 2:
				entries[0][2] = json.RawMessage(`[]`)
			default:
				entries[0][index] = json.RawMessage(`1`)
			}
			raw, err := json.Marshal(entries)
			if err != nil {
				t.Fatal(err)
			}
			service["ExecStartEx"] = managerValue{Type: "a(sasasttttuii)", Data: raw}
			if err := verifyStoppedEndpointProperties("255.4", unit, service, request, digest); !errors.Is(err, ErrBinding) {
				t.Fatal("foreign execution observation accepted", err)
			}
		})
	}
}

func TestManagerJSONRefusesDuplicateVariantsUnknownFieldsAndTrailingData(t *testing.T) {
	for _, raw := range []string{
		`{"type":"b","type":"b","data":true}`, `{"type":"b","data":true,"foreign":0}`, `{"type":"b","data":true} {}`,
		`{"type":"a{sv}","data":[{"NoNewPrivileges":{"type":"b","data":false},"NoNewPrivileges":{"type":"b","data":true}}]}`,
	} {
		var value managerValue
		if err := decodeManagerJSON([]byte(raw), &value); !errors.Is(err, ErrBinding) {
			t.Fatal("ambiguous manager response accepted", err)
		}
	}
	var payload []managerProperties
	if err := decodeManagerJSON([]byte(`[{"NoNewPrivileges":{"type":"b","data":true,"foreign":0}}]`), &payload); !errors.Is(err, ErrBinding) {
		t.Fatal("unknown nested variant field accepted", err)
	}
	for _, raw := range []string{`0.0`, `null`, `false`, `18446744073709551615`} {
		properties := managerProperties{"CapabilityBoundingSet": {Type: "t", Data: json.RawMessage(raw)}}
		if managerPropertyMatches(properties, "CapabilityBoundingSet", "t", uint64(0)) {
			t.Fatal("nonzero/untyped representation accepted", raw)
		}
	}
}
