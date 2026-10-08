package unit

import (
	"encoding/json"
	"errors"
	"path"
	"strings"
	"testing"
)

func TestQueuedEndpointJobDoesNotBecomeQuiescence(t *testing.T) {
	for _, observed := range []struct {
		name     string
		value    managerValue
		accepted bool
	}{
		{"none", managerValue{Type: "(uo)", Data: json.RawMessage(`[0,"/"]`)}, true},
		{"pending", managerValue{Type: "(uo)", Data: json.RawMessage(`[42,"/org/freedesktop/systemd1/job/42"]`)}, false},
		{"lost signature", managerValue{Type: "uo", Data: json.RawMessage(`[0,"/"]`)}, false},
		{"foreign zero job path", managerValue{Type: "(uo)", Data: json.RawMessage(`[0,"/org/freedesktop/systemd1/job/42"]`)}, false},
		{"noninteger", managerValue{Type: "(uo)", Data: json.RawMessage(`[0.0,"/"]`)}, false},
		{"extra part", managerValue{Type: "(uo)", Data: json.RawMessage(`[0,"/",0]`)}, false},
		{"missing", managerValue{}, false},
	} {
		t.Run(observed.name, func(t *testing.T) {
			err := VerifyNoJob(managerProperties{"Job": observed.value})
			if observed.accepted && err != nil || !observed.accepted && !errors.Is(err, ErrBinding) {
				t.Fatal("queued operation classification differs", err)
			}
		})
	}
}

func TestFailedStoppedAttemptRetainsFailureWithoutGrantingAdmission(t *testing.T) {
	fixture := func() (managerProperties, managerProperties, Configuration, string) {
		u, s, request, digest := stoppedEndpointFixture(t)
		u["ActiveState"] = managerValue{Type: "s", Data: json.RawMessage(`"failed"`)}
		u["SubState"] = managerValue{Type: "s", Data: json.RawMessage(`"failed"`)}
		u["Job"] = managerValue{Type: "(uo)", Data: json.RawMessage(`[0,"/"]`)}
		s["Result"] = managerValue{Type: "s", Data: json.RawMessage(`"exit-code"`)}
		program := path.Join(request.InstallationRoot, "generations", digest, "ardents-linux-amd64")
		raw, err := json.Marshal([]any{[]any{program, []string{program, "endpoint", "start-installed", request.InstallationRoot}, []string{"no-env-expand"}, 100, 200, 110, 210, 123, 1, 2}})
		if err != nil {
			t.Fatal(err)
		}
		s["ExecStartEx"] = managerValue{Type: "a(sasasttttuii)", Data: raw}
		return u, s, request, digest
	}
	u, s, request, digest := fixture()
	if err := VerifyStoppedAttempt("255.4", u, s, request, digest); err != nil {
		t.Fatal("completed failed attempt refused physical cleanup", err)
	}
	if VerifyFreshStopped("255.4", u, s, request, digest) == nil || VerifyQuiescent("255.4", u, s, request, digest) == nil {
		t.Fatal("failed cleanup observation became installation admission")
	}
	for _, change := range []string{"pending-job", "foreign-job", "missing-job", "live-pid", "activating", "wrong-substate", "missing-result", "success-result", "weak-protection", "missing-start", "missing-stop", "reversed-stop", "missing-pid", "missing-code", "unknown-code", "zero-status", "fractional-clock", "foreign-program"} {
		t.Run(change, func(t *testing.T) {
			u, s, request, digest := fixture()
			var exec [][]json.RawMessage
			if err := json.Unmarshal(s["ExecStartEx"].Data, &exec); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "pending-job":
				u["Job"] = managerValue{Type: "(uo)", Data: json.RawMessage(`[42,"/org/freedesktop/systemd1/job/42"]`)}
			case "foreign-job":
				u["Job"] = managerValue{Type: "(uo)", Data: json.RawMessage(`[0,"/foreign"]`)}
			case "missing-job":
				delete(u, "Job")
			case "live-pid":
				s["MainPID"] = managerValue{Type: "u", Data: json.RawMessage(`123`)}
			case "activating":
				u["ActiveState"] = managerValue{Type: "s", Data: json.RawMessage(`"activating"`)}
			case "wrong-substate":
				u["SubState"] = managerValue{Type: "s", Data: json.RawMessage(`"dead"`)}
			case "missing-result":
				delete(s, "Result")
			case "success-result":
				s["Result"] = managerValue{Type: "s", Data: json.RawMessage(`"success"`)}
			case "weak-protection":
				s["NoNewPrivileges"] = managerValue{Type: "b", Data: json.RawMessage(`false`)}
			case "missing-start":
				exec[0][3] = json.RawMessage(`0`)
			case "missing-stop":
				exec[0][5] = json.RawMessage(`0`)
			case "reversed-stop":
				exec[0][6] = json.RawMessage(`199`)
			case "missing-pid":
				exec[0][7] = json.RawMessage(`0`)
			case "missing-code":
				exec[0][8] = json.RawMessage(`null`)
			case "unknown-code":
				exec[0][8] = json.RawMessage(`9`)
			case "zero-status":
				exec[0][9] = json.RawMessage(`0`)
			case "fractional-clock":
				exec[0][3] = json.RawMessage(`100.0`)
			case "foreign-program":
				exec[0][0] = json.RawMessage(`"/foreign"`)
			}
			raw, err := json.Marshal(exec)
			if err != nil {
				t.Fatal(err)
			}
			s["ExecStartEx"] = managerValue{Type: "a(sasasttttuii)", Data: raw}
			if err := VerifyStoppedAttempt("255.4", u, s, request, digest); !errors.Is(err, ErrBinding) {
				t.Fatal("uncertain/foreign attempt became cleanup completion", err)
			}
		})
	}
}

