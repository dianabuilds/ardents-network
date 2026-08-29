package contributor

import (
	"context"
	"errors"
	"os"
	"time"
)

// Control executes one fixed lifecycle action. Confirmation is used only by
// the destructive Remove action.
func (profile *Profile) Control(ctx context.Context, action Action, confirmation string) (Report, error) {
	if profile == nil || ctx == nil {
		return Report{}, errors.New("contributor profile is unavailable")
	}
	if action != Remove && confirmation != "" {
		return Report{}, errors.New("contributor confirmation is not accepted for this action")
	}
	record, err := readInstallation(profile.paths.record)
	if err != nil {
		return Report{}, err
	}
	if err := profile.clearCommittedInstallationMarker(record); err != nil {
		return Report{}, err
	}
	if err := profile.recoverInterruptedUpdate(ctx, record); err != nil {
		return Report{}, err
	}
	switch action {
	case Diagnose:
		return profile.report(ctx)
	case Restart:
		if _, err := profile.report(ctx); err != nil {
			return Report{}, err
		}
		_ = os.Remove(profile.paths.lifecycle)
		state, err := profile.supervisor.Do(ctx, SupervisorRestart)
		if err != nil {
			return Report{}, err
		}
		if !state.Active {
			return Report{}, errors.New("restarted Contributor did not become active")
		}
		if _, err := profile.awaitLifecycle(ctx, profile.paths.lifecycle, "READY", 15*time.Second); err != nil {
			return Report{}, err
		}
		return profile.report(ctx)
	case Drain:
		return profile.stop(ctx, false)
	case Withdraw:
		return profile.stop(ctx, true)
	case Remove:
		current, err := profile.report(ctx)
		if err != nil {
			return Report{}, err
		}
		if confirmation != current.DeploymentID {
			return Report{}, errors.New("contributor removal confirmation does not match the installed deployment")
		}
		if current.Active || current.Enabled || current.LifecycleState != "WITHDRAWN" {
			return Report{}, errors.New("contributor must be withdrawn before removal")
		}
		if err := removeInstallation(profile.paths); err != nil {
			return Report{}, err
		}
		if _, err := profile.supervisor.Do(ctx, SupervisorReload); err != nil {
			return Report{}, err
		}
		current.LifecycleState = "REMOVED"
		current.Active, current.Enabled = false, false
		return current, nil
	default:
		return Report{}, errors.New("contributor lifecycle action is not implemented")
	}
}

func (profile *Profile) stop(ctx context.Context, disable bool) (Report, error) {
	if _, err := profile.report(ctx); err != nil {
		return Report{}, err
	}
	if _, err := profile.supervisor.Do(ctx, SupervisorStop); err != nil {
		return Report{}, err
	}
	if _, err := profile.awaitLifecycle(ctx, profile.paths.lifecycle, "WITHDRAWN", 15*time.Second); err != nil {
		return Report{}, err
	}
	if disable {
		if _, err := profile.supervisor.Do(ctx, SupervisorDisable); err != nil {
			return Report{}, err
		}
	}
	return profile.report(ctx)
}
