package join

import (
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/successor/deployment"
	"github.com/dianabuilds/ardents-network/internal/successor/systemdproperty"
)

// JOIN configuration policy is shared by its separate stopped and running
// comparisons. Neither fabricates another phase's native facts.
func verifyJoinConfiguration(unit, service systemdproperty.Properties, r Request, generation string) error {
	section, name, ok := deployment.CheckInfrastructureService(unit, service, "join", r.InstallationRoot, generation, programName, []string{r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot})
	if ok {
		return nil
	}
	if section != "" {
		return fmt.Errorf("join %s property %s: %w", section, name, ErrBinding)
	}
	return ErrBinding
}

func verifyJoinStopped(unit, service systemdproperty.Properties, r Request, generation string) error {
	if err := verifyJoinConfiguration(unit, service, r, generation); err != nil {
		return err
	}
	if !systemdproperty.Matches(unit, "ActiveState", "s", "inactive") || !systemdproperty.Matches(unit, "SubState", "s", "dead") ||
		!systemdproperty.Matches(unit, "InvocationID", "ay", []any{}) || !systemdproperty.Matches(service, "ControlGroup", "s", "") ||
		!systemdproperty.Matches(service, "MainPID", "u", uint32(0)) || !systemdproperty.Matches(service, "ExecMainPID", "u", uint32(0)) {
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

func joinExecutionStatus(service systemdproperty.Properties) (joinExecution, error) {
	observed, ok := systemdproperty.ReadExecution(service)
	if !ok {
		return joinExecution{}, ErrBinding
	}
	return joinExecution{observed.StartedRealtime, observed.StartedMonotonic, observed.ExitedRealtime, observed.ExitedMonotonic, observed.PID, observed.Code, observed.Status}, nil
}

func verifyJoinRunning(unit, service systemdproperty.Properties, r Request, generation string, pid uint32, invocation [16]byte) error {
	if pid == 0 || invocation == [16]byte{} {
		return ErrBinding
	}
	if err := verifyJoinConfiguration(unit, service, r, generation); err != nil {
		return err
	}
	if !systemdproperty.Matches(unit, "ActiveState", "s", "active") || !systemdproperty.Matches(unit, "SubState", "s", "running") ||
		!systemdproperty.Matches(unit, "InvocationID", "ay", invocation) || !systemdproperty.Matches(unit, "TriggeredBy", "as", []string{}) ||
		!systemdproperty.Matches(service, "MainPID", "u", pid) || !systemdproperty.Matches(service, "ExecMainPID", "u", pid) ||
		!systemdproperty.Matches(service, "ControlPID", "u", uint32(0)) || !systemdproperty.Matches(service, "NRestarts", "u", uint32(0)) ||
		!systemdproperty.Matches(service, "DynamicUser", "b", false) || !systemdproperty.Matches(service, "ControlGroup", "s", "/system.slice/ardents-join.service") ||
		!systemdproperty.Matches(service, "StandardInput", "s", "null") {
		return ErrBinding
	}
	status, err := joinExecutionStatus(service)
	if err != nil || status.startedRealtime == 0 || status.startedMonotonic == 0 || status.exitedRealtime != 0 || status.exitedMonotonic != 0 || status.pid != pid || status.code != 0 || status.status != 0 {
		return ErrBinding
	}
	return nil
}
