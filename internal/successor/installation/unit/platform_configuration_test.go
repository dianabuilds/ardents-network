package unit

import (
	"encoding/json"
	"errors"
	"path"
	"strings"
	"testing"
)

// The platform-managed Endpoint unit admits exactly one privileged ExecStartPre
// whose argv is the original F-25 pre-start platform check, while the transitional
// native configuration keeps ExecStartPre empty. These observations exercise the
// shared confinement contract and the platform pre-start binding only; they supply
// no successful native manager, installation, start, replacement or recovery
// authority. Worker confinement (F-27) is the same shared contract.

const (
	platformTestCheckProgram = "/distribution/ardents-linux-amd64"
	platformTestManifest     = "/etc/ardents/endpoint.json"
)

func platformTestPin() string { return strings.Repeat("ab", 32) }

// platformEndpointFixture extends the stopped configuration with the sole
// privileged platform pre-start check and the matching ExecStartEx binding.
func platformEndpointFixture(t *testing.T) (managerProperties, managerProperties, Configuration, string) {
	t.Helper()
	unit, service, request, digest := stoppedEndpointFixture(t)
	pin := platformTestPin()
	put := func(name, signature string, data any) {
		raw, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		service[name] = managerValue{Type: signature, Data: raw}
	}
	argv := []string{platformTestCheckProgram, "platform", "check", platformTestManifest, pin}
	put("ExecStartPreEx", "a(sasasttttuii)", []any{[]any{platformTestCheckProgram, argv, []string{"privileged", "no-env-expand"}, 0, 0, 0, 0, 0, 0, 0}})
	// A real platform unit also carries the pre-start command in ExecStartPre; the
	// transitional empty-ExecStartPre admission must refuse it.
	put("ExecStartPre", "a(sasbttttuii)", []any{[]any{platformTestCheckProgram, argv, false, 0, 0, 0, 0, 0, 0, 0}})
	return unit, service, request, digest
}

func verifyPlatform(unit, service managerProperties, request Configuration, digest string) error {
	return VerifyPlatformConfiguration("255.4", unit, service, request, digest, platformTestCheckProgram, platformTestManifest, platformTestPin())
}

func TestPlatformConfigurationAdmitsSolePrivilegedPreStartCheck(t *testing.T) {
	unit, service, request, digest := platformEndpointFixture(t)
	if err := verifyPlatform(unit, service, request, digest); err != nil {
		t.Fatal("valid platform configuration refused", err)
	}
	// The older admitted manager may omit the selectable lifetime policies.
	delete(service, "ExitType")
	delete(service, "RestartMode")
	if err := VerifyPlatformConfiguration("249.11", unit, service, request, digest, platformTestCheckProgram, platformTestManifest, platformTestPin()); err != nil {
		t.Fatal("older admitted manager platform configuration refused", err)
	}
	// The transitional configuration keeps ExecStartPre empty and never admits the
	// platform pre-start check through its own admission path.
	transitional, transitionalService, transitionalRequest, transitionalDigest := stoppedEndpointFixture(t)
	if err := VerifyConfiguration("255.4", transitional, transitionalService, transitionalRequest, transitionalDigest); err != nil {
		t.Fatal("transitional configuration refused", err)
	}
	freshUnit, freshService, freshRequest, freshDigest := platformEndpointFixture(t)
	if err := VerifyConfiguration("255.4", freshUnit, freshService, freshRequest, freshDigest); !errors.Is(err, ErrBinding) {
		t.Fatal("transitional admission accepted the platform pre-start check", err)
	}
}

