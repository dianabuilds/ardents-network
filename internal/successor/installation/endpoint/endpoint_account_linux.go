package endpoint

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Native account mechanisms are shared by initial preparation and independent
// installed inspection. Callers retain effect admission and exact expected IDs;
// these functions grant no journal, lease, recovery or startup authority.
func createEndpointAccount(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// The dedicated account uses a matching uid==gid private group, the
	// platform-led account contract shared with infrastructure Source and the
	// platform pre-start account check. `useradd --user-group` would allocate an
	// independent gid (uid!=gid) that the platform check refuses.
	id, err := freeEndpointAccountID()
	if err != nil {
		return err
	}
	numeric := strconv.FormatUint(uint64(id), 10)
	group := exec.CommandContext(bounded, "/usr/sbin/groupadd", "-g", numeric, "ardents-endpoint")
	group.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	group.WaitDelay = 5 * time.Second
	var groupOutput, groupDiagnostic boundedAccountOutput
	group.Stdout, group.Stderr = &groupOutput, &groupDiagnostic
	if err := runOriginalAccountCommand(group); err != nil {
		return errors.Join(err, bounded.Err(), ctx.Err())
	}
	command := exec.CommandContext(bounded, "/usr/sbin/useradd", "--system", "-u", numeric, "-g", numeric, "--no-create-home", "--home-dir", "/nonexistent", "--shell", "/usr/sbin/nologin", "ardents-endpoint")
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	command.WaitDelay = 5 * time.Second
	var output, diagnostic boundedAccountOutput
	command.Stdout, command.Stderr = &output, &diagnostic
	return errors.Join(runOriginalAccountCommand(command), bounded.Err(), ctx.Err())
}

// freeEndpointAccountID selects one unused 100-999 identity for both the user
// and its private group. It refuses an already-present endpoint account; it
// creates nothing and grants no authority.
func freeEndpointAccountID() (uint32, error) {
	used := map[uint32]bool{}
	for _, path := range []string{"/etc/passwd", "/etc/group"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			return 0, errors.Join(ErrNativeUnavailable, err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			fields := strings.Split(line, ":")
			if len(fields) < 3 {
				continue
			}
			if fields[0] == "ardents-endpoint" {
				return 0, ErrNativeUnavailable
			}
			if n, err := strconv.ParseUint(fields[2], 10, 32); err == nil {
				used[uint32(n)] = true
			}
		}
	}
	for id := uint32(999); id >= 100; id-- {
		if !used[id] {
			return id, nil
		}
	}
	return 0, ErrNativeUnavailable
}

func runOriginalAccountCommand(command *exec.Cmd) error {
	// Linux binds the parent-death signal to the creating OS thread. Retain
	// that thread until Run joins the exact account helper, including failure.
	// Fatal caller death cannot leave the helper to make later account changes;
	// this grants no recovery or adoption of an interrupted account creation.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	command.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	return command.Run()
}

func observeEndpointAccount() (uint32, uint32, error) {
	account, err := user.Lookup("ardents-endpoint")
	if err != nil {
		return 0, 0, err
	}
	uid, uidErr := strconv.ParseUint(account.Uid, 10, 32)
	gid, gidErr := strconv.ParseUint(account.Gid, 10, 32)
	group, groupErr := user.LookupGroup("ardents-endpoint")
	groups, groupsErr := account.GroupIds()
	if uidErr != nil || gidErr != nil || uid == 0 || gid == 0 || uid == 1<<32-1 || gid == 1<<32-1 ||
		groupErr != nil || group.Gid != account.Gid || groupsErr != nil || len(groups) != 1 || groups[0] != account.Gid {
		return 0, 0, ErrNativeUnavailable
	}
	return uint32(uid), uint32(gid), nil
}

type boundedAccountOutput struct{ body strings.Builder }

func (output *boundedAccountOutput) Write(body []byte) (int, error) {
	if output.body.Len()+len(body) > 64<<10 {
		return 0, ErrNativeUnavailable
	}
	return output.body.Write(body)
}
