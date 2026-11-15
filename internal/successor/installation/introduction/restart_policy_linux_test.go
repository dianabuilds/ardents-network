package introduction

import (
	"encoding/hex"
	"encoding/json"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"path/filepath"
	"strings"
	"testing"
)

// Typed detached native observations exercise only the policy. They cannot
// supply the independent original history/process/scope custody used by Restart.
func TestIntroductionRestartCollectedUnitStillRequiresOriginalRetirement(t *testing.T) {
	r := Request{InstallationRoot: "/opt/node", StateRoot: "/opt/state", ParticipationRoot: "/opt/participation", SpendRoot: "/opt/spend", HostingRoot: "/opt/hosting", IntroductionRoot: "/opt/slots"}
	generation := strings.Repeat("a", 64)
	unit, service := systemd.Properties{}, systemd.Properties{}
	put := func(m systemd.Properties, name, kind string, v any) {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		m[name] = systemd.Value{Type: kind, Data: raw}
	}
	for name, v := range map[string]string{"Id": "ardents-introduction.service", "LoadState": "loaded", "FragmentPath": "/etc/systemd/system/ardents-introduction.service", "UnitFileState": "static", "ActiveState": "inactive", "SubState": "dead"} {
		put(unit, name, "s", v)
	}
	put(unit, "DropInPaths", "as", []string{})
	put(unit, "TriggeredBy", "as", []string{})
	put(unit, "Job", "(uo)", []any{uint32(0), "/"})
	put(unit, "InvocationID", "ay", []any{})
	for name, v := range map[string]string{"User": "ardents-node", "Group": "ardents-node", "Type": "exec", "WorkingDirectory": "/", "ProtectHome": "yes", "ProtectSystem": "strict", "KillMode": "control-group", "Restart": "no", "RootDirectory": "", "RootImage": "", "ExitType": "main", "RestartMode": "normal", "StandardOutput": "journal", "StandardError": "journal", "StandardInput": "null", "ControlGroup": ""} {
		put(service, name, "s", v)
	}
	for name, v := range map[string]uint64{"CapabilityBoundingSet": 0, "AmbientCapabilities": 0, "LimitCORE": 0, "LimitCORESoft": 0, "LimitNOFILE": 1024, "LimitNOFILESoft": 1024, "CPUQuotaPerSecUSec": 1000000, "CPUQuotaPeriodUSec": 100000, "MemoryHigh": 805306368, "MemoryMax": 1073741824, "TasksMax": 256, "TimeoutStopUSec": 30000000, "ExecMainStartTimestamp": 0, "ExecMainStartTimestampMonotonic": 0, "ExecMainExitTimestamp": 0, "ExecMainExitTimestampMonotonic": 0} {
		put(service, name, "t", v)
	}
	for _, name := range []string{"NoNewPrivileges", "PrivateTmp", "ProtectControlGroups", "ProtectKernelTunables", "ProtectKernelModules", "ProtectKernelLogs", "RestrictSUIDSGID", "LockPersonality", "MemoryAccounting", "CPUAccounting", "TasksAccounting"} {
		put(service, name, "b", true)
	}
	for _, name := range []string{"RemainAfterExit", "DynamicUser"} {
		put(service, name, "b", false)
	}
	for _, name := range []string{"MainPID", "ExecMainPID", "ControlPID", "NRestarts"} {
		put(service, name, "u", uint32(0))
	}
	for _, name := range []string{"ExecMainCode", "ExecMainStatus"} {
		put(service, name, "i", int32(0))
	}
	put(service, "UMask", "u", uint32(0077))
	put(service, "SupplementaryGroups", "as", []string{})
	put(service, "ReadWritePaths", "as", []string{r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot, r.IntroductionRoot})
	put(service, "Environment", "as", []string{"GOMAXPROCS=1", "GOMEMLIMIT=536870912"})
	for _, name := range []string{"ExecCondition", "ExecStartPre", "ExecStartPost", "ExecReload", "ExecStop", "ExecStopPost"} {
		put(service, name, "a(sasbttttuii)", []any{})
	}
	put(service, "RestrictAddressFamilies", "(bas)", []any{true, []string{"AF_INET", "AF_INET6", "AF_UNIX"}})
	program := filepath.Join(r.InstallationRoot, "generations", generation, "ardents-node-linux-amd64")
	put(service, "ExecStartEx", "a(sasasttttuii)", []any{[]any{program, []string{program, "introduction", "start-installed", "/opt/node/binding.json"}, []string{"no-env-expand"}, 0, 0, 0, 0, 0, 0, 0}})
	id := [16]byte{1}
	archive := startArchive{Generation: generation, InvocationID: hex.EncodeToString(id[:]), PID: 1024, StartedRealtime: 100, StartedMonotonic: 50}
	retired := introductionExecution{pid: 1024, startedRealtime: 100, startedMonotonic: 50, exitedRealtime: 120, exitedMonotonic: 70, code: 1}
	check := func() error {
		return verifyIntroductionRestartStopped(unit, service, r, generation, archive, id, retired)
	}
	if err := check(); err != nil {
		t.Fatal("fully collected unit refused despite independently required retired execution", err)
	}
	original := retired
	retired = introductionExecution{}
	if check() == nil {
		t.Fatal("zero current execution manufactured retirement")
	}
	retired = original
	retired.status = 1
	if check() == nil {
		t.Fatal("failed archived exit accepted")
	}
	retired = original
	for _, name := range []string{"MainPID", "ControlPID", "NRestarts"} {
		put(service, name, "u", uint32(1))
		if check() == nil {
			t.Fatal("active/unknown invocation accepted", name)
		}
		put(service, name, "u", uint32(0))
	}
	put(unit, "InvocationID", "ay", [16]byte{2})
	if check() == nil {
		t.Fatal("foreign current invocation accepted")
	}
}
