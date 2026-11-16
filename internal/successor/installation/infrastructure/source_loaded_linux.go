package infrastructure

import (
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/successor/deployment"
	releaseartifact "github.com/dianabuilds/ardents-network/internal/successor/release/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/systemdproperty"
)

// Source configuration policy is shared by its separate stopped and running
// comparisons. Neither fabricates another phase's native facts.
func verifySourceConfiguration(unit, service systemdproperty.Properties, r SourceRequest, generation string) error {
	section, name, ok := deployment.CheckInfrastructureService(unit, service, "source", r.InstallationRoot, generation, releaseartifact.ProgramName, []string{r.StateRoot, r.ParticipationRoot})
	if ok {
		return nil
	}
	if section != "" {
		return fmt.Errorf("source %s property %s: %w", section, name, ErrBinding)
	}
	return ErrBinding
}

func verifySourceStopped(unit, service systemdproperty.Properties, r SourceRequest, generation string) error {
	if err := verifySourceConfiguration(unit, service, r, generation); err != nil {
		return err
	}
	if !systemdproperty.Matches(unit, "ActiveState", "s", "inactive") || !systemdproperty.Matches(unit, "SubState", "s", "dead") ||
		!systemdproperty.Matches(unit, "InvocationID", "ay", []any{}) || !systemdproperty.Matches(service, "ControlGroup", "s", "") ||
		!systemdproperty.Matches(service, "MainPID", "u", uint32(0)) || !systemdproperty.Matches(service, "ExecMainPID", "u", uint32(0)) {
		return ErrBinding
	}
	status, err := sourceExecutionStatus(service)
	if err != nil || status != (sourceExecution{}) {
		return ErrBinding
	}
	return nil
}

type sourceExecution struct {
	startedRealtime, startedMonotonic, exitedRealtime, exitedMonotonic uint64
	pid                                                                uint32
	code, status                                                       int32
}

func sourceExecutionStatus(service systemdproperty.Properties) (sourceExecution, error) {
	observed, ok := systemdproperty.ReadExecution(service)
	if !ok {
		return sourceExecution{}, ErrBinding
	}
	return sourceExecution{observed.StartedRealtime, observed.StartedMonotonic, observed.ExitedRealtime, observed.ExitedMonotonic, observed.PID, observed.Code, observed.Status}, nil
}

func sourceInvocation(unit, service systemdproperty.Properties) (uint32, [16]byte, error) {
	pid, id, ok := systemdproperty.ReadInvocation(unit, service)
	if !ok {
		return 0, [16]byte{}, ErrBinding
	}
	return pid, id, nil
}

func verifySourceRunning(unit, service systemdproperty.Properties, r SourceRequest, generation string, pid uint32, invocation [16]byte) error {
	if pid == 0 || invocation == [16]byte{} {
		return ErrBinding
	}
	if err := verifySourceConfiguration(unit, service, r, generation); err != nil {
		return err
	}
	if !systemdproperty.Matches(unit, "ActiveState", "s", "active") || !systemdproperty.Matches(unit, "SubState", "s", "running") ||
		!systemdproperty.Matches(unit, "InvocationID", "ay", invocation) || !systemdproperty.Matches(unit, "TriggeredBy", "as", []string{}) ||
		!systemdproperty.Matches(service, "MainPID", "u", pid) ||
		!systemdproperty.Matches(service, "ExecMainPID", "u", pid) || !systemdproperty.Matches(service, "ControlPID", "u", uint32(0)) ||
		!systemdproperty.Matches(service, "NRestarts", "u", uint32(0)) || !systemdproperty.Matches(service, "DynamicUser", "b", false) ||
		!systemdproperty.Matches(service, "ControlGroup", "s", "/system.slice/ardents-source.service") ||
		!systemdproperty.Matches(service, "StandardInput", "s", "null") {
		return ErrBinding
	}
	status, err := sourceExecutionStatus(service)
	if err != nil || status.startedRealtime == 0 || status.startedMonotonic == 0 ||
		status.exitedRealtime != 0 || status.exitedMonotonic != 0 || status.pid != pid || status.code != 0 || status.status != 0 {
		return ErrBinding
	}
	return nil
}

// Native terminal facts are checked only in conjunction with the original
// domain frame and kernel scope join. These copied properties alone never
// establish clean completion or permit a new invocation.
func verifySourceCompletedStop(unit, service systemdproperty.Properties, r SourceRequest, generation string, original *sourceStartHistory, execution sourceExecution) error {
	if original == nil || execution.pid != original.archive.PID || execution.startedRealtime != original.archive.StartedRealtime || execution.startedMonotonic != original.archive.StartedMonotonic || execution.exitedRealtime < execution.startedRealtime || execution.exitedMonotonic < execution.startedMonotonic || execution.code != 1 || execution.status != 0 {
		return ErrBinding
	}
	if err := verifySourceConfiguration(unit, service, r, generation); err != nil {
		return err
	}
	if !systemdproperty.Matches(unit, "ActiveState", "s", "inactive") || !systemdproperty.Matches(unit, "SubState", "s", "dead") ||
		!(systemdproperty.Matches(unit, "InvocationID", "ay", original.id) || systemdproperty.Matches(unit, "InvocationID", "ay", []any{})) ||
		!systemdproperty.Matches(unit, "TriggeredBy", "as", []string{}) || !systemdproperty.Matches(service, "MainPID", "u", uint32(0)) ||
		!systemdproperty.Matches(service, "ExecMainPID", "u", original.archive.PID) || !systemdproperty.Matches(service, "ControlPID", "u", uint32(0)) ||
		!systemdproperty.Matches(service, "NRestarts", "u", uint32(0)) || !systemdproperty.Matches(service, "DynamicUser", "b", false) ||
		!systemdproperty.Matches(service, "StandardInput", "s", "null") ||
		!(systemdproperty.Matches(service, "ControlGroup", "s", "") || systemdproperty.Matches(service, "ControlGroup", "s", "/system.slice/ardents-source.service")) {
		return ErrBinding
	}
	actual, err := sourceExecutionStatus(service)
	if err != nil || actual != execution {
		return ErrBinding
	}
	return nil
}
