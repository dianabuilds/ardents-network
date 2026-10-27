package forwarding

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

// Adapted from the maintained Source native account mechanism; this distinct
// owner binds only the fixed shared Node principal, with no Source lifetime.
// nodeAccount is born only after a successful original fixed useradd, exact
// preimage comparison and numeric identity binding. A detached passwd row or
// preflight observation cannot construct it. Retained failures are not adopted.
type accountFile struct {
	path string
	file *os.File
	info os.FileInfo
	raw  []byte
}

type nodeAccount struct {
	ctx       context.Context
	uid, gid  uint32
	before    map[string]*accountFile
	after     map[string]*accountFile
	auxBefore map[string]*accountFile
	auxAfter  map[string]*accountFile
	hooks     map[string]*accountFile
}

func prepareNodeAccount(ctx context.Context, inputs Inputs) (*nodeAccount, error) {
	a := &nodeAccount{ctx: ctx, before: map[string]*accountFile{}, after: map[string]*accountFile{}, auxBefore: map[string]*accountFile{}, auxAfter: map[string]*accountFile{}, hooks: map[string]*accountFile{}}
	for _, p := range []string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow", "/etc/nsswitch.conf", "/usr/lib/os-release", "/usr/bin/getent", "/etc/default/useradd", "/etc/login.defs", "/usr/sbin/useradd", "/usr/sbin/nologin"} {
		f, err := readNodeAccountFile(p)
		if err != nil {
			return a, err
		}
		a.before[p] = f
	}
	if err := validateNodeNSS(a.before["/etc/nsswitch.conf"].raw); err != nil {
		return a, err
	}
	operatingSystem := map[string]string{}
	for _, line := range strings.Split(string(a.before["/usr/lib/os-release"].raw), "\n") {
		name, value, ok := strings.Cut(line, "=")
		if ok {
			if _, duplicate := operatingSystem[name]; duplicate {
				return a, ErrBinding
			}
			operatingSystem[name] = strings.Trim(value, "\"")
		}
	}
	if operatingSystem["ID"] != "ubuntu" || operatingSystem["VERSION_ID"] != "24.04" {
		return a, ErrBinding
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
		hook := &accountFile{path: p, file: f}
		a.hooks[p] = hook
		hook.info, err = f.Stat()
		if err != nil || !trustedInputDirectory(hook.info) {
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
	for _, path := range nodeAccountAuxiliaryPaths() {
		f, e := readNodeAccountFile(path)
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
			if len(fields) < 3 || fields[0] == "ardents-node" {
				return a, ErrBinding
			}
			n, err := strconv.ParseUint(fields[2], 10, 32)
			if err != nil {
				return a, ErrBinding
			}
			used[uint32(n)] = true
			if p == "/etc/passwd" {
				if len(fields) != 7 {
					return a, ErrBinding
				}
				primary, err := strconv.ParseUint(fields[3], 10, 32)
				if err != nil {
					return a, ErrBinding
				}
				used[uint32(primary)] = true
			}
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
	return a, errors.Join(inputs.Check(), a.checkBefore(), a.checkAbsentNSS())
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

func (a *nodeAccount) checkBefore() error {
	if e := a.checkHooks(); e != nil {
		return e
	}
	for _, f := range a.before {
		if err := checkNodeAccountFile(f); err != nil {
			return err
		}
	}
	for path, f := range a.auxBefore {
		if f == nil {
			if _, e := os.Lstat(path); !os.IsNotExist(e) {
				return errors.Join(ErrBinding, e)
			}
		} else if e := checkNodeAccountFile(f); e != nil {
			return e
		}
	}
	return a.ctx.Err()
}

func nodeAccountAuxiliaryPaths() []string {
	return []string{"/etc/.pwd.lock", "/etc/passwd-", "/etc/group-", "/etc/shadow-", "/etc/gshadow-", "/etc/passwd.lock", "/etc/group.lock", "/etc/shadow.lock", "/etc/gshadow.lock", "/etc/passwd.1", "/etc/group.1", "/etc/shadow.1", "/etc/gshadow.1"}
}

func (a *nodeAccount) create(j *initialBirths, inputs Inputs, admit, admitCreated func() error) error {
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
	for _, path := range nodeAccountAuxiliaryPaths() {
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
	if err := j.record("account-intent", "ardents-node", nil, "", 0, a.uid, a.gid); err != nil {
		return err
	}
	if err := errors.Join(a.checkBefore(), inputs.Check()); err != nil {
		return err
	}
	if err := admit(); err != nil {
		return err
	}
	if err := a.checkAbsentNSS(); err != nil {
		return err
	}
	if err := admit(); err != nil {
		return err
	}
	bounded, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	id := strconv.FormatUint(uint64(a.uid), 10)
	cmd := exec.CommandContext(bounded, "/usr/sbin/useradd", "--system", "--user-group", "--uid", id, "--no-create-home", "--no-log-init", "--home-dir", "/nonexistent", "--shell", "/usr/sbin/nologin", "--comment", "", "-K", "SYS_GID_MIN="+id, "-K", "SYS_GID_MAX="+id, "ardents-node")
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	var diagnostic nodeAccountDiagnostic
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
		return fmt.Errorf("original fixed Node account helper: %w: %s", err, diagnostic.body.String())
	}
	for _, p := range []string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow"} {
		f, e := readNodeAccountFile(p)
		if e != nil {
			return e
		}
		a.after[p] = f
		if e = nodeAccountDelta(p, a.before[p].raw, f.raw, a.uid, a.gid); e != nil {
			return e
		}
	}
	for _, path := range nodeAccountAuxiliaryPaths() {
		if strings.HasSuffix(path, "-") || path == "/etc/.pwd.lock" {
			f, e := readNodeAccountFile(path)
			if e != nil {
				return e
			}
			a.auxAfter[path] = f
			if path == "/etc/.pwd.lock" {
				if !nodeAccess(f.info, 0600, 0, 0, false) || len(f.raw) != 0 {
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
	for _, path := range append([]string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow"}, nodeAccountAuxiliaryPaths()...) {
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
		if e := errors.Join(f.file.Sync(), syncInitialParent(path), checkNodeAccountFile(f)); e != nil {
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
	return j.record("account-ready", "ardents-node", nil, "", 0, a.uid, a.gid)
}

func (a *nodeAccount) checkAbsentNSS() error {
	for _, query := range []struct{ database, key string }{
		{"passwd", "ardents-node"}, {"group", "ardents-node"},
		{"passwd", strconv.FormatUint(uint64(a.uid), 10)}, {"group", strconv.FormatUint(uint64(a.gid), 10)},
	} {
		if err := checkNodeAccountFile(a.before["/usr/bin/getent"]); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(a.ctx, 3*time.Second)
		command := exec.CommandContext(ctx, "/usr/bin/getent", query.database, query.key)
		command.Env = []string{"LANG=C", "LC_ALL=C", "PATH=/usr/bin:/bin"}
		var output nodeAccountDiagnostic
		command.Stdout = &output
		command.Stderr = &output
		runtime.LockOSThread()
		command.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL, Cloneflags: syscall.CLONE_NEWPID}
		err := command.Run()
		runtime.UnlockOSThread()
		ctxErr := ctx.Err()
		cancel()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 || output.body.Len() != 0 || ctxErr != nil || a.ctx.Err() != nil {
			return errors.Join(ErrBinding, err, ctxErr, a.ctx.Err())
		}
	}
	return a.checkBefore()
}

type nodeAccountDiagnostic struct{ body strings.Builder }

func (d *nodeAccountDiagnostic) Write(raw []byte) (int, error) {
	if d.body.Len()+len(raw) > 4096 {
		return 0, ErrBinding
	}
	return d.body.Write(raw)
}

func nodeAccountDelta(path string, before, after []byte, uid, gid uint32) error {
	if len(before) == 0 || before[len(before)-1] != '\n' || len(after) == 0 || after[len(after)-1] != '\n' {
		return ErrBinding
	}
	var own string
	var rest strings.Builder
	for _, line := range strings.SplitAfter(string(after), "\n") {
		if strings.HasPrefix(line, "ardents-node:") {
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
	return validateNodePrincipalRow(path, own, uid, gid)
}

// The initial creator checks its foreign-byte delta separately. Later native
// admission independently checks the same fixed principal's current rows; this
// pure condition supplies neither creation provenance nor account ownership.
func validateNodePrincipalRow(path, own string, uid, gid uint32) error {
	if !strings.HasPrefix(own, "ardents-node:") {
		return ErrBinding
	}
	u, g := strconv.FormatUint(uint64(uid), 10), strconv.FormatUint(uint64(gid), 10)
	switch path {
	case "/etc/passwd":
		if own != "ardents-node:x:"+u+":"+g+"::/nonexistent:/usr/sbin/nologin" {
			return ErrBinding
		}
	case "/etc/group":
		if own != "ardents-node:x:"+g+":" {
			return ErrBinding
		}
	case "/etc/gshadow":
		if own != "ardents-node:!::" {
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

func (a *nodeAccount) check() error {
	if e := a.checkHooks(); e != nil {
		return e
	}
	if len(a.after) != 4 {
		return ErrBinding
	}
	for p, f := range a.before {
		if a.after[p] == nil {
			if err := checkNodeAccountFile(f); err != nil {
				return err
			}
		}
	}
	for _, f := range a.after {
		if err := checkNodeAccountFile(f); err != nil {
			return err
		}
	}
	if len(a.auxAfter) != 5 {
		return ErrBinding
	}
	for _, f := range a.auxAfter {
		if e := checkNodeAccountFile(f); e != nil {
			return e
		}
	}
	for _, path := range nodeAccountAuxiliaryPaths() {
		if a.auxAfter[path] == nil {
			if _, e := os.Lstat(path); !os.IsNotExist(e) {
				return errors.Join(ErrBinding, e)
			}
		}
	}
	return a.ctx.Err()
}
func (a *nodeAccount) close() error {
	if a == nil {
		return nil
	}
	var err error
	for _, set := range []map[string]*accountFile{a.after, a.before, a.auxBefore, a.auxAfter, a.hooks} {
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

func (a *nodeAccount) checkHooks() error {
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
		if err != nil || e != nil || !sameInputFile(original.info, info) || !sameInputFile(original.info, named) {
			return errors.Join(ErrBinding, err, e)
		}
		entries, e := os.ReadDir(path)
		if e != nil || len(entries) != 0 {
			return errors.Join(ErrBinding, e)
		}
		named, e = os.Lstat(path)
		if e != nil || !sameInputFile(original.info, named) {
			return errors.Join(ErrBinding, e)
		}
	}
	return a.ctx.Err()
}