func TestPlatformConfigurationRefusesUnadmittedManagerOrGeneration(t *testing.T) {
	for _, version := range []string{"259.5", "", "0", "248.9"} {
		t.Run("version/"+version, func(t *testing.T) {
			unit, service, request, digest := platformEndpointFixture(t)
			if err := VerifyPlatformConfiguration(version, unit, service, request, digest, platformTestCheckProgram, platformTestManifest, platformTestPin()); !errors.Is(err, ErrNativeUnavailable) {
				t.Fatal("unadmitted manager accepted", version, err)
			}
		})
	}
	for name, digest := range map[string]string{
		"uppercase": strings.Repeat("AB", 32),
		"short":     strings.Repeat("01", 31),
		"nonhex":    strings.Repeat("zz", 32),
		"empty":     "",
		"prefixed":  "0x" + strings.Repeat("01", 32),
	} {
		t.Run("digest/"+name, func(t *testing.T) {
			unit, service, request, _ := platformEndpointFixture(t)
			if err := verifyPlatform(unit, service, request, digest); !errors.Is(err, ErrNativeUnavailable) {
				t.Fatal("non-canonical generation digest accepted", name, err)
			}
		})
	}
}

func TestPlatformConfigurationRefusesMissingNullWrongTypeServiceObservation(t *testing.T) {
	_, baseline, _, _ := platformEndpointFixture(t)
	for name := range baseline {
		// ExecStartPre belongs to the transitional empty-check path; MainPID is a
		// process-state observation owned by the stopped/running wrappers, not the
		// shared configuration contract.
		if name == "ExecStartPre" || name == "MainPID" {
			continue
		}
		for _, kind := range []string{"missing", "null", "wrong-type"} {
			t.Run(name+"/"+kind, func(t *testing.T) {
				unit, service, request, digest := platformEndpointFixture(t)
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
				if err := verifyPlatform(unit, service, request, digest); !errors.Is(err, ErrBinding) {
					t.Fatal("required platform observation lost", name, kind, err)
				}
			})
		}
	}
}

func TestPlatformConfigurationRefusesMissingOrForeignUnitIdentity(t *testing.T) {
	baseline, _, _, _ := platformEndpointFixture(t)
	for name := range baseline {
		// ActiveState/SubState are process-state observations owned by the
		// stopped/running wrappers, not the shared configuration contract.
		if name == "ActiveState" || name == "SubState" {
			continue
		}
		for _, kind := range []string{"missing", "null", "wrong-type"} {
			t.Run(name+"/"+kind, func(t *testing.T) {
				unit, service, request, digest := platformEndpointFixture(t)
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
				if err := verifyPlatform(unit, service, request, digest); !errors.Is(err, ErrBinding) {
					t.Fatal("required platform unit identity lost", name, kind, err)
				}
			})
		}
	}
	for name, value := range map[string]managerValue{
		"Id":           {Type: "s", Data: json.RawMessage(`"ardents-other.service"`)},
		"LoadState":    {Type: "s", Data: json.RawMessage(`"not-found"`)},
		"FragmentPath": {Type: "s", Data: json.RawMessage(`"/etc/systemd/system/ardents-other.service"`)},
		"DropInPaths":  {Type: "as", Data: json.RawMessage(`["/etc/systemd/system/ardents-endpoint.service.d/override.conf"]`)},
		"Requires":     {Type: "as", Data: json.RawMessage(`["ardents-text-reader.socket"]`)},
	} {
		t.Run(name+"/foreign", func(t *testing.T) {
			unit, service, request, digest := platformEndpointFixture(t)
			unit[name] = value
			if err := verifyPlatform(unit, service, request, digest); !errors.Is(err, ErrBinding) {
				t.Fatal("foreign platform unit identity accepted", name, err)
			}
		})
	}
}

