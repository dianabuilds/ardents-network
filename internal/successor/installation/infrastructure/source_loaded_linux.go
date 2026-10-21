package infrastructure

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
)

// verifySourceStopped is the Source policy oracle over fresh complete native
// dictionaries. It does not borrow Endpoint configuration or process authority.
func verifySourceStopped(unit, service systemd.Properties, r SourceRequest, generation string) error {
	for name, wanted := range map[string]string{"Id": "ardents-source.service", "LoadState": "loaded", "ActiveState": "inactive", "SubState": "dead", "FragmentPath": "/etc/systemd/system/ardents-source.service", "UnitFileState": "static"} {
		if !systemd.Matches(unit, name, "s", wanted) {
			return fmt.Errorf("source Unit property %s: %w", name, ErrBinding)
		}
	}
	for name, wanted := range map[string]struct {
		signature string
		value     any
	}{
		"DropInPaths": {"as", []string{}}, "Job": {"(uo)", []any{uint32(0), "/"}}, "InvocationID": {"ay", []any{}},
	} {
		if !systemd.Matches(unit, name, wanted.signature, wanted.value) {
			return fmt.Errorf("source Unit property %s: %w", name, ErrBinding)
		}
	}
	for name, wanted := range map[string]string{"User": "ardents-source", "Group": "ardents-source", "Type": "exec", "WorkingDirectory": "/", "ProtectHome": "yes", "ProtectSystem": "strict", "KillMode": "control-group", "Restart": "no", "RootDirectory": "", "RootImage": "", "ExitType": "main", "RestartMode": "normal", "StandardOutput": "journal", "StandardError": "journal", "ControlGroup": ""} {
		if !systemd.Matches(service, name, "s", wanted) {
			return fmt.Errorf("source Service property %s: %w", name, ErrBinding)
		}
	}
	for name, wanted := range map[string]uint64{"CapabilityBoundingSet": 0, "AmbientCapabilities": 0, "LimitCORE": 0, "LimitCORESoft": 0, "LimitNOFILE": 1024, "LimitNOFILESoft": 1024, "CPUQuotaPerSecUSec": 1_000_000, "CPUQuotaPeriodUSec": 100_000, "MemoryHigh": 805306368, "MemoryMax": 1073741824, "TasksMax": 256, "TimeoutStopUSec": 30_000_000} {
		if !systemd.Matches(service, name, "t", wanted) {
			return fmt.Errorf("source Service property %s: %w", name, ErrBinding)
		}
	}
	for _, name := range []string{"NoNewPrivileges", "PrivateTmp", "ProtectControlGroups", "ProtectKernelTunables", "ProtectKernelModules", "ProtectKernelLogs", "RestrictSUIDSGID", "LockPersonality", "MemoryAccounting", "CPUAccounting", "TasksAccounting"} {
		if !systemd.Matches(service, name, "b", true) {
			return fmt.Errorf("source Service property %s: %w", name, ErrBinding)
		}
	}
	if !systemd.Matches(service, "MainPID", "u", uint32(0)) || !systemd.Matches(service, "ExecMainPID", "u", uint32(0)) || !systemd.Matches(service, "UMask", "u", uint32(0077)) || !systemd.Matches(service, "RemainAfterExit", "b", false) || !systemd.Matches(service, "SupplementaryGroups", "as", []string{}) || !systemd.Matches(service, "ReadWritePaths", "as", []string{r.StateRoot, r.ParticipationRoot}) || !systemd.Matches(service, "Environment", "as", []string{"GOMAXPROCS=1", "GOMEMLIMIT=536870912"}) {
		return ErrBinding
	}
	for _, name := range []string{"ExecCondition", "ExecStartPre", "ExecStartPost", "ExecReload", "ExecStop", "ExecStopPost"} {
		if !systemd.Matches(service, name, "a(sasbttttuii)", []any{}) {
			return ErrBinding
		}
	}
	families := service["RestrictAddressFamilies"]
	var parts []json.RawMessage
	var allow bool
	var names []string
	if families.Type != "(bas)" || systemd.Decode(families.Data, &parts) != nil || len(parts) != 2 || systemd.Decode(parts[0], &allow) != nil || !allow || systemd.Decode(parts[1], &names) != nil {
		return ErrBinding
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"AF_INET", "AF_INET6", "AF_UNIX"}) {
		return ErrBinding
	}
	exec := service["ExecStartEx"]
	var entries [][]json.RawMessage
	if exec.Type != "a(sasasttttuii)" || systemd.Decode(exec.Data, &entries) != nil || len(entries) != 1 || len(entries[0]) != 10 {
		return ErrBinding
	}
	program := filepath.Join(r.InstallationRoot, "generations", generation, ProgramName)
	for index, wanted := range map[int]any{0: program, 1: []string{program, "source", "start-installed", filepath.Join(r.InstallationRoot, "binding.json")}, 2: []string{"no-env-expand"}} {
		p := systemd.Properties{"value": {Type: "v", Data: entries[0][index]}}
		if !systemd.Matches(p, "value", "v", wanted) {
			return ErrBinding
		}
	}
	for _, part := range entries[0][3:] {
		var number uint64
		if systemd.Decode(part, &number) != nil || number != 0 {
			return ErrBinding
		}
	}
	return nil
}
