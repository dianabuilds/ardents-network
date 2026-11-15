package introduction

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/successor/systemdproperty"
)

// Introduction configuration policy is shared by its separate stopped and running
// comparisons. Neither fabricates another phase's native facts.
func verifyIntroductionConfiguration(unit, service systemdproperty.Properties, r Request, generation string) error {
	for name, wanted := range map[string]string{"Id": "ardents-introduction.service", "LoadState": "loaded", "FragmentPath": "/etc/systemd/system/ardents-introduction.service", "UnitFileState": "static"} {
		if !systemdproperty.Matches(unit, name, "s", wanted) {
			return fmt.Errorf("introduction Unit property %s: %w", name, ErrBinding)
		}
	}
	for name, wanted := range map[string]struct {
		signature string
		value     any
	}{
		"DropInPaths": {"as", []string{}}, "Job": {"(uo)", []any{uint32(0), "/"}},
	} {
		if !systemdproperty.Matches(unit, name, wanted.signature, wanted.value) {
			return fmt.Errorf("introduction Unit property %s: %w", name, ErrBinding)
		}
	}
	for name, wanted := range map[string]string{"User": "ardents-node", "Group": "ardents-node", "Type": "exec", "WorkingDirectory": "/", "ProtectHome": "yes", "ProtectSystem": "strict", "KillMode": "control-group", "Restart": "no", "RootDirectory": "", "RootImage": "", "ExitType": "main", "RestartMode": "normal", "StandardOutput": "journal", "StandardError": "journal"} {
		if !systemdproperty.Matches(service, name, "s", wanted) {
			return fmt.Errorf("introduction Service property %s: %w", name, ErrBinding)
		}
	}
	for name, wanted := range map[string]uint64{"CapabilityBoundingSet": 0, "AmbientCapabilities": 0, "LimitCORE": 0, "LimitCORESoft": 0, "LimitNOFILE": 1024, "LimitNOFILESoft": 1024, "CPUQuotaPerSecUSec": 1_000_000, "CPUQuotaPeriodUSec": 100_000, "MemoryHigh": 805306368, "MemoryMax": 1073741824, "TasksMax": 256, "TimeoutStopUSec": 30_000_000} {
		if !systemdproperty.Matches(service, name, "t", wanted) {
			return fmt.Errorf("introduction Service property %s: %w", name, ErrBinding)
		}
	}
	for _, name := range []string{"NoNewPrivileges", "PrivateTmp", "ProtectControlGroups", "ProtectKernelTunables", "ProtectKernelModules", "ProtectKernelLogs", "RestrictSUIDSGID", "LockPersonality", "MemoryAccounting", "CPUAccounting", "TasksAccounting"} {
		if !systemdproperty.Matches(service, name, "b", true) {
			return fmt.Errorf("introduction Service property %s: %w", name, ErrBinding)
		}
	}
	if !systemdproperty.Matches(service, "UMask", "u", uint32(0077)) || !systemdproperty.Matches(service, "RemainAfterExit", "b", false) || !systemdproperty.Matches(service, "SupplementaryGroups", "as", []string{}) || !systemdproperty.Matches(service, "ReadWritePaths", "as", []string{r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot, r.IntroductionRoot}) || !systemdproperty.Matches(service, "Environment", "as", []string{"GOMAXPROCS=1", "GOMEMLIMIT=536870912"}) {
		return ErrBinding
	}
	for _, name := range []string{"ExecCondition", "ExecStartPre", "ExecStartPost", "ExecReload", "ExecStop", "ExecStopPost"} {
		if !systemdproperty.Matches(service, name, "a(sasbttttuii)", []any{}) {
			return ErrBinding
		}
	}
	families := service["RestrictAddressFamilies"]
	var parts []json.RawMessage
	var allow bool
	var names []string
	if families.Type != "(bas)" || systemdproperty.Decode(families.Data, &parts) != nil || len(parts) != 2 || systemdproperty.Decode(parts[0], &allow) != nil || !allow || systemdproperty.Decode(parts[1], &names) != nil {
		return ErrBinding
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"AF_INET", "AF_INET6", "AF_UNIX"}) {
		return ErrBinding
	}
	exec := service["ExecStartEx"]
	var entries [][]json.RawMessage
	if exec.Type != "a(sasasttttuii)" || systemdproperty.Decode(exec.Data, &entries) != nil || len(entries) != 1 || len(entries[0]) != 10 {
		return ErrBinding
	}
	program := filepath.Join(r.InstallationRoot, "generations", generation, programName)
	for index, wanted := range map[int]any{0: program, 1: []string{program, "introduction", "start-installed", filepath.Join(r.InstallationRoot, "binding.json")}, 2: []string{"no-env-expand"}} {
		p := systemdproperty.Properties{"value": {Type: "v", Data: entries[0][index]}}
		if !systemdproperty.Matches(p, "value", "v", wanted) {
			return ErrBinding
		}
	}
	return nil
}

