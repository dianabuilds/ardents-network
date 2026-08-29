//go:build linux

package main

import (
	"context"
	"errors"
	"os/exec"

	"github.com/dianabuilds/ardents-network/internal/contributor"
)

const contributorUnit = "ardents-rendezvous-contributor.service"

type systemdSupervisor struct{}

func newSystemdSupervisor() (contributor.Supervisor, error) { return systemdSupervisor{}, nil }
func contributorHostRoot() string                           { return "/" }

func (systemdSupervisor) Do(ctx context.Context, action contributor.SupervisorAction) (contributor.SupervisorState, error) {
	var arguments []string
	switch action {
	case contributor.SupervisorReload:
		arguments = []string{"daemon-reload"}
	case contributor.SupervisorEnable:
		arguments = []string{"enable", contributorUnit}
	case contributor.SupervisorStart:
		arguments = []string{"start", contributorUnit}
	case contributor.SupervisorRestart:
		arguments = []string{"restart", contributorUnit}
	case contributor.SupervisorStop:
		arguments = []string{"stop", contributorUnit}
	case contributor.SupervisorDisable:
		arguments = []string{"disable", contributorUnit}
	case contributor.SupervisorStatus:
		return systemdState(ctx)
	default:
		return contributor.SupervisorState{}, errors.New("Contributor supervisor action is invalid")
	}
	if err := exec.CommandContext(ctx, "systemctl", arguments...).Run(); err != nil {
		return contributor.SupervisorState{}, err
	}
	return systemdState(ctx)
}

func systemdState(ctx context.Context) (contributor.SupervisorState, error) {
	active, err := systemdBoolean(ctx, "is-active", contributorUnit)
	if err != nil {
		return contributor.SupervisorState{}, err
	}
	enabled, err := systemdBoolean(ctx, "is-enabled", contributorUnit)
	return contributor.SupervisorState{Active: active, Enabled: enabled}, err
}

func systemdBoolean(ctx context.Context, operation, unit string) (bool, error) {
	err := exec.CommandContext(ctx, "systemctl", "--quiet", operation, unit).Run()
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() >= 1 && exit.ExitCode() <= 4 {
		return false, nil
	}
	return false, err
}