// Independently spelled required configuration. These observations exercise
// refusal/coherence only; they do not supply a successful native manager.
func stoppedEndpointFixture(t *testing.T) (managerProperties, managerProperties, Configuration, string) {
	t.Helper()
	request := Configuration{InstallationRoot: "/installation", WritePaths: []string{"/entry", "/permissions", "/roles", "/socket", "/state", "/tokens"}}
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
	if err := VerifyFreshStopped("255.4-1ubuntu8.17", unit, service, request, digest); err != nil {
		t.Fatal(err)
	}
	delete(service, "ExitType")
	delete(service, "RestartMode")
	if err := VerifyFreshStopped("249.11-0ubuntu3.22", unit, service, request, digest); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"255.4-1ubuntu8.17", "259.5-0ubuntu3.4", "", "0"} {
		if err := VerifyFreshStopped(version, unit, service, request, digest); err == nil {
			t.Fatal("missing lifetime policy/new profile accepted", version)
		}
	}
	service["RemainAfterExit"] = managerValue{Type: "b", Data: json.RawMessage("true")}
	if err := VerifyFreshStopped("249.11", unit, service, request, digest); !errors.Is(err, ErrBinding) {
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
				if err := VerifyFreshStopped("255.4", unit, service, request, digest); !errors.Is(err, ErrBinding) {
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
			if err := VerifyFreshStopped("255.4", unit, service, request, digest); !errors.Is(err, ErrBinding) {
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
				if err := VerifyFreshStopped("255.4", unit, service, request, digest); !errors.Is(err, ErrBinding) {
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
			if err := VerifyFreshStopped("255.4", unit, service, request, digest); !errors.Is(err, ErrBinding) {
				t.Fatal("foreign execution observation accepted", err)
			}
		})
	}
}

func TestStoppedEndpointExecRefusesNullProcessHistory(t *testing.T) {
	for index := 3; index < 10; index++ {
		unit, service, request, digest := stoppedEndpointFixture(t)
		var entries [][]json.RawMessage
		if err := json.Unmarshal(service["ExecStartEx"].Data, &entries); err != nil {
			t.Fatal(err)
		}
		entries[0][index] = json.RawMessage(`null`)
		raw, err := json.Marshal(entries)
		if err != nil {
			t.Fatal(err)
		}
		service["ExecStartEx"] = managerValue{Type: "a(sasasttttuii)", Data: raw}
		if err := VerifyFreshStopped("255.4", unit, service, request, digest); !errors.Is(err, ErrBinding) {
			t.Fatal("missing process history became observed zero", index, err)
		}
	}
}

// Typed data fixtures test refusal rules, not genuine live manager authority.
func runningEndpointFixture(t *testing.T) (managerProperties, managerProperties, Configuration, string, [16]byte) {
	t.Helper()
	unit, service, request, digest := stoppedEndpointFixture(t)
	invocation := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	raw, err := json.Marshal(invocation)
	if err != nil {
		t.Fatal(err)
	}
	unit["InvocationID"] = managerValue{Type: "ay", Data: raw}
	unit["ActiveState"] = managerValue{Type: "s", Data: json.RawMessage(`"active"`)}
	unit["SubState"] = managerValue{Type: "s", Data: json.RawMessage(`"running"`)}
	service["MainPID"] = managerValue{Type: "u", Data: json.RawMessage(`123`)}
	service["ControlGroup"] = managerValue{Type: "s", Data: json.RawMessage(`"/system.slice/ardents-endpoint.service"`)}
	var entries [][]json.RawMessage
	if err := json.Unmarshal(service["ExecStartEx"].Data, &entries); err != nil {
		t.Fatal(err)
	}
	entries[0][3] = json.RawMessage(`1000000`)
	entries[0][4] = json.RawMessage(`2000000`)
	entries[0][7] = json.RawMessage(`123`)
	raw, err = json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	service["ExecStartEx"] = managerValue{Type: "a(sasasttttuii)", Data: raw}
	return unit, service, request, digest, invocation
}