func TestPlatformConfigurationRefusesWeakerConfinement(t *testing.T) {
	for name, value := range map[string]managerValue{
		"User":                  {Type: "s", Data: json.RawMessage(`"root"`)},
		"Group":                 {Type: "s", Data: json.RawMessage(`"root"`)},
		"Type":                  {Type: "s", Data: json.RawMessage(`"simple"`)},
		"ProtectSystem":         {Type: "s", Data: json.RawMessage(`"full"`)},
		"ProtectHome":           {Type: "s", Data: json.RawMessage(`"no"`)},
		"KillMode":              {Type: "s", Data: json.RawMessage(`"process"`)},
		"Restart":               {Type: "s", Data: json.RawMessage(`"always"`)},
		"WorkingDirectory":      {Type: "s", Data: json.RawMessage(`"/root"`)},
		"RootDirectory":         {Type: "s", Data: json.RawMessage(`"/chroot"`)},
		"UMask":                 {Type: "u", Data: json.RawMessage(`0`)},
		"SupplementaryGroups":   {Type: "as", Data: json.RawMessage(`["wheel"]`)},
		"ReadWritePaths":        {Type: "as", Data: json.RawMessage(`["/"]`)},
		"CapabilityBoundingSet": {Type: "t", Data: json.RawMessage(`18446744073709551615`)},
		"AmbientCapabilities":   {Type: "t", Data: json.RawMessage(`1`)},
		"LimitCORE":             {Type: "t", Data: json.RawMessage(`1`)},
		"TimeoutStopUSec":       {Type: "t", Data: json.RawMessage(`0`)},
		"NoNewPrivileges":       {Type: "b", Data: json.RawMessage(`false`)},
		"PrivateTmp":            {Type: "b", Data: json.RawMessage(`false`)},
		"ProtectControlGroups":  {Type: "b", Data: json.RawMessage(`false`)},
		"RestrictSUIDSGID":      {Type: "b", Data: json.RawMessage(`false`)},
		"LockPersonality":       {Type: "b", Data: json.RawMessage(`false`)},
		"RemainAfterExit":       {Type: "b", Data: json.RawMessage(`true`)},
		"ExecStartPost":         {Type: "a(sasbttttuii)", Data: json.RawMessage(`[["/bin/true",["/bin/true"],false,0,0,0,0,0,0,0]]`)},
		"ExecReload":            {Type: "a(sasbttttuii)", Data: json.RawMessage(`[["/bin/true",["/bin/true"],false,0,0,0,0,0,0,0]]`)},
		"ExecStop":              {Type: "a(sasbttttuii)", Data: json.RawMessage(`[["/bin/true",["/bin/true"],false,0,0,0,0,0,0,0]]`)},
	} {
		t.Run(name, func(t *testing.T) {
			unit, service, request, digest := platformEndpointFixture(t)
			service[name] = value
			if err := verifyPlatform(unit, service, request, digest); !errors.Is(err, ErrBinding) {
				t.Fatal("weaker platform confinement accepted", name, err)
			}
		})
	}
	for name, families := range map[string]managerValue{
		"deny":         {Type: "(bas)", Data: json.RawMessage(`[false,["AF_UNIX","AF_INET","AF_INET6"]]`)},
		"missing-unix": {Type: "(bas)", Data: json.RawMessage(`[true,["AF_INET","AF_INET6"]]`)},
		"extra":        {Type: "(bas)", Data: json.RawMessage(`[true,["AF_UNIX","AF_INET","AF_INET6","AF_NETLINK"]]`)},
		"wrong-type":   {Type: "as", Data: json.RawMessage(`["AF_UNIX"]`)},
	} {
		t.Run("RestrictAddressFamilies/"+name, func(t *testing.T) {
			unit, service, request, digest := platformEndpointFixture(t)
			service["RestrictAddressFamilies"] = families
			if err := verifyPlatform(unit, service, request, digest); !errors.Is(err, ErrBinding) {
				t.Fatal("weaker address family confinement accepted", name, err)
			}
		})
	}
}

