//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
)

func TestPlatformRetiresDetachedSourceStartBeforeEffects(t *testing.T) {
	var out bytes.Buffer
	if code := runInfrastructure(context.Background(), []string{"start-source", "/absent/distribution", "/absent/trust", "closed", "private", "/absent/binding"}, &out); code != 2 || out.Len() != 0 {
		t.Fatal("retired Source startup entered Installation", code, out.String())
	}
}

func TestPlatformAccountRefusesPrivilegedAndSharedPrincipals(t *testing.T) {
	passwd := "root:x:0:0:root:/root:/bin/sh\nardents-node:x:999:999::/nonexistent:/usr/sbin/nologin\n"
	group := "root:x:0:\nardents-node:x:999:\n"
	if !platformAccount([]byte(passwd), []byte(group), "ardents-node") {
		t.Fatal("fixed unprivileged account refused")
	}
	for _, changed := range []string{strings.ReplaceAll(passwd, "999", "0"), passwd + "alias:x:999:999::/nonexistent:/usr/sbin/nologin\n", strings.ReplaceAll(passwd, "/usr/sbin/nologin", "/bin/sh"), passwd + "ardents-node:x:999:999::/nonexistent:/usr/sbin/nologin\n"} {
		if platformAccount([]byte(changed), []byte(group), "ardents-node") {
			t.Fatal("privileged/shared account accepted")
		}
	}
}

func platformTestLaunch(role string) platformLaunch {
	base := "/var/lib/ardents/roles/" + role
	roots := []string{base + "/state", base + "/participation", base + "/spend", base + "/hosting"}
	if role == "introduction" {
		roots = append(roots, base+"/slots")
	}
	inputs := map[string]string{"/etc/ardents/node.crt": strings.Repeat("c", 64), "/etc/ardents/node.key": strings.Repeat("d", 64)}
	if role == "source" {
		inputs["/etc/ardents/client-ca.pem"] = strings.Repeat("e", 64)
	}
	return platformLaunch{Role: role, Distribution: "/var/lib/ardents/artifacts/release-one", ReleaseRoot: "/var/lib/ardents/release-trust", Environment: "closed", Network: "private", Program: "/usr/lib/ardents/release-one/ardents-next", Configuration: "/etc/ardents/" + role + ".json", ConfigSHA256: strings.Repeat("a", 64), WritableRoots: roots, Inputs: inputs}
}

func TestPlatformLaunchRejectsExpansionAndWritableTrust(t *testing.T) {
	for name, change := range map[string]func(*platformLaunch){
		"root-escape":   func(p *platformLaunch) { p.WritableRoots[0] = "/var/lib/ardents" },
		"role-escape":   func(p *platformLaunch) { p.Role = "other" },
		"specifier":     func(p *platformLaunch) { p.Program += "%i" },
		"environment":   func(p *platformLaunch) { p.Configuration += "$X" },
		"newline":       func(p *platformLaunch) { p.Distribution += "\nExecStart=/bin/sh" },
		"overlap":       func(p *platformLaunch) { p.WritableRoots[1] = p.WritableRoots[0] + "/child" },
		"trust":         func(p *platformLaunch) { p.ReleaseRoot = p.WritableRoots[0] },
		"artifact":      func(p *platformLaunch) { p.Distribution = p.WritableRoots[0] },
		"configuration": func(p *platformLaunch) { p.Configuration = p.WritableRoots[0] + "/plan" },
		"pin":           func(p *platformLaunch) { p.ConfigSHA256 = strings.Repeat("z", 64) },
		"mutable-key": func(p *platformLaunch) {
			delete(p.Inputs, "/etc/ardents/node.key")
			p.Inputs[p.WritableRoots[0]+"/node.key"] = strings.Repeat("d", 64)
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := platformTestLaunch("introduction")
			if !validPlatformLaunch(p) {
				t.Fatal("healthy declaration refused")
			}
			change(&p)
			if validPlatformLaunch(p) {
				t.Fatal("unsafe platform declaration accepted")
			}
		})
	}
}

func TestPlatformConfigurationBindsEveryCredentialAndSourceTrustInput(t *testing.T) {
	raw := []byte(`{"certificate":"/etc/ardents/node.crt","private_key":"/etc/ardents/node.key","client_root":"/etc/ardents/client-ca.pem"}`)
	paths, err := platformPlanInputs("source", raw)
	if err != nil || len(paths) != 3 || paths[2] != "/etc/ardents/client-ca.pem" {
		t.Fatal("Source trust input omitted", paths, err)
	}
	for _, bad := range []string{`{"certificate":"/etc/ardents/node.crt","private_key":"/etc/ardents/node.key"}`, `{"certificate":"/etc/ardents/node.crt","private_key":"/etc/ardents/node.key","client_root":"/var/lib/ardents/roles/source/state/ca.pem"}`} {
		if _, err := platformPlanInputs("source", []byte(bad)); err == nil {
			t.Fatal("unbound/mutable Source trust admitted")
		}
	}
}

