package systemd

import (
	"bytes"
	"encoding/json"
	"math"
	"path/filepath"
	"strings"
	"testing"
)

// These typed fixtures test only the pure comparison of loaded policy. They
// supply no actual manager, root precheck, Release proof or installed acceptance.
func infrastructureLoadedFixture(p InfrastructureUnit) (Properties, Properties) {
	u, s := Properties{}, Properties{}
	set := func(dst Properties, name, signature string, value any) {
		raw, _ := json.Marshal(value)
		dst[name] = Value{Type: signature, Data: raw}
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
	set(s, "RestrictAddressFamilies", "(bas)", []any{true, []string{"AF_INET", "AF_INET6", "AF_UNIX"}})
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
	p := InfrastructureUnit{
		Role: "introduction", Distribution: "/var/lib/ardents/artifacts/release-one",
		Program: "/usr/lib/ardents/intro/ardents-next", Configuration: "/etc/ardents/introduction.json",
		Manifest: "/etc/ardents/launch.json", ManifestSHA256: strings.Repeat("b", 64),
		WritableRoots: []string{"/var/lib/ardents/roles/introduction/state", "/var/lib/ardents/roles/introduction/slots"},
	}
	for _, scenario := range []string{"healthy", "foreign-control-pid", "dropin", "unconfined", "automatic-restart", "forced-kill", "wrong-config", "ignored-precheck", "extra-command", "writable-trust", "foreign-address-family"} {
		t.Run(scenario, func(t *testing.T) {
			u, s := infrastructureLoadedFixture(p)
			set := func(dst Properties, name, sig string, value any) {
				raw, _ := json.Marshal(value)
				dst[name] = Value{Type: sig, Data: raw}
			}
			expected := p
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
				expected.Manifest = "/etc/ardents/other.json"
			case "ignored-precheck":
				s["ExecStartPreEx"] = Value{Type: "a(sasasttttuii)", Data: bytes.Replace(s["ExecStartPreEx"].Data, []byte(`"privileged"`), []byte(`"ignore-failure"`), 1)}
			case "extra-command":
				set(s, "ExecStartPost", "a(sasbttttuii)", []any{[]any{"/bin/true"}})
			case "writable-trust":
				set(s, "ReadWritePaths", "as", append(append([]string{}, p.WritableRoots...), "/var/lib/ardents/release-trust"))
			case "foreign-address-family":
				set(s, "RestrictAddressFamilies", "(bas)", []any{true, []string{"AF_INET", "AF_INET6", "AF_NETLINK", "AF_UNIX"}})
			}
			err := expected.verifyPrestart(u, s, 123)
			if (err == nil) != (scenario == "healthy") {
				t.Fatal("loaded policy result", scenario, err)
			}
		})
	}
}
