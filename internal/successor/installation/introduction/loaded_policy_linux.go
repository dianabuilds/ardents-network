package introduction

import (
	"encoding/json"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/successor/deployment"
	"github.com/dianabuilds/ardents-network/internal/successor/systemdproperty"
)

// Introduction configuration policy is shared by its separate stopped and running
// comparisons. Neither fabricates another phase's native facts.
func verifyIntroductionConfiguration(unit, service systemdproperty.Properties, r Request, generation string) error {
	section, name, ok := deployment.CheckInfrastructureService(unit, service, "introduction", r.InstallationRoot, generation, programName, []string{r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot, r.IntroductionRoot})
	if ok {
		return nil
	}
	if section != "" {
		return fmt.Errorf("introduction %s property %s: %w", section, name, ErrBinding)
	}
	return ErrBinding
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