func TestPlatformConfigurationRefusesForeignExecStart(t *testing.T) {
	for name, change := range map[string]func([][]json.RawMessage, Configuration, string){
		"foreign-program": func(exec [][]json.RawMessage, _ Configuration, _ string) {
			exec[0][0] = json.RawMessage(`"/foreign/ardents-linux-amd64"`)
		},
		"foreign-argv": func(exec [][]json.RawMessage, _ Configuration, _ string) {
			exec[0][1] = json.RawMessage(`["/installation/generations/` + strings.Repeat("01", 32) + `/ardents-linux-amd64","endpoint","start-installed","/foreign"]`)
		},
		"env-expand": func(exec [][]json.RawMessage, _ Configuration, _ string) {
			exec[0][2] = json.RawMessage(`["env-expand"]`)
		},
		"extra-fields": func(exec [][]json.RawMessage, request Configuration, digest string) {
			program := path.Join(request.InstallationRoot, "generations", digest, "ardents-linux-amd64")
			second := []json.RawMessage{json.RawMessage(`"` + program + `"`), json.RawMessage(`["` + program + `","endpoint","start-installed","` + request.InstallationRoot + `"]`), json.RawMessage(`["no-env-expand"]`), json.RawMessage(`0`), json.RawMessage(`0`), json.RawMessage(`0`), json.RawMessage(`0`), json.RawMessage(`0`), json.RawMessage(`0`), json.RawMessage(`0`)}
			exec[0] = append(exec[0], second...)
		},
	} {
		t.Run(name, func(t *testing.T) {
			unit, service, request, digest := platformEndpointFixture(t)
			var exec [][]json.RawMessage
			if err := json.Unmarshal(service["ExecStartEx"].Data, &exec); err != nil {
				t.Fatal(err)
			}
			change(exec, request, digest)
			raw, err := json.Marshal(exec)
			if err != nil {
				t.Fatal(err)
			}
			service["ExecStartEx"] = managerValue{Type: "a(sasasttttuii)", Data: raw}
			if err := verifyPlatform(unit, service, request, digest); !errors.Is(err, ErrBinding) {
				t.Fatal("foreign ExecStart accepted", name, err)
			}
		})
	}
	// Exactly one ExecStart entry is admitted; a second complete entry refuses.
	t.Run("two-entries", func(t *testing.T) {
		unit, service, request, digest := platformEndpointFixture(t)
		var exec [][]json.RawMessage
		if err := json.Unmarshal(service["ExecStartEx"].Data, &exec); err != nil {
			t.Fatal(err)
		}
		program := path.Join(request.InstallationRoot, "generations", digest, "ardents-linux-amd64")
		second := []json.RawMessage{json.RawMessage(`"` + program + `"`), json.RawMessage(`["` + program + `","endpoint","start-installed","` + request.InstallationRoot + `"]`), json.RawMessage(`["no-env-expand"]`), json.RawMessage(`0`), json.RawMessage(`0`), json.RawMessage(`0`), json.RawMessage(`0`), json.RawMessage(`0`), json.RawMessage(`0`), json.RawMessage(`0`)}
		raw, err := json.Marshal([][]json.RawMessage{exec[0], second})
		if err != nil {
			t.Fatal(err)
		}
		service["ExecStartEx"] = managerValue{Type: "a(sasasttttuii)", Data: raw}
		if err := verifyPlatform(unit, service, request, digest); !errors.Is(err, ErrBinding) {
			t.Fatal("second ExecStart entry accepted", err)
		}
	})
}

