//go:build linux

package worker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Cleanup owns only one observed invocation and its pinned cgroup.
// Possession does not establish stop permission or qualify a launch. The launch
// owner must establish installed cleanup authority before exposing any Grant.
// Close retains its first failure; a retry cannot make a failed cleanup green.
// Its caller must retire the job and close the worker attachment before Close.
// Private roots are shared immutable artifacts, never writable job state.
// No system permission is installed or elevated by this owner.
type Cleanup struct {
	instance     Instance
	events       *os.File
	once         sync.Once
	err          error
	failureOnce  sync.Once
	firstFailure func(error)
}

// NewCleanup pins the original scope and binds its first physical failure to
// the concrete launch owner. Reporting precedes further waiting; it grants no
// cleanup completion and must not wait for Close or start another worker.
func NewCleanup(instance Instance, firstFailure func(error)) (*Cleanup, error) {
	if instance.PID == 0 || instance.UID == 0 || instance.Invocation == [16]byte{} || firstFailure == nil {
		return nil, errors.New("text worker cleanup identity is unavailable")
	}
	events, err := pinCgroup(instance)
	if err != nil {
		return nil, err
	}
	return &Cleanup{instance: instance, events: events, firstFailure: firstFailure}, nil
}

func (owner *Cleanup) reportFailure(err error) {
	if err != nil {
		owner.failureOnce.Do(func() {
			if owner.firstFailure != nil {
				owner.firstFailure(err)
			}
		})
	}
}

func (owner *Cleanup) Close() error {
	if owner == nil {
		return errors.New("text worker cleanup owner is absent")
	}
	owner.once.Do(func() {
		owner.err = owner.join()
		owner.reportFailure(owner.err)
		if owner.events != nil {
			closeErr := owner.events.Close()
			owner.reportFailure(closeErr)
			owner.err = errors.Join(owner.err, closeErr)
		}
	})
	return owner.err
}

func (owner *Cleanup) join() error {
	// Cleanup must survive cancellation of the job's context. The manager's
	// selected two-second stop is inside this independent finite join bound.
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return owner.joinObserved(ctx, readCgroup, readProperties, stopInstance)
}

func (owner *Cleanup) joinObserved(
	ctx context.Context,
	read func(*os.File) (bool, bool, error),
	properties func(context.Context, string, string) (properties, properties, error),
	stop func(context.Context, string, string) error,
) error {
	removed, _, initialErr := read(owner.events)
	owner.reportFailure(initialErr)
	if removed && initialErr == nil {
		return nil
	}
	unit, service, err := properties(ctx, owner.instance.Name, owner.instance.Role)
	if err != nil {
		// The manager may disappear while the original cgroup is still live.
		// Keep observing its pinned descriptor without stopping another unit.
		return owner.waitOriginalCgroup(ctx, read, initialErr,
			errors.New("text worker cleanup invocation is unavailable"), true)
	}
	if !sameCleanupInstance(owner.instance, unit, service) {
		identityErr := errors.New("text worker cleanup invocation changed")
		owner.reportFailure(identityErr)
		return owner.waitOriginalCgroup(ctx, read, initialErr,
			identityErr, false)
	}
	// A failed manager Stop is not an original-descendant join. Preserve that
	// failure while continuing to observe only the pinned kernel object.
	stopErr := stop(ctx, owner.instance.Name, owner.instance.Role)
	owner.reportFailure(stopErr)
	return owner.waitOriginalCgroup(ctx, read, errors.Join(initialErr, stopErr), nil, false)
}

// waitOriginalCgroup observes only the cgroup pinned before worker readiness.
// A manager identity mismatch cannot authorize a stop of the replacement.
func (owner *Cleanup) waitOriginalCgroup(
	ctx context.Context,
	read func(*os.File) (bool, bool, error),
	initialErr, identityErr error,
	acceptRemoval bool,
) error {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		gone, populated, err := read(owner.events)
		if err != nil {
			result := errors.Join(initialErr, identityErr, err)
			owner.reportFailure(result)
			return result
		}
		if (gone || !populated) && ctx.Err() == nil {
			if gone && acceptRemoval && initialErr == nil {
				return nil
			}
			result := errors.Join(initialErr, identityErr)
			owner.reportFailure(result)
			return result
		}
		select {
		case <-ctx.Done():
			result := errors.Join(initialErr, identityErr, errors.New("text worker cgroup cleanup did not complete"))
			owner.reportFailure(result)
			return result
		case <-ticker.C:
		}
	}
}

func sameCleanupInstance(instance Instance, unit, service properties) bool {
	var invocation [16]byte
	if !cgroupPath(instance.Cgroup, instance.Name, instance.Role) || instance.PID == 0 ||
		!unit.exact("Id", "s", instance.Name) || !unit.exact("LoadState", "s", "loaded") ||
		!decodeInvocation(unit["InvocationID"], &invocation) || invocation != instance.Invocation {
		return false
	}
	pid, group := service["MainPID"], service["ControlGroup"]
	currentPID, pidErr := strconv.ParseUint(strings.TrimSpace(string(pid.Data)), 10, 32)
	var currentGroup string
	if pid.Type != "u" || group.Type != "s" || pidErr != nil || strings.TrimSpace(string(group.Data)) == "null" ||
		json.Unmarshal(group.Data, &currentGroup) != nil ||
		(currentPID != 0 && currentPID != uint64(instance.PID)) || (currentGroup != "" && currentGroup != instance.Cgroup) {
		return false
	}
	return service.exact("Restart", "s", "no") && service.exact("KillMode", "s", "control-group") &&
		service.exact("Delegate", "b", false) && service.exact("TimeoutStopUSec", "t", uint64(2_000_000)) &&
		service.exact("ExecStop", "a(sasbttttuii)", []any{}) && service.exact("ExecStopPost", "a(sasbttttuii)", []any{})
}

func stopInstance(ctx context.Context, name, role string) error {
	if ctx == nil || ctx.Err() != nil || !validUnit(name, role) {
		return errors.New("text worker stop identity is invalid")
	}
	// systemctl waits for the manager's stop job by default. The fixed command
	// inherits the unprivileged Endpoint account; it cannot request a password,
	// accept a shell fragment, choose another operation or install authority.
	command := exec.CommandContext(ctx, "/usr/bin/systemctl", "--system", "--no-ask-password", "--no-pager", "stop", name)
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	command.Stdout, command.Stderr = io.Discard, io.Discard
	if runOriginalCommand(command) != nil || ctx.Err() != nil {
		return errors.New("text worker system manager stop failed")
	}
	return nil
}
