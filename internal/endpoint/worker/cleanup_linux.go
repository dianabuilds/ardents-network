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
	instance Instance
	events   *os.File
	once     sync.Once
	err      error
}

func NewCleanup(instance Instance) (*Cleanup, error) {
	if instance.PID == 0 || instance.UID == 0 || instance.Invocation == [16]byte{} {
		return nil, errors.New("text worker cleanup identity is unavailable")
	}
	events, err := PinCgroup(instance)
	if err != nil {
		return nil, err
	}
	return &Cleanup{instance: instance, events: events}, nil
}

func (owner *Cleanup) Close() error {
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

func (owner *Cleanup) join() error {
	// Cleanup must survive cancellation of the job's context. The manager's
	// selected two-second stop is inside this independent finite join bound.
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	removed, _, initialErr := ReadCgroup(owner.events)
	if removed && initialErr == nil {
		return nil
	}
	unit, service, err := ReadProperties(ctx, owner.instance.Name, owner.instance.Role)
	if err != nil {
		// A crashing unit can disappear between the first observation and the
		// manager query. Only the original kernel object's removal resolves it.
		if gone, _, observedErr := ReadCgroup(owner.events); gone && observedErr == nil && initialErr == nil {
			return nil
		}
		return errors.New("text worker cleanup invocation is unavailable")
	}
	if !sameCleanupInstance(owner.instance, unit, service) {
		return errors.New("text worker cleanup invocation changed")
	}
	if err := stopInstance(ctx, owner.instance.Name, owner.instance.Role); err != nil {
		return errors.Join(initialErr, err)
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		gone, populated, err := ReadCgroup(owner.events)
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

func sameCleanupInstance(instance Instance, unit, service Properties) bool {
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
	if ctx == nil || ctx.Err() != nil || !ValidUnit(name, role) {
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
