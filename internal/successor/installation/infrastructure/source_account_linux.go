package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// sourceAccount is born only after a successful original fixed useradd, exact
// preimage comparison and numeric identity binding. A detached passwd row or
// preflight observation cannot construct it. Retained failures are not adopted.
type sourceAccount struct {
	ctx       context.Context
	uid, gid  uint32
	before    map[string]*sourceFile
	after     map[string]*sourceFile
	auxBefore map[string]*sourceFile
	auxAfter  map[string]*sourceFile
	hooks     map[string]*sourceFile
}

func prepareSourceAccount(ctx context.Context, inputs SourceInputs) (*sourceAccount, error) {
	a := &sourceAccount{ctx: ctx, before: map[string]*sourceFile{}, after: map[string]*sourceFile{}, auxBefore: map[string]*sourceFile{}, auxAfter: map[string]*sourceFile{}, hooks: map[string]*sourceFile{}}
	for _, p := range []string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow", "/etc/default/useradd", "/etc/login.defs", "/usr/sbin/useradd", "/usr/sbin/nologin"} {
		f, err := readSourceAccountFile(p)
		if err != nil {
			return a, err
		}
		a.before[p] = f
	}
	// Fixed native argv disables home/mail/subordinate-ID/log initialization.
	// Reject installation-specific scripts and supplementary-group defaults.
	for _, p := range []string{"/etc/shadow-maint/useradd-pre.d", "/etc/shadow-maint/useradd-post.d"} {
		f, err := os.OpenFile(p, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
		if os.IsNotExist(err) {
			a.hooks[p] = nil
			continue
		}
		if err != nil {
			return a, err
		}
		hook := &sourceFile{path: p, file: f}
		a.hooks[p] = hook
		hook.info, err = f.Stat()
		if err != nil || !trustedDirectory(hook.info) {
			return a, errors.Join(ErrBinding, err)
		}
	}
	if err := a.checkHooks(); err != nil {
		return a, err
	}
	for _, line := range strings.Split(string(a.before["/etc/default/useradd"].raw), "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if strings.HasPrefix(line, "GROUPS=") && strings.Trim(strings.TrimPrefix(line, "GROUPS="), "\"' ") != "" {
			return a, ErrBinding
		}
	}
	for _, path := range sourceAccountAuxiliaryPaths() {
		f, e := readSourceAccountFile(path)
		if e != nil && !os.IsNotExist(e) {
			return a, e
		}
		if (strings.HasSuffix(path, ".lock") && path != "/etc/.pwd.lock" || strings.HasSuffix(path, ".1")) && f != nil {
			err := errors.Join(ErrBinding, f.file.Close())
			clear(f.raw)
			return a, err
		}
		a.auxBefore[path] = f
	}
	used := map[uint32]bool{}
	for _, p := range []string{"/etc/passwd", "/etc/group"} {
		for _, line := range strings.Split(string(a.before[p].raw), "\n") {
			if line == "" {
				continue
			}
			fields := strings.Split(line, ":")
			if len(fields) < 3 || fields[0] == "ardents-source" {
				return a, ErrBinding
			}
			n, err := strconv.ParseUint(fields[2], 10, 32)
			if err != nil {
				return a, ErrBinding
			}
			used[uint32(n)] = true
		}
	}
	for id := uint32(999); id >= 100; id-- {
		if !used[id] {
			a.uid, a.gid = id, id
			break
		}
	}
	if a.uid == 0 {
		return a, ErrBinding
	}
	return a, errors.Join(inputs.Check(), a.checkBefore())
}

func readSourceAccountFile(path string) (result *sourceFile, err error) {
	// Input paths are a closed literal inventory, never caller-selected helpers.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	result = &sourceFile{path: path, file: f}
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
	return result, checkSourceAccountFile(result)
}

