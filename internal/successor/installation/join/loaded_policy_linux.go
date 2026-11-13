package join

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
)

// JOIN configuration policy is shared by its separate stopped and running
// comparisons. Neither fabricates another phase's native facts.
func verifyJoinConfiguration(unit, service systemd.Properties, r Request, generation string) error {
	for name, wanted := range map[string]string{"Id": "ardents-join.service", "LoadState": "loaded", "FragmentPath": "/etc/systemd/system/ardents-join.service", "UnitFileState": "static"} {
		if !systemd.Matches(unit, name, "s", wanted) {
			return fmt.Errorf("join Unit property %s: %w", name, ErrBinding)
		}
	}
	for name, wanted := range map[string]struct {
		signature string
		value     any
	}{
		"DropInPaths": {"as", []string{}}, "Job": {"(uo)", []any{uint32(0), "/"}},
	} {
		if !systemd.Matches(unit, name, wanted.signature, wanted.value) {
			return fmt.Errorf("join Unit property %s: %w", name, ErrBinding)
		}
	}
	for name, wanted := range map[string]string{"User": "ardents-node", "Group": "ardents-node", "Type": "exec", "WorkingDirectory": "/", "ProtectHome": "yes", "ProtectSystem": "strict", "KillMode": "control-group", "Restart": "no", "RootDirectory": "", "RootImage": "", "ExitType": "main", "RestartMode": "normal", "StandardOutput": "journal", "StandardError": "journal"} {
		if !systemd.Matches(service, name, "s", wanted) {
			return fmt.Errorf("join Service property %s: %w", name, ErrBinding)
		}
	}
	for name, wanted := range map[string]uint64{"CapabilityBoundingSet": 0, "AmbientCapabilities": 0, "LimitCORE": 0, "LimitCORESoft": 0, "LimitNOFILE": 1024, "LimitNOFILESoft": 1024, "CPUQuotaPerSecUSec": 1_000_000, "CPUQuotaPeriodUSec": 100_000, "MemoryHigh": 805306368, "MemoryMax": 1073741824, "TasksMax": 256, "TimeoutStopUSec": 30_000_000} {
		if !systemd.Matches(service, name, "t", wanted) {
			return fmt.Errorf("join Service property %s: %w", name, ErrBinding)
		}
	}
	for _, name := range []string{"NoNewPrivileges", "PrivateTmp", "ProtectControlGroups", "ProtectKernelTunables", "ProtectKernelModules", "ProtectKernelLogs", "RestrictSUIDSGID", "LockPersonality", "MemoryAccounting", "CPUAccounting", "TasksAccounting"} {
		if !systemd.Matches(service, name, "b", true) {
			return fmt.Errorf("join Service property %s: %w", name, ErrBinding)
		}
	}
	if !systemd.Matches(service, "UMask", "u", uint32(0077)) || !systemd.Matches(service, "RemainAfterExit", "b", false) || !systemd.Matches(service, "SupplementaryGroups", "as", []string{}) || !systemd.Matches(service, "ReadWritePaths", "as", []string{r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot}) || !systemd.Matches(service, "Environment", "as", []string{"GOMAXPROCS=1", "GOMEMLIMIT=536870912"}) {
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
	program := filepath.Join(r.InstallationRoot, "generations", generation, programName)
	for index, wanted := range map[int]any{0: program, 1: []string{program, "join", "start-installed", filepath.Join(r.InstallationRoot, "binding.json")}, 2: []string{"no-env-expand"}} {
		p := systemd.Properties{"value": {Type: "v", Data: entries[0][index]}}
		if !systemd.Matches(p, "value", "v", wanted) {
			return ErrBinding
		}
	}
	return nil
}

func verifyJoinStopped(unit, service systemd.Properties, r Request, generation string) error {
	if err := verifyJoinConfiguration(unit, service, r, generation); err != nil {
		return err
	}
	if !systemd.Matches(unit, "ActiveState", "s", "inactive") || !systemd.Matches(unit, "SubState", "s", "dead") ||
		!systemd.Matches(unit, "InvocationID", "ay", []any{}) || !systemd.Matches(service, "ControlGroup", "s", "") ||
		!systemd.Matches(service, "MainPID", "u", uint32(0)) || !systemd.Matches(service, "ExecMainPID", "u", uint32(0)) {
		return ErrBinding
	}
	status, err := joinExecutionStatus(service)
	if err != nil || status != (joinExecution{}) {
		return ErrBinding
	}
	return nil
}

type joinExecution struct {
	startedRealtime, startedMonotonic, exitedRealtime, exitedMonotonic uint64
	pid                                                                uint32
	code, status                                                       int32
}

func joinExecutionStatus(service systemd.Properties) (result joinExecution, err error) {
	value := service["ExecStartEx"]
	var entries [][]json.RawMessage
	if value.Type != "a(sasasttttuii)" || systemd.Decode(value.Data, &entries) != nil || len(entries) != 1 || len(entries[0]) != 10 {
		return result, ErrBinding
	}
	// ExecStartEx initially carries the command's fork clock. systemd replaces
	// it with the main process clock when reaping that same process. Retain the
	// separate ExecMain clocks, which identify the process throughout its life.
	var command joinExecution
	for index, target := range []any{&command.startedRealtime, &command.startedMonotonic, &command.exitedRealtime, &command.exitedMonotonic, &command.pid, &command.code, &command.status} {
		if systemd.Decode(entries[0][index+3], target) != nil {
			return joinExecution{}, ErrBinding
		}
	}
	for _, field := range []struct {
		name, signature string
		target          any
	}{
		{"ExecMainStartTimestamp", "t", &result.startedRealtime},
		{"ExecMainStartTimestampMonotonic", "t", &result.startedMonotonic},
		{"ExecMainExitTimestamp", "t", &result.exitedRealtime},
		{"ExecMainExitTimestampMonotonic", "t", &result.exitedMonotonic},
		{"ExecMainPID", "u", &result.pid},
		{"ExecMainCode", "i", &result.code},
		{"ExecMainStatus", "i", &result.status},
	} {
		value, present := service[field.name]
		if !present || value.Type != field.signature || systemd.Decode(value.Data, field.target) != nil {
			return joinExecution{}, ErrBinding
		}
	}
	if command.pid != result.pid || command.code != result.code || command.status != result.status ||
		command.exitedRealtime != result.exitedRealtime || command.exitedMonotonic != result.exitedMonotonic ||
		(command.startedRealtime == 0) != (result.startedRealtime == 0) || (command.startedMonotonic == 0) != (result.startedMonotonic == 0) ||
		command.startedRealtime > result.startedRealtime || command.startedMonotonic > result.startedMonotonic {
		return joinExecution{}, ErrBinding
	}
	if result.code != 0 && (command.startedRealtime != result.startedRealtime || command.startedMonotonic != result.startedMonotonic) {
		return joinExecution{}, ErrBinding
	}
	return result, nil
}