func TestRunningEndpointRequiresOriginalInvocationAndConfiguration(t *testing.T) {
	unit, service, request, digest, invocation := runningEndpointFixture(t)
	pid, actual, err := Invocation(unit, service)
	if err != nil || pid != 123 || actual != invocation {
		t.Fatalf("typed invocation differs: pid=%d error=%v", pid, err)
	}
	if err := VerifyRunning("255.4", unit, service, request, digest, pid, invocation); err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"replacement-pid", "replacement-invocation", "different-cgroup", "stopped", "weak-protection", "hook", "foreign-executable"} {
		t.Run(change, func(t *testing.T) {
			unit, service, request, digest, invocation := runningEndpointFixture(t)
			switch change {
			case "replacement-pid":
				service["MainPID"] = managerValue{Type: "u", Data: json.RawMessage(`124`)}
			case "replacement-invocation":
				unit["InvocationID"].Data[1] = '9'
			case "different-cgroup":
				service["ControlGroup"] = managerValue{Type: "s", Data: json.RawMessage(`"/system.slice/replacement.service"`)}
			case "stopped":
				unit["ActiveState"] = managerValue{Type: "s", Data: json.RawMessage(`"inactive"`)}
			case "weak-protection":
				service["NoNewPrivileges"] = managerValue{Type: "b", Data: json.RawMessage(`false`)}
			case "hook":
				service["ExecStop"] = managerValue{Type: "a(sasbttttuii)", Data: json.RawMessage(`[["/bin/true",["/bin/true"],false,0,0,0,0,0,0,0]]`)}
			case "foreign-executable":
				service["ExecStartEx"] = managerValue{Type: "a(sasasttttuii)", Data: json.RawMessage(`[["/foreign",["/foreign"],[],1,2,0,0,123,0,0]]`)}
			}
			if err := VerifyRunning("255.4", unit, service, request, digest, 123, invocation); err == nil {
				t.Fatal("changed original invocation/configuration accepted")
			}
		})
	}
}

func TestManagerInvocationRefusesIncompleteByteIdentity(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `[1]`, `[null,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16]`, `[256,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16]`, `[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`} {
		unit, service, _, _, _ := runningEndpointFixture(t)
		unit["InvocationID"] = managerValue{Type: "ay", Data: json.RawMessage(raw)}
		if _, _, err := Invocation(unit, service); err == nil {
			t.Fatal("incomplete invocation accepted", raw)
		}
	}
}

// These typed history fixtures grant no native empty-scope or stopped authority.
func quiescentEndpointFixture(t *testing.T) (managerProperties, managerProperties, Configuration, string) {
	t.Helper()
	unit, service, request, digest := stoppedEndpointFixture(t)
	var entries [][]json.RawMessage
	if err := json.Unmarshal(service["ExecStartEx"].Data, &entries); err != nil {
		t.Fatal(err)
	}
	for index, value := range []string{"1000000", "2000000", "3000000", "4000000", "123", "2", "15"} {
		entries[0][index+3] = json.RawMessage(value)
	}
	body, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	service["ExecStartEx"] = managerValue{Type: "a(sasasttttuii)", Data: body}
	return unit, service, request, digest
}

func TestQuiescentEndpointPreservesStoppedHistoryWithoutInitialAdmission(t *testing.T) {
	unit, service, request, digest := quiescentEndpointFixture(t)
	if err := VerifyQuiescent("255.4", unit, service, request, digest); err != nil {
		t.Fatal("stopped predecessor history refused", err)
	}
	if err := VerifyFreshStopped("255.4", unit, service, request, digest); !errors.Is(err, ErrBinding) {
		t.Fatal("historical execution became fresh initial admission", err)
	}
	delete(service, "ExitType")
	delete(service, "RestartMode")
	if err := VerifyQuiescent("249.11", unit, service, request, digest); err != nil {
		t.Fatal(err)
	}
	if err := VerifyQuiescent("259.5", unit, service, request, digest); !errors.Is(err, ErrNativeUnavailable) {
		t.Fatal("new manager admitted by stopped history", err)
	}
}

