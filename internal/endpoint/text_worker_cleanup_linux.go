//go:build linux

package endpoint

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

// textWorkerCleanup owns only one observed invocation and its pinned cgroup.
// Possession does not establish stop permission or qualify a launch. The launch
// owner must establish installed cleanup authority before exposing any Grant.
// Close retains its first failure; a retry cannot make a failed cleanup green.
// Its caller must retire the job and close the worker attachment before Close.
// Private roots are shared immutable artifacts, never writable job state.
// No system permission is installed or elevated by this owner.
type textWorkerCleanup struct {
	instance       textWorkerInstance
	events         *os.File
	readCgroup     func(*os.File) (bool, bool, error)
	readProperties func(context.Context, string, string) (textManagerProperties, textManagerProperties, error)
	stop           func(context.Context, string, string) error
	joinTimeout    time.Duration
	once           sync.Once
	err            error
}

func newTextWorkerCleanup(instance textWorkerInstance) (*textWorkerCleanup, error) {
	if instance.pid == 0 || instance.uid == 0 || instance.invocation == [16]byte{} {
		return nil, errors.New("text worker cleanup identity is unavailable")
	}
	events, err := pinTextWorkerCgroup(instance)
	if err != nil {
		return nil, err
	}
	return &textWorkerCleanup{instance: instance, events: events}, nil
}

func (owner *textWorkerCleanup) Close() error {
	if owner == nil {
		return errors.New("text worker cleanup owner is absent")
	}
	owner.once.Do(func() {
		owner.err = owner.join()
		if owner.events != nil {
			owner.err = errors.Join(owner.err, owner.events.Close())
		}
	})
	return owner.err
}

func (owner *textWorkerCleanup) join() error {
	// Cleanup must survive cancellation of the job's context. The manager's
	// selected two-second stop is inside this independent finite join bound.
	ctx, cancel := context.WithTimeout(context.Background(), owner.timeout())
	defer cancel()
	removed, _, initialErr := owner.observeCgroup()
	if removed && initialErr == nil {
		return nil
	}
	unit, service, err := owner.observeProperties(ctx)
	if err != nil {
		// A crashing unit can disappear between the first observation and the
		// manager query. Only retirement of the original pinned cgroup resolves
		// that race; a replacement manager invocation is never stopped.
		if owner.waitForPinnedCgroupRetirement(ctx, initialErr) {
			return nil
		}
		return firstTextWorkerCleanupFailure(initialErr, "text worker cleanup invocation is unavailable")
	}
	if !sameTextWorkerCleanupInstance(owner.instance, unit, service) {
		if owner.waitForPinnedCgroupRetirement(ctx, initialErr) {
			return nil
		}
		return firstTextWorkerCleanupFailure(initialErr, "text worker cleanup invocation changed")
	}
	if err := owner.stopInstance(ctx); err != nil {
		return errors.Join(initialErr, err)
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		gone, populated, err := owner.observeCgroup()
		if err != nil {
			return errors.Join(initialErr, err)
		}
		if (gone || !populated) && ctx.Err() == nil {
			return initialErr
		}
		select {
		case <-ctx.Done():
			return errors.Join(initialErr, errors.New("text worker cgroup cleanup did not complete"))
		case <-ticker.C:
		}
	}
}

func firstTextWorkerCleanupFailure(initialErr error, fallback string) error {
	if initialErr != nil {
		return initialErr
	}
	return errors.New(fallback)
}

func (owner *textWorkerCleanup) timeout() time.Duration {
	if owner != nil && owner.joinTimeout > 0 {
		return owner.joinTimeout
	}
	return 8 * time.Second
}

func (owner *textWorkerCleanup) observeCgroup() (bool, bool, error) {
	if owner != nil && owner.readCgroup != nil {
		return owner.readCgroup(owner.events)
	}
	return readTextWorkerCgroup(owner.events)
}

func (owner *textWorkerCleanup) observeProperties(ctx context.Context) (textManagerProperties, textManagerProperties, error) {
	if owner != nil && owner.readProperties != nil {
		return owner.readProperties(ctx, owner.instance.name, owner.instance.role)
	}
	return readTextWorkerProperties(ctx, owner.instance.name, owner.instance.role)
}

func (owner *textWorkerCleanup) stopInstance(ctx context.Context) error {
	if owner != nil && owner.stop != nil {
		return owner.stop(ctx, owner.instance.name, owner.instance.role)
	}
	return stopTextWorkerInstance(ctx, owner.instance.name, owner.instance.role)
}

func (owner *textWorkerCleanup) waitForPinnedCgroupRetirement(ctx context.Context, initialErr error) bool {
	if initialErr != nil {
		return false
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		removed, populated, err := owner.observeCgroup()
		if err != nil {
			return false
		}
		if (removed || !populated) && ctx.Err() == nil {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
}

func sameTextWorkerCleanupInstance(instance textWorkerInstance, unit, service textManagerProperties) bool {
	var invocation [16]byte
	if !textWorkerCgroupPath(instance.cgroup, instance.name, instance.role) || instance.pid == 0 ||
		!unit.exact("Id", "s", instance.name) || !unit.exact("LoadState", "s", "loaded") ||
		!decodeTextWorkerInvocation(unit["InvocationID"], &invocation) || invocation != instance.invocation {
		return false
	}
	pid, group := service["MainPID"], service["ControlGroup"]
	currentPID, pidErr := strconv.ParseUint(strings.TrimSpace(string(pid.Data)), 10, 32)
	var currentGroup string
	if pid.Type != "u" || group.Type != "s" || pidErr != nil || strings.TrimSpace(string(group.Data)) == "null" ||
		json.Unmarshal(group.Data, &currentGroup) != nil ||
		(currentPID != 0 && currentPID != uint64(instance.pid)) || (currentGroup != "" && currentGroup != instance.cgroup) {
		return false
	}
	return service.exact("Restart", "s", "no") && service.exact("KillMode", "s", "control-group") &&
		service.exact("Delegate", "b", false) && service.exact("TimeoutStopUSec", "t", uint64(2_000_000)) &&
		service.exact("ExecStop", "a(sasbttttuii)", []any{}) && service.exact("ExecStopPost", "a(sasbttttuii)", []any{})
}

func stopTextWorkerInstance(ctx context.Context, name, role string) error {
	if ctx == nil || ctx.Err() != nil || !textWorkerUnit(name, role) {
		return errors.New("text worker stop identity is invalid")
	}
	// systemctl waits for the manager's stop job by default. The fixed command
	// inherits the unprivileged Endpoint account; it cannot request a password,
	// accept a shell fragment, choose another operation or install authority.
	command := exec.CommandContext(ctx, "/usr/bin/systemctl", "--system", "--no-ask-password", "--no-pager", "stop", name)
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	command.Stdout, command.Stderr = io.Discard, io.Discard
	if command.Run() != nil || ctx.Err() != nil {
		return errors.New("text worker system manager stop failed")
	}
	return nil
}