func checkSourceAccountFile(f *sourceFile) error {
	info, err := f.file.Stat()
	named, ne := os.Lstat(f.path)
	if err != nil || ne != nil || !sameSourceFile(f.info, info) || !sameSourceFile(f.info, named) {
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
	if err != nil || ne != nil || !sameSourceFile(f.info, info) || !sameSourceFile(f.info, named) {
		return errors.Join(ErrBinding, err, ne)
	}
	return nil
}

func (a *sourceAccount) checkBefore() error {
	if e := a.checkHooks(); e != nil {
		return e
	}
	for _, f := range a.before {
		if err := checkSourceAccountFile(f); err != nil {
			return err
		}
	}
	for path, f := range a.auxBefore {
		if f == nil {
			if _, e := os.Lstat(path); !os.IsNotExist(e) {
				return errors.Join(ErrBinding, e)
			}
		} else if e := checkSourceAccountFile(f); e != nil {
			return e
		}
	}
	return a.ctx.Err()
}

func sourceAccountAuxiliaryPaths() []string {
	return []string{"/etc/.pwd.lock", "/etc/passwd-", "/etc/group-", "/etc/shadow-", "/etc/gshadow-", "/etc/passwd.lock", "/etc/group.lock", "/etc/shadow.lock", "/etc/gshadow.lock", "/etc/passwd.1", "/etc/group.1", "/etc/shadow.1", "/etc/gshadow.1"}
}

func (a *sourceAccount) create(j *sourceBirths, inputs SourceInputs, admit, admitCreated func() error) error {
	if err := errors.Join(a.checkBefore(), inputs.Check()); err != nil {
		return err
	}
	for _, p := range []string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow"} {
		f := a.before[p]
		st := f.info.Sys().(*syscall.Stat_t)
		if err := j.record("account-preimage", p, f.info, digest(f.raw), f.info.Mode().Perm(), st.Uid, st.Gid); err != nil {
			return err
		}
	}
	for _, path := range sourceAccountAuxiliaryPaths() {
		f := a.auxBefore[path]
		var info os.FileInfo
		var sha string
		var mode os.FileMode
		var uid, gid uint32
		if f != nil {
			info, sha, mode = f.info, digest(f.raw), f.info.Mode().Perm()
			st := f.info.Sys().(*syscall.Stat_t)
			uid, gid = st.Uid, st.Gid
		}
		if e := j.record("account-auxiliary-preimage", path, info, sha, mode, uid, gid); e != nil {
			return e
		}
	}
	if err := j.record("account-intent", "ardents-source", nil, "", 0, a.uid, a.gid); err != nil {
		return err
	}
	if err := errors.Join(a.checkBefore(), inputs.Check()); err != nil {
		return err
	}
	if err := admit(); err != nil {
		return err
	}
	bounded, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	id := strconv.FormatUint(uint64(a.uid), 10)
	cmd := exec.CommandContext(bounded, "/usr/sbin/useradd", "--system", "--user-group", "--uid", id, "--no-create-home", "--no-log-init", "--home-dir", "/nonexistent", "--shell", "/usr/sbin/nologin", "--comment", "", "-K", "SYS_GID_MIN="+id, "-K", "SYS_GID_MAX="+id, "ardents-source")
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	var diagnostic sourceAccountDiagnostic
	cmd.Stderr = &diagnostic
	// The fixed helper is PID1 of its own new PID namespace. Kernel teardown
	// joins all namespace descendants before that init process can be reaped;
	// parent-death SIGKILL also tears down that namespace on original death.
	// This does not rely on Wait alone, missing PIDs or a bounded observation.
	runtime.LockOSThread()
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL, Cloneflags: syscall.CLONE_NEWPID}
	err := cmd.Run()
	runtime.UnlockOSThread()
	if err = errors.Join(err, bounded.Err(), a.ctx.Err()); err != nil {
		return fmt.Errorf("original fixed Source account helper: %w: %s", err, diagnostic.body.String())
	}
	for _, p := range []string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow"} {
		f, e := readSourceAccountFile(p)
		if e != nil {
			return e
		}
		a.after[p] = f
		if e = sourceAccountDelta(p, a.before[p].raw, f.raw, a.uid, a.gid); e != nil {
			return e
		}
	}
	for _, path := range sourceAccountAuxiliaryPaths() {
		if strings.HasSuffix(path, "-") || path == "/etc/.pwd.lock" {
			f, e := readSourceAccountFile(path)
			if e != nil {
				return e
			}
			a.auxAfter[path] = f
			if path == "/etc/.pwd.lock" {
				if !sourceAccess(f.info, 0600, 0, 0, false) || len(f.raw) != 0 {
					return ErrBinding
				}
				if before := a.auxBefore[path]; before != nil && !os.SameFile(before.info, f.info) {
					return ErrBinding
				}
			} else if !bytes.Equal(f.raw, a.before[strings.TrimSuffix(path, "-")].raw) {
				return ErrBinding
			}
		} else {
			if _, e := os.Lstat(path); !os.IsNotExist(e) {
				return errors.Join(ErrBinding, e)
			}
		}
	}
	if e := a.check(); e != nil {
		return e
	}
	// The successful original helper and exact detached shape establish private
	// created-account custody. Reobserve all original owners under that phase
	// before any subsequent sync or journal effect; old-account checks cannot
	// accept our own atomic database replacements.
	for _, path := range append([]string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow"}, sourceAccountAuxiliaryPaths()...) {
		if e := admitCreated(); e != nil {
			return e
		}
		f, phase := a.after[path], "account-born"
		if f == nil {
			f, phase = a.auxAfter[path], "account-auxiliary-result"
		}
		if f == nil {
			if e := j.record("account-auxiliary-cleared", path, nil, "", 0, 0, 0); e != nil {
				return e
			}
			continue
		}
		if e := errors.Join(f.file.Sync(), syncSourceParent(path), checkSourceAccountFile(f)); e != nil {
			return e
		}
		if e := admitCreated(); e != nil {
			return e
		}
		st := f.info.Sys().(*syscall.Stat_t)
		if e := j.record(phase, path, f.info, digest(f.raw), f.info.Mode().Perm(), st.Uid, st.Gid); e != nil {
			return e
		}
	}
	if e := admitCreated(); e != nil {
		return e
	}
	return j.record("account-ready", "ardents-source", nil, "", 0, a.uid, a.gid)
}