func TestQuiescentEndpointRejectsInvalidHistoricalTypes(t *testing.T) {
	for index := 3; index < 10; index++ {
		invalid := []string{`null`, `"1"`, `1.5`, `18446744073709551616`}
		if index < 8 {
			invalid = append(invalid, `-1`)
		}
		if index == 7 {
			invalid = append(invalid, `4294967296`)
		}
		if index > 7 {
			invalid = append(invalid, `2147483648`, `-2147483649`)
		}
		for _, value := range invalid {
			unit, service, request, digest := quiescentEndpointFixture(t)
			var entries [][]json.RawMessage
			if err := json.Unmarshal(service["ExecStartEx"].Data, &entries); err != nil {
				t.Fatal(err)
			}
			entries[0][index] = json.RawMessage(value)
			body, err := json.Marshal(entries)
			if err != nil {
				t.Fatal(err)
			}
			service["ExecStartEx"] = managerValue{Type: "a(sasasttttuii)", Data: body}
			if err := VerifyQuiescent("255.4", unit, service, request, digest); !errors.Is(err, ErrBinding) {
				t.Fatalf("invalid historical field %d accepted: %s, %v", index, value, err)
			}
		}
	}
}

func TestQuiescentEndpointRejectsLiveOrWeakerConfiguration(t *testing.T) {
	for _, change := range []string{"live", "main-pid", "null-pid", "failed", "weak-protection", "foreign-program"} {
		unit, service, request, digest := quiescentEndpointFixture(t)
		switch change {
		case "live":
			unit["ActiveState"] = managerValue{Type: "s", Data: json.RawMessage(`"active"`)}
			unit["SubState"] = managerValue{Type: "s", Data: json.RawMessage(`"running"`)}
		case "main-pid":
			service["MainPID"] = managerValue{Type: "u", Data: json.RawMessage(`123`)}
		case "null-pid":
			service["MainPID"] = managerValue{Type: "u", Data: json.RawMessage(`null`)}
		case "failed":
			unit["ActiveState"] = managerValue{Type: "s", Data: json.RawMessage(`"failed"`)}
		case "weak-protection":
			service["NoNewPrivileges"] = managerValue{Type: "b", Data: json.RawMessage(`false`)}
		case "foreign-program":
			service["ExecStartEx"] = managerValue{Type: "a(sasasttttuii)", Data: json.RawMessage(`[["/foreign",["/foreign"],[],1,2,3,4,123,2,15]]`)}
		}
		if err := VerifyQuiescent("255.4", unit, service, request, digest); !errors.Is(err, ErrBinding) {
			t.Fatal("unquiescent/foreign configuration accepted", change, err)
		}
	}
}

// Independently spelled typed observations are grammar/refusal fixtures only.
func activationSocketFixture(t *testing.T, role string) (managerProperties, managerProperties) {
	t.Helper()
	unit, socket := managerProperties{}, managerProperties{}
	put := func(properties managerProperties, name, signature string, value any) {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		properties[name] = managerValue{Type: signature, Data: raw}
	}
	name := "ardents-text-" + role + ".socket"
	for key, value := range map[string]string{"Id": name, "LoadState": "loaded", "FragmentPath": "/etc/systemd/system/" + name, "ActiveState": "active", "SubState": "listening"} {
		put(unit, key, "s", value)
	}
	put(unit, "DropInPaths", "as", []string{})
	put(unit, "PartOf", "as", []string{"ardents-endpoint.service"})
	put(socket, "Accept", "b", true)
	put(socket, "SocketUser", "s", "ardents-endpoint")
	put(socket, "SocketGroup", "s", "ardents-endpoint")
	put(socket, "SocketMode", "u", uint32(0600))
	put(socket, "DirectoryMode", "u", uint32(0711))
	put(socket, "MaxConnections", "u", uint32(64))
	put(socket, "RemoveOnStop", "b", true)
	put(socket, "Listen", "a(ss)", [][2]string{{"Stream", "/run/ardents-text/" + role + ".sock"}})
	for _, name := range []string{"ExecStartPre", "ExecStartPost", "ExecStopPre", "ExecStopPost"} {
		put(socket, name, "a(sasbttttuii)", []any{})
	}
	return unit, socket
}