func verifyIntroductionStopped(unit, service systemdproperty.Properties, r Request, generation string) error {
	if err := verifyIntroductionConfiguration(unit, service, r, generation); err != nil {
		return err
	}
	if !systemdproperty.Matches(unit, "ActiveState", "s", "inactive") || !systemdproperty.Matches(unit, "SubState", "s", "dead") ||
		!systemdproperty.Matches(unit, "InvocationID", "ay", []any{}) || !systemdproperty.Matches(service, "ControlGroup", "s", "") ||
		!systemdproperty.Matches(service, "MainPID", "u", uint32(0)) || !systemdproperty.Matches(service, "ExecMainPID", "u", uint32(0)) {
		return ErrBinding
	}
	status, err := introductionExecutionStatus(service)
	if err != nil || status != (introductionExecution{}) {
		return ErrBinding
	}
	return nil
}

type introductionExecution struct {
	startedRealtime, startedMonotonic, exitedRealtime, exitedMonotonic uint64
	pid                                                                uint32
	code, status                                                       int32
}

func introductionExecutionStatus(service systemdproperty.Properties) (result introductionExecution, err error) {
	value := service["ExecStartEx"]
	var entries [][]json.RawMessage
	if value.Type != "a(sasasttttuii)" || systemdproperty.Decode(value.Data, &entries) != nil || len(entries) != 1 || len(entries[0]) != 10 {
		return result, ErrBinding
	}
	// ExecStartEx initially carries the command's fork clock. systemd replaces
	// it with the main process clock when reaping that same process. Retain the
	// separate ExecMain clocks, which identify the process throughout its life.
	var command introductionExecution
	for index, target := range []any{&command.startedRealtime, &command.startedMonotonic, &command.exitedRealtime, &command.exitedMonotonic, &command.pid, &command.code, &command.status} {
		if systemdproperty.Decode(entries[0][index+3], target) != nil {
			return introductionExecution{}, ErrBinding
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
		if !present || value.Type != field.signature || systemdproperty.Decode(value.Data, field.target) != nil {
			return introductionExecution{}, ErrBinding
		}
	}
	if command.pid != result.pid || command.code != result.code || command.status != result.status ||
		command.exitedRealtime != result.exitedRealtime || command.exitedMonotonic != result.exitedMonotonic ||
		(command.startedRealtime == 0) != (result.startedRealtime == 0) || (command.startedMonotonic == 0) != (result.startedMonotonic == 0) ||
		command.startedRealtime > result.startedRealtime || command.startedMonotonic > result.startedMonotonic {
		return introductionExecution{}, ErrBinding
	}
	if result.code != 0 && (command.startedRealtime != result.startedRealtime || command.startedMonotonic != result.startedMonotonic) {
		return introductionExecution{}, ErrBinding
	}
	return result, nil
}

func verifyIntroductionRunning(unit, service systemdproperty.Properties, r Request, generation string, pid uint32, invocation [16]byte) error {
	if pid == 0 || invocation == [16]byte{} {
		return ErrBinding
	}
	if err := verifyIntroductionConfiguration(unit, service, r, generation); err != nil {
		return err
	}
	if !systemdproperty.Matches(unit, "ActiveState", "s", "active") || !systemdproperty.Matches(unit, "SubState", "s", "running") ||
		!systemdproperty.Matches(unit, "InvocationID", "ay", invocation) || !systemdproperty.Matches(unit, "TriggeredBy", "as", []string{}) ||
		!systemdproperty.Matches(service, "MainPID", "u", pid) || !systemdproperty.Matches(service, "ExecMainPID", "u", pid) ||
		!systemdproperty.Matches(service, "ControlPID", "u", uint32(0)) || !systemdproperty.Matches(service, "NRestarts", "u", uint32(0)) ||
		!systemdproperty.Matches(service, "DynamicUser", "b", false) || !systemdproperty.Matches(service, "ControlGroup", "s", "/system.slice/ardents-introduction.service") ||
		!systemdproperty.Matches(service, "StandardInput", "s", "null") {
		return ErrBinding
	}
	status, err := introductionExecutionStatus(service)
	if err != nil || status.startedRealtime == 0 || status.startedMonotonic == 0 || status.exitedRealtime != 0 || status.exitedMonotonic != 0 || status.pid != pid || status.code != 0 || status.status != 0 {
		return ErrBinding
	}
	return nil
}