type sourceAccountDiagnostic struct{ body strings.Builder }

func (d *sourceAccountDiagnostic) Write(raw []byte) (int, error) {
	if d.body.Len()+len(raw) > 4096 {
		return 0, ErrBinding
	}
	return d.body.Write(raw)
}

func sourceAccountDelta(path string, before, after []byte, uid, gid uint32) error {
	if len(before) == 0 || before[len(before)-1] != '\n' || len(after) == 0 || after[len(after)-1] != '\n' {
		return ErrBinding
	}
	var own string
	var rest strings.Builder
	for _, line := range strings.SplitAfter(string(after), "\n") {
		if strings.HasPrefix(line, "ardents-source:") {
			if own != "" {
				return ErrBinding
			}
			own = strings.TrimSuffix(line, "\n")
		} else {
			rest.WriteString(line)
		}
	}
	if rest.String() != string(before) || own == "" {
		return ErrBinding
	}
	u, g := strconv.FormatUint(uint64(uid), 10), strconv.FormatUint(uint64(gid), 10)
	switch path {
	case "/etc/passwd":
		if own != "ardents-source:x:"+u+":"+g+"::/nonexistent:/usr/sbin/nologin" {
			return ErrBinding
		}
	case "/etc/group":
		if own != "ardents-source:x:"+g+":" {
			return ErrBinding
		}
	case "/etc/gshadow":
		if own != "ardents-source:!::" {
			return ErrBinding
		}
	case "/etc/shadow":
		f := strings.Split(own, ":")
		if len(f) != 9 || f[1] != "!" {
			return ErrBinding
		}
		if _, e := strconv.ParseUint(f[2], 10, 32); e != nil {
			return ErrBinding
		}
		for _, v := range f[3:] {
			if v != "" {
				return ErrBinding
			}
		}
	default:
		return ErrBinding
	}
	return nil
}

func (a *sourceAccount) replaces(path string) bool { return a != nil && a.after[path] != nil }
func (a *sourceAccount) checkOriginal(f *sourceFile) error {
	before := a.before[f.path]
	if before == nil || !os.SameFile(before.info, f.info) || !bytes.Equal(before.raw, f.raw) {
		return ErrBinding
	}
	// useradd may atomically replace its databases. The still-open old inode
	// remains a preimage, never the current account observation.
	info, err := f.file.Stat()
	if err != nil || !os.SameFile(f.info, info) || info.Size() != int64(len(f.raw)) {
		return errors.Join(ErrBinding, err)
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
	return nil
}
func (a *sourceAccount) check() error {
	if e := a.checkHooks(); e != nil {
		return e
	}
	if len(a.after) != 4 {
		return ErrBinding
	}
	for p, f := range a.before {
		if a.after[p] == nil {
			if err := checkSourceAccountFile(f); err != nil {
				return err
			}
		}
	}
	for _, f := range a.after {
		if err := checkSourceAccountFile(f); err != nil {
			return err
		}
	}
	if len(a.auxAfter) != 5 {
		return ErrBinding
	}
	for _, f := range a.auxAfter {
		if e := checkSourceAccountFile(f); e != nil {
			return e
		}
	}
	for _, path := range sourceAccountAuxiliaryPaths() {
		if a.auxAfter[path] == nil {
			if _, e := os.Lstat(path); !os.IsNotExist(e) {
				return errors.Join(ErrBinding, e)
			}
		}
	}
	return a.ctx.Err()
}
func (a *sourceAccount) close() error {
	if a == nil {
		return nil
	}
	var err error
	for _, set := range []map[string]*sourceFile{a.after, a.before, a.auxBefore, a.auxAfter, a.hooks} {
		for _, f := range set {
			if f == nil {
				continue
			}
			err = errors.Join(err, f.file.Close())
			clear(f.raw)
		}
	}
	return err
}

func (a *sourceAccount) checkHooks() error {
	// The reviewed shadow helper can spawn these fixed cache flush commands.
	// Their effects are outside this finite local-files account contract.
	for _, path := range []string{"/usr/sbin/nscd", "/usr/sbin/sss_cache"} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			return errors.Join(ErrBinding, err)
		}
	}
	for path, original := range a.hooks {
		named, err := os.Lstat(path)
		if original == nil {
			if !os.IsNotExist(err) {
				return errors.Join(ErrBinding, err)
			}
			continue
		}
		info, e := original.file.Stat()
		if err != nil || e != nil || !sameSourceFile(original.info, info) || !sameSourceFile(original.info, named) {
			return errors.Join(ErrBinding, err, e)
		}
		entries, e := os.ReadDir(path)
		if e != nil || len(entries) != 0 {
			return errors.Join(ErrBinding, e)
		}
		named, e = os.Lstat(path)
		if e != nil || !sameSourceFile(original.info, named) {
			return errors.Join(ErrBinding, e)
		}
	}
	return a.ctx.Err()
}