func TestActivationSocketRequiresFixedRoleOwnershipAndLifetime(t *testing.T) {
	for _, role := range []string{"reader", "publisher"} {
		unit, socket := activationSocketFixture(t, role)
		if err := VerifyListeningSocket(unit, socket, role); err != nil {
			t.Fatal(err)
		}
		for _, side := range []string{"unit", "socket"} {
			baseline := unit
			if side == "socket" {
				baseline = socket
			}
			for key := range baseline {
				for _, change := range []string{"missing", "null", "wrong-type"} {
					t.Run(role+"/"+side+"/"+key+"/"+change, func(t *testing.T) {
						unit, socket := activationSocketFixture(t, role)
						properties := unit
						if side == "socket" {
							properties = socket
						}
						value := properties[key]
						switch change {
						case "missing":
							delete(properties, key)
						case "null":
							value.Data = json.RawMessage(`null`)
							properties[key] = value
						case "wrong-type":
							value.Type = "v"
							properties[key] = value
						}
						if err := VerifyListeningSocket(unit, socket, role); err == nil {
							t.Fatal("required fixed socket observation lost")
						}
					})
				}
			}
		}
	}
	unit, socket := activationSocketFixture(t, "reader")
	if err := VerifyListeningSocket(unit, socket, "publisher"); err == nil {
		t.Fatal("other role adopted reader socket")
	}
	for key, value := range map[string]managerValue{
		"Listen":       {Type: "a(ss)", Data: json.RawMessage(`[["Stream","/foreign.sock"]]`)},
		"SocketMode":   {Type: "u", Data: json.RawMessage(`438`)},
		"RemoveOnStop": {Type: "b", Data: json.RawMessage(`false`)},
		"ExecStopPost": {Type: "a(sasbttttuii)", Data: json.RawMessage(`[["/bin/true",["/bin/true"],false,0,0,0,0,0,0,0]]`)},
	} {
		unit, socket := activationSocketFixture(t, "reader")
		socket[key] = value
		if err := VerifyListeningSocket(unit, socket, "reader"); err == nil {
			t.Fatal("foreign/weak stop configuration accepted", key)
		}
	}
}

func TestStoppedActivationSocketRetainsConfigurationAndRefusesListening(t *testing.T) {
	for _, role := range []string{"reader", "publisher"} {
		unit, socket := activationSocketFixture(t, role)
		if err := VerifyStoppedSocket(unit, socket, role); err == nil {
			t.Fatal("listening socket became stopped", role)
		}
		unit["ActiveState"] = managerValue{Type: "s", Data: json.RawMessage(`"inactive"`)}
		unit["SubState"] = managerValue{Type: "s", Data: json.RawMessage(`"dead"`)}
		if err := VerifyStoppedSocket(unit, socket, role); err != nil {
			t.Fatal("bound stopped socket refused", role, err)
		}
		if err := VerifyListeningSocket(unit, socket, role); err == nil {
			t.Fatal("stopped socket became listening", role)
		}
		socket["Accept"] = managerValue{Type: "b", Data: json.RawMessage(`false`)}
		if err := VerifyStoppedSocket(unit, socket, role); err == nil {
			t.Fatal("stopped socket waived configuration", role)
		}
	}
}

func TestStoppedManagerPropertyGrammar(t *testing.T) {
	const endpoint = "LoadState=loaded\nActiveState=inactive\nSubState=dead\nFragmentPath=/etc/systemd/system/ardents-endpoint.service\nDropInPaths=\nMainPID=0\n"
	if err := VerifyStoppedUnit(endpoint, "ardents-endpoint.service"); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		strings.Replace(endpoint, "MainPID=0", "MainPID=123", 1),
		strings.Replace(endpoint, "ActiveState=inactive", "ActiveState=active", 1),
		strings.Replace(endpoint, "DropInPaths=", "DropInPaths=/etc/systemd/system/foreign.conf", 1),
		strings.Replace(endpoint, "FragmentPath=/etc", "FragmentPath=/foreign", 1),
		strings.Replace(endpoint, "MainPID=0\n", "", 1),
		endpoint + "MainPID=0\n", endpoint + "Foreign=accepted\n",
	} {
		if err := VerifyStoppedUnit(body, "ardents-endpoint.service"); !errors.Is(err, ErrBinding) {
			t.Fatal("invalid stopped properties accepted", err)
		}
	}
	for _, unit := range []string{"ardents-text-reader.socket", "ardents-text-publisher.socket"} {
		body := "LoadState=loaded\nActiveState=inactive\nSubState=dead\nFragmentPath=/etc/systemd/system/" + unit + "\nDropInPaths=\n"
		if err := VerifyStoppedUnit(body, unit); err != nil {
			t.Fatal(err)
		}
	}
	if !errors.Is(VerifyStoppedUnit(endpoint, "foreign.service"), ErrBinding) {
		t.Fatal("foreign manager unit accepted")
	}
}