func TestPlatformUnitRunsExistingRetainedRuntimeDirectly(t *testing.T) {
	for _, role := range []string{"source", "forwarding", "issuer", "descriptor", "introduction", "join"} {
		p := platformTestLaunch(role)
		unit := string(renderPlatformUnit(p, "/etc/ardents/launch.json", strings.Repeat("b", 64)))
		for _, required := range []string{"ExecStartPre=+:/var/lib/ardents/artifacts/release-one/ardents-node-linux-amd64 platform check /etc/ardents/launch.json ", "Restart=no\n", "KillMode=control-group\n", "TimeoutStopSec=infinity\n", "SendSIGKILL=no\n", "NoNewPrivileges=yes\n", "CapabilityBoundingSet=\n"} {
			if !strings.Contains(unit, required) {
				t.Fatalf("%s lacks %q", role, required)
			}
		}
		command := " node " + role + " --retained-config " + p.Configuration
		if role == "source" {
			command = " source serve-retained " + p.Configuration
		}
		if !strings.Contains(unit, "ExecStart=:"+p.Program+command+"\n") || strings.Contains(unit, "start-installed") || strings.Contains(unit, "@") {
			t.Fatal("unit delegates to Installation or leaves substitution", role, unit)
		}
	}
	var out bytes.Buffer
	if runPlatform(t.Context(), []string{"template"}, &out) != 0 || out.String() != platformUnitTemplate {
		t.Fatal("signed-template byte producer differs")
	}
}

func TestPlatformPlanWriteProjectionCannotWidenRoots(t *testing.T) {
	p := platformTestLaunch("introduction")
	fields := map[string]any{"network": networkAuthorityPlan{Root: p.WritableRoots[0], Authorities: [][32]byte{{1}}}, "participation_root": p.WritableRoots[1], "spend_root": p.WritableRoots[2], "hosting_root": p.WritableRoots[3], "introduction_root": p.WritableRoots[4]}
	raw, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	roots, err := platformPlanRoots(p.Role, raw)
	if err != nil || !equalPlatformRoots(roots, p.WritableRoots) {
		t.Fatal("original domain roots differ", roots, err)
	}
	if equalPlatformRoots(roots, append(roots, "/etc")) {
		t.Fatal("extra platform write permission accepted")
	}
	duplicate := append([]byte(`{"spend_root":"/foreign",`), raw[1:]...)
	if _, err := platformPlanRoots(p.Role, duplicate); err == nil {
		t.Fatal("duplicate configuration field accepted")
	}
}

