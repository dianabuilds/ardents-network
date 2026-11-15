package nodeprincipal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type accountFile struct {
	path string
	file *os.File
	info os.FileInfo
	raw  []byte
}

func readNodeAccountFile(path string) (result *accountFile, err error) {
	// Input paths are a closed literal inventory, never caller-selected helpers.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	result = &accountFile{path: path, file: f}
	defer func() {
		if err != nil {
			err = errors.Join(err, f.Close())
			clear(result.raw)
			result = nil
		}
	}()
	result.info, err = f.Stat()
	if err != nil {
		return result, err
	}
	st, ok := result.info.Sys().(*syscall.Stat_t)
	if !ok || !result.info.Mode().IsRegular() || st.Uid != 0 || st.Nlink != 1 || result.info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || result.info.Mode().Perm()&0022 != 0 || result.info.Size() > 8<<20 {
		return result, ErrBinding
	}
	result.raw, err = io.ReadAll(io.LimitReader(f, 8<<20+1))
	if err != nil || int64(len(result.raw)) != result.info.Size() {
		return result, errors.Join(ErrBinding, err)
	}
	return result, checkNodeAccountFile(result)
}

func checkNodeAccountFile(f *accountFile) error {
	info, err := f.file.Stat()
	named, ne := os.Lstat(f.path)
	if err != nil || ne != nil || !sameInputFile(f.info, info) || !sameInputFile(f.info, named) {
		return errors.Join(ErrBinding, err, ne)
	}
	if _, err = f.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(f.file, int64(len(f.raw))+1))
	equal := bytes.Equal(raw, f.raw)
	clear(raw)
	if err != nil || !equal {
		return errors.Join(ErrBinding, err)
	}
	info, err = f.file.Stat()
	named, ne = os.Lstat(f.path)
	if err != nil || ne != nil || !sameInputFile(f.info, info) || !sameInputFile(f.info, named) {
		return errors.Join(ErrBinding, err, ne)
	}
	return nil
}

type nodeAccountDiagnostic struct{ body strings.Builder }

func (d *nodeAccountDiagnostic) Write(raw []byte) (int, error) {
	if d.body.Len()+len(raw) > 4096 {
		return 0, ErrBinding
	}
	return d.body.Write(raw)
}

func sameInputFile(before, after os.FileInfo) bool {
	if before == nil || after == nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() {
		return false
	}
	a, aok := before.Sys().(*syscall.Stat_t)
	b, bok := after.Sys().(*syscall.Stat_t)
	return aok && bok && a.Uid == b.Uid && a.Gid == b.Gid && a.Nlink == b.Nlink && a.Ctim == b.Ctim && a.Mtim == b.Mtim
}

// Observe independently reads host-owned account databases. Each observation reads the
// current fixed principal; unrelated completed host updates are not frozen to
// the initial helper's postimages. Root additionally checks private lock rows
// and actual NSS results before its effect. Runtime verifies its own kernel
// credentials separately and spawns no NSS helper in the Node cgroup.
func Observe(ctx context.Context, uid, gid uint32) (err error) {
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
	rows, err := ValidatePublic(files["/etc/passwd"].raw, files["/etc/group"].raw, files["/etc/nsswitch.conf"].raw, uid, gid)
	if err != nil {
		return ErrBinding
	}
	if os.Geteuid() == 0 {
		for _, path := range []string{"/etc/shadow", "/etc/gshadow"} {
			row, e := CurrentRow(files[path].raw)
			if e != nil || ValidateRow(path, row, uid, gid) != nil {
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