// The F-25 pre-start binding is the security core: exactly one privileged,
// no-env-expand ExecStartPre whose argv is the original platform check bound to
// this exact check program, manifest and launch digest. No substituted program,
// weakened flag, extra hook, environment expansion or foreign argument is admitted.
func TestPlatformConfigurationRefusesForeignPreStartCheck(t *testing.T) {
	pin := platformTestPin()
	argv := []string{platformTestCheckProgram, "platform", "check", platformTestManifest, pin}
	entry := func(program string, args []string, flags []string) []any {
		return []any{program, args, flags, 0, 0, 0, 0, 0, 0, 0}
	}
	for name, change := range map[string]func(managerProperties){
		"missing": func(s managerProperties) { delete(s, "ExecStartPreEx") },
		"empty": func(s managerProperties) {
			s["ExecStartPreEx"] = managerValue{Type: "a(sasasttttuii)", Data: json.RawMessage(`[]`)}
		},
		"wrong-type": func(s managerProperties) {
			s["ExecStartPreEx"] = managerValue{Type: "a(sasbttttuii)", Data: json.RawMessage(`[]`)}
		},
		"extra-hook": func(s managerProperties) {
			raw, _ := json.Marshal([]any{entry(platformTestCheckProgram, argv, []string{"privileged", "no-env-expand"}), entry("/bin/true", []string{"/bin/true"}, []string{"privileged", "no-env-expand"})})
			s["ExecStartPreEx"] = managerValue{Type: "a(sasasttttuii)", Data: raw}
		},
		"substituted-program": func(s managerProperties) {
			foreign := []string{"/foreign/ardents-linux-amd64", "platform", "check", platformTestManifest, pin}
			raw, _ := json.Marshal([]any{entry("/foreign/ardents-linux-amd64", foreign, []string{"privileged", "no-env-expand"})})
			s["ExecStartPreEx"] = managerValue{Type: "a(sasasttttuii)", Data: raw}
		},
		"unprivileged": func(s managerProperties) {
			raw, _ := json.Marshal([]any{entry(platformTestCheckProgram, argv, []string{"no-env-expand"})})
			s["ExecStartPreEx"] = managerValue{Type: "a(sasasttttuii)", Data: raw}
		},
		"env-expand": func(s managerProperties) {
			raw, _ := json.Marshal([]any{entry(platformTestCheckProgram, argv, []string{"privileged", "env-expand"})})
			s["ExecStartPreEx"] = managerValue{Type: "a(sasasttttuii)", Data: raw}
		},
		"no-flags": func(s managerProperties) {
			raw, _ := json.Marshal([]any{entry(platformTestCheckProgram, argv, []string{})})
			s["ExecStartPreEx"] = managerValue{Type: "a(sasasttttuii)", Data: raw}
		},
		"foreign-manifest": func(s managerProperties) {
			foreign := []string{platformTestCheckProgram, "platform", "check", "/etc/ardents/other.json", pin}
			raw, _ := json.Marshal([]any{entry(platformTestCheckProgram, foreign, []string{"privileged", "no-env-expand"})})
			s["ExecStartPreEx"] = managerValue{Type: "a(sasasttttuii)", Data: raw}
		},
		"foreign-pin": func(s managerProperties) {
			foreign := []string{platformTestCheckProgram, "platform", "check", platformTestManifest, strings.Repeat("cd", 32)}
			raw, _ := json.Marshal([]any{entry(platformTestCheckProgram, foreign, []string{"privileged", "no-env-expand"})})
			s["ExecStartPreEx"] = managerValue{Type: "a(sasasttttuii)", Data: raw}
		},
		"foreign-verb": func(s managerProperties) {
			foreign := []string{platformTestCheckProgram, "platform", "start", platformTestManifest, pin}
			raw, _ := json.Marshal([]any{entry(platformTestCheckProgram, foreign, []string{"privileged", "no-env-expand"})})
			s["ExecStartPreEx"] = managerValue{Type: "a(sasasttttuii)", Data: raw}
		},
		"argv-program-mismatch": func(s managerProperties) {
			raw, _ := json.Marshal([]any{entry(platformTestCheckProgram, append([]string{"/foreign/ardents-linux-amd64"}, argv[1:]...), []string{"privileged", "no-env-expand"})})
			s["ExecStartPreEx"] = managerValue{Type: "a(sasasttttuii)", Data: raw}
		},
	} {
		t.Run(name, func(t *testing.T) {
			unit, service, request, digest := platformEndpointFixture(t)
			change(service)
			if err := verifyPlatform(unit, service, request, digest); !errors.Is(err, ErrBinding) {
				t.Fatal("foreign platform pre-start check accepted", name, err)
			}
		})
	}
	// A check bound to a different launch digest or manifest refuses even when the
	// loaded pre-start check is otherwise well-formed.
	unit, service, request, digest := platformEndpointFixture(t)
	if err := VerifyPlatformConfiguration("255.4", unit, service, request, digest, platformTestCheckProgram, platformTestManifest, strings.Repeat("cd", 32)); !errors.Is(err, ErrBinding) {
		t.Fatal("pre-start check bound to a foreign launch digest accepted", err)
	}
	if err := VerifyPlatformConfiguration("255.4", unit, service, request, digest, platformTestCheckProgram, "/etc/ardents/other.json", pin); !errors.Is(err, ErrBinding) {
		t.Fatal("pre-start check bound to a foreign manifest accepted", err)
	}
	if err := VerifyPlatformConfiguration("255.4", unit, service, request, digest, "/foreign/ardents-linux-amd64", platformTestManifest, pin); !errors.Is(err, ErrBinding) {
		t.Fatal("pre-start check bound to a foreign check program accepted", err)
	}
}

