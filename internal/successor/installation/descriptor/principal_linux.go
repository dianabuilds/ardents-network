package descriptor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/nodeprincipal"
)

// Host databases remain host-owned. Each observation independently reads the
// current fixed principal; unrelated completed host updates are not frozen to
// the initial helper's postimages. Root additionally checks private lock rows
// and actual NSS results before its effect. Runtime verifies its own kernel
// credentials separately and spawns no NSS helper in the Node cgroup.
func observeNodePrincipal(ctx context.Context, uid, gid uint32) (err error) {
	if ctx == nil || uid < 100 || uid > 999 || gid != uid {
		return ErrInput
	}
	files := map[string]*accountFile{}
	defer func() {
		var physical error
		for _, file := range files {
			if closed := file.file.Close(); closed != nil {
				physical = errors.Join(physical, closed)
			}
			clear(file.raw)
		}
		if physical != nil {
			err = errors.Join(err, physical)
		}
		if caller := ctx.Err(); caller != nil && caller != err {
			if err == nil {
				err = caller
			} else {
				err = errors.Join(err, caller)
			}
		}
	}()
	paths := []string{"/etc/passwd", "/etc/group", "/etc/nsswitch.conf"}
	if os.Geteuid() == 0 {
		paths = append(paths, "/etc/shadow", "/etc/gshadow", "/usr/bin/getent")
	}
	for _, path := range paths {
		if err = ctx.Err(); err != nil {
			return err
		}
		file, e := readNodeAccountFile(path)
		if e != nil {
			return e
		}
		files[path] = file
	}
	rows, err := nodeprincipal.ValidatePublic(files["/etc/passwd"].raw, files["/etc/group"].raw, files["/etc/nsswitch.conf"].raw, uid, gid)
	if err != nil {
		return ErrBinding
	}
	if os.Geteuid() == 0 {
		for _, path := range []string{"/etc/shadow", "/etc/gshadow"} {
			row, e := nodeprincipal.CurrentRow(files[path].raw)
			if e != nil || nodeprincipal.ValidateRow(path, row, uid, gid) != nil {
				if e != nil {
					e = ErrBinding
				}
				return errors.Join(ErrBinding, e)
			}
		}
		// Each database still independently resolves both the fixed name and
		// numeric identity. One getent process accepts both keys and returns
		// both rows in order; require two exact rows, not one deduplicated or
		// partially successful result. This keeps every fresh NSS observation
		// while avoiding a second namespace/helper lifetime for that database.
		for _, query := range []struct {
			database string
			id       uint32
		}{
			{"passwd", uid}, {"group", gid},
		} {
			if err = ctx.Err(); err != nil {
				return err
			}
			if err = checkNodeAccountFile(files["/usr/bin/getent"]); err != nil {
				return err
			}
			bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
			command := exec.CommandContext(bounded, "/usr/bin/getent", query.database, "ardents-node", strconv.FormatUint(uint64(query.id), 10))
			command.Env = []string{"LANG=C", "PATH=/usr/bin:/bin"}
			var output nodeAccountDiagnostic
			command.Stdout = &output
			runtime.LockOSThread()
			// The finite helper is PID1 of its own namespace. Kernel teardown
			// joins descendants before reap, including after caller death/kill.
			command.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL, Cloneflags: syscall.CLONE_NEWPID}
			runErr := command.Run()
			runtime.UnlockOSThread()
			contextErr := bounded.Err()
			cancel()
			if runErr != nil || contextErr != nil || output.body.String() != strings.Repeat(rows[query.database]+"\n", 2) {
				return errors.Join(ErrBinding, runErr, contextErr)
			}
		}
	}
	for _, file := range files {
		if err = checkNodeAccountFile(file); err != nil {
			return err
		}
	}
	return ctx.Err()
}