// These typed fixtures test only the pure comparison of loaded policy. They
// supply no actual manager, root precheck, Release proof or installed acceptance.
func platformLoadedFixture(p platformLaunch) (systemd.Properties, systemd.Properties) {
	u, s := systemd.Properties{}, systemd.Properties{}
	set := func(dst systemd.Properties, name, signature string, value any) {
		raw, _ := json.Marshal(value)
		dst[name] = systemd.Value{Type: signature, Data: raw}
	}
	for name, value := range map[string]string{"Id": "ardents-" + p.Role + ".service", "LoadState": "loaded", "FragmentPath": "/etc/systemd/system/ardents-" + p.Role + ".service", "UnitFileState": "static", "ActiveState": "activating", "SubState": "start-pre"} {
		set(u, name, "s", value)
	}
	set(u, "DropInPaths", "as", []string{})
	set(u, "TriggeredBy", "as", []string{})
	set(u, "InvocationID", "ay", []int{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	for name, value := range map[string]string{"User": "ardents-node", "Group": "ardents-node", "Type": "exec", "WorkingDirectory": "/", "ProtectHome": "yes", "ProtectSystem": "strict", "KillMode": "control-group", "Restart": "no", "RootDirectory": "", "RootImage": "", "ExitType": "main", "StandardInput": "null", "StandardOutput": "journal", "StandardError": "journal", "ControlGroup": "/system.slice/ardents-" + p.Role + ".service"} {
		set(s, name, "s", value)
	}
	for name, value := range map[string]uint64{"CapabilityBoundingSet": 0, "AmbientCapabilities": 0, "LimitCORE": 0, "LimitCORESoft": 0, "LimitNOFILE": 1024, "LimitNOFILESoft": 1024, "CPUQuotaPerSecUSec": 1000000, "CPUQuotaPeriodUSec": 100000, "MemoryHigh": 805306368, "MemoryMax": 1073741824, "TasksMax": 256, "TimeoutStopUSec": math.MaxUint64} {
		set(s, name, "t", value)
	}
	for _, name := range []string{"NoNewPrivileges", "PrivateTmp", "ProtectControlGroups", "ProtectKernelTunables", "ProtectKernelModules", "ProtectKernelLogs", "RestrictSUIDSGID", "LockPersonality", "MemoryAccounting", "CPUAccounting", "TasksAccounting"} {
		set(s, name, "b", true)
	}
	for _, name := range []string{"SendSIGKILL", "RemainAfterExit", "DynamicUser"} {
		set(s, name, "b", false)
	}
	for name, value := range map[string]uint32{"UMask": 0077, "MainPID": 0, "ControlPID": 123, "NRestarts": 0} {
		set(s, name, "u", value)
	}
	set(s, "SupplementaryGroups", "as", []string{})
	set(s, "ReadWritePaths", "as", p.WritableRoots)
	set(s, "Environment", "as", []string{"GOMAXPROCS=1", "GOMEMLIMIT=536870912"})
	for name, signature := range map[string]string{"EnvironmentFiles": "a(sb)", "BindPaths": "a(ssbt)", "BindReadOnlyPaths": "a(ssbt)", "TemporaryFileSystem": "a(ss)", "ReadOnlyPaths": "as", "InaccessiblePaths": "as", "ExecSearchPath": "as", "ExecCondition": "a(sasbttttuii)", "ExecStartPost": "a(sasbttttuii)", "ExecReload": "a(sasbttttuii)", "ExecStop": "a(sasbttttuii)", "ExecStopPost": "a(sasbttttuii)"} {
		set(s, name, signature, []any{})
	}
	set(s, "RestrictAddressFamilies", "(bas)", []any{true, []string{"AF_UNIX", "AF_INET", "AF_INET6"}})
	set(s, "SuccessExitStatus", "(aiai)", []any{[]int{130}, []int{}})
	check := filepath.Join(p.Distribution, "ardents-node-linux-amd64")
	for name, value := range map[string][]any{
		"ExecStartEx":    {p.Program, []string{p.Program, "node", "introduction", "--retained-config", p.Configuration}, []string{"no-env-expand"}, 0, 0, 0, 0, 0, 0, 0},
		"ExecStartPreEx": {check, []string{check, "platform", "check", "/etc/ardents/launch.json", strings.Repeat("b", 64)}, []string{"privileged", "no-env-expand"}, 1, 1, 0, 0, 123, 0, 0},
	} {
		set(s, name, "a(sasasttttuii)", []any{value})
	}
	return u, s
}

func TestPlatformPrestartRejectsDifferentLoadedInvocationAndPolicy(t *testing.T) {
	p := platformTestLaunch("introduction")
	for _, scenario := range []string{"healthy", "foreign-control-pid", "dropin", "unconfined", "automatic-restart", "forced-kill", "wrong-config", "ignored-precheck", "extra-command", "writable-trust"} {
		t.Run(scenario, func(t *testing.T) {
			u, s := platformLoadedFixture(p)
			set := func(dst systemd.Properties, name, sig string, value any) {
				raw, _ := json.Marshal(value)
				dst[name] = systemd.Value{Type: sig, Data: raw}
			}
			manifest := "/etc/ardents/launch.json"
			switch scenario {
			case "foreign-control-pid":
				set(s, "ControlPID", "u", 124)
			case "dropin":
				set(u, "DropInPaths", "as", []string{"/etc/systemd/system/override.conf"})
			case "unconfined":
				set(s, "ProtectSystem", "s", "no")
			case "automatic-restart":
				set(s, "Restart", "s", "always")
			case "forced-kill":
				set(s, "SendSIGKILL", "b", true)
			case "wrong-config":
				manifest = "/etc/ardents/other.json"
			case "ignored-precheck":
				s["ExecStartPreEx"] = systemd.Value{Type: "a(sasasttttuii)", Data: bytes.Replace(s["ExecStartPreEx"].Data, []byte(`"privileged"`), []byte(`"ignore-failure"`), 1)}
			case "extra-command":
				set(s, "ExecStartPost", "a(sasbttttuii)", []any{[]any{"/bin/true"}})
			case "writable-trust":
				set(s, "ReadWritePaths", "as", append(append([]string{}, p.WritableRoots...), p.ReleaseRoot))
			}
			err := verifyPlatformPrestart(u, s, p, manifest, strings.Repeat("b", 64), 123)
			if (err == nil) != (scenario == "healthy") {
				t.Fatal("loaded policy result", scenario, err)
			}
		})
	}
}
