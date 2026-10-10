package endpoint

import (
	"context"
	"errors"
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
	command := exec.CommandContext(bounded, "/usr/sbin/useradd", "--system", "--user-group", "--no-create-home", "--home-dir", "/nonexistent", "--shell", "/usr/sbin/nologin", "ardents-endpoint")
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	command.WaitDelay = 5 * time.Second
	var output, diagnostic boundedAccountOutput
	command.Stdout, command.Stderr = &output, &diagnostic
	return errors.Join(runOriginalAccountCommand(command), bounded.Err(), ctx.Err())
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
