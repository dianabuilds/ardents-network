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

// Typed data fixtures test refusal rules, not genuine live manager authority.
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