// platformRunningEndpointFixture extends the platform configuration with the
// exact running invocation (active/running, this InvocationID/MainPID/control
// group, nonzero start clocks). It supplies no native manager or process proof.
func platformRunningEndpointFixture(t *testing.T) (managerProperties, managerProperties, Configuration, string, [16]byte) {
	t.Helper()
	unit, service, request, digest := platformEndpointFixture(t)
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

func verifyPlatformRunning(unit, service managerProperties, request Configuration, digest string, invocation [16]byte) error {
	return VerifyPlatformRunning("255.4", unit, service, request, digest, 123, invocation, platformTestCheckProgram, platformTestManifest, platformTestPin())
}

func TestPlatformRunningAdmitsSolePrivilegedPreStartCheck(t *testing.T) {
	unit, service, request, digest, invocation := platformRunningEndpointFixture(t)
	if err := verifyPlatformRunning(unit, service, request, digest, invocation); err != nil {
		t.Fatal("valid platform running invocation refused", err)
	}
	// The transitional running admission keeps ExecStartPre empty and refuses the
	// platform pre-start check; the two running modes do not cross-admit.
	if err := VerifyRunning("255.4", unit, service, request, digest, 123, invocation); !errors.Is(err, ErrBinding) {
		t.Fatal("transitional running admission accepted the platform pre-start check", err)
	}
	for _, change := range []string{"replacement-pid", "replacement-invocation", "different-cgroup", "stopped", "weak-protection", "hook", "foreign-executable", "foreign-pre-start", "zero-start-clock"} {
		t.Run(change, func(t *testing.T) {
			unit, service, request, digest, invocation := platformRunningEndpointFixture(t)
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
			case "foreign-pre-start":
				service["ExecStartPreEx"] = managerValue{Type: "a(sasasttttuii)", Data: json.RawMessage(`[["/foreign",["/foreign","platform","check","/etc/ardents/endpoint.json","` + platformTestPin() + `"],["privileged","no-env-expand"],0,0,0,0,0,0,0]]`)}
			case "zero-start-clock":
				var entries [][]json.RawMessage
				if err := json.Unmarshal(service["ExecStartEx"].Data, &entries); err != nil {
					t.Fatal(err)
				}
				entries[0][3] = json.RawMessage(`0`)
				raw, err := json.Marshal(entries)
				if err != nil {
					t.Fatal(err)
				}
				service["ExecStartEx"] = managerValue{Type: "a(sasasttttuii)", Data: raw}
			}
			if err := verifyPlatformRunning(unit, service, request, digest, invocation); !errors.Is(err, ErrBinding) {
				t.Fatal("changed platform running invocation/configuration accepted", change, err)
			}
		})
	}
}
