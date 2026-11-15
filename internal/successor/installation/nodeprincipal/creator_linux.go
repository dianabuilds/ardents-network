package nodeprincipal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/ubuntu"
)

// Creator independently retains fixed Node account preimages, helper process
// and observed postimages. The caller admits and records every effect; this
// owner supplies no Release, journal, startup or cross-role authority.
type Creator struct {
	ctx                                   context.Context
	uid, gid                              uint32
	before                                map[string]*accountFile
	after                                 map[string]*accountFile
	auxBefore                             map[string]*accountFile
	auxAfter                              map[string]*accountFile
	hooks                                 map[string]*accountFile
	prepared, executed, completed, closed bool
}

// PrepareCreator retains the original fixed account preimages and selected identity.
func PrepareCreator(ctx context.Context) (*Creator, error) {
	a := &Creator{ctx: ctx, before: map[string]*accountFile{}, after: map[string]*accountFile{}, auxBefore: map[string]*accountFile{}, auxAfter: map[string]*accountFile{}, hooks: map[string]*accountFile{}}
	if ctx == nil {
		return a, ErrInput
	}
	for _, p := range []string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow", "/etc/nsswitch.conf", "/usr/lib/os-release", "/usr/bin/getent", "/etc/default/useradd", "/etc/login.defs", "/usr/sbin/useradd", "/usr/sbin/nologin"} {
		f, err := readNodeAccountFile(p)
		if err != nil {
			return a, err
		}
		a.before[p] = f
	}
	if err := ValidateNSS(a.before["/etc/nsswitch.conf"].raw); err != nil {
		return a, ErrBinding
	}
	if !ubuntu.Ubuntu2404(a.before["/usr/lib/os-release"].raw) {
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
	for _, path := range AuxiliaryPaths() {
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
	id, err := SelectID(a.before["/etc/passwd"].raw, a.before["/etc/group"].raw)
	if err != nil {
		return a, ErrBinding
	}
	a.uid, a.gid = id, id
	a.prepared = true
	return a, nil
}

// CheckBefore reobserves the original preimages and absence of helper hooks.
func (a *Creator) CheckBefore() error {
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

// AuxiliaryPaths returns the closed fixed account auxiliary inventory.
func AuxiliaryPaths() []string {
	return []string{"/etc/.pwd.lock", "/etc/passwd-", "/etc/group-", "/etc/shadow-", "/etc/gshadow-", "/etc/passwd.lock", "/etc/group.lock", "/etc/shadow.lock", "/etc/gshadow.lock", "/etc/passwd.1", "/etc/group.1", "/etc/shadow.1", "/etc/gshadow.1"}
}

// Execute runs and joins the fixed helper after the caller admits the effect.
func (a *Creator) Execute() error {
	if a == nil || !a.prepared || a.executed || a.closed {
		return ErrInput
	}
	a.executed = true
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
		if e = Delta(p, a.before[p].raw, f.raw, a.uid, a.gid); e != nil {
			return ErrBinding
		}
	}
	for _, path := range AuxiliaryPaths() {
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
	if e := a.Check(); e != nil {
		return e
	}
	a.completed = true
	return nil
}

// CheckAbsentNSS independently verifies the fixed account and ID are absent.
func (a *Creator) CheckAbsentNSS() error {
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
	return a.CheckBefore()
}

// Check reobserves the original helper postimages and unchanged dependencies.
func (a *Creator) Check() error {
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
	for _, path := range AuxiliaryPaths() {
		if a.auxAfter[path] == nil {
			if _, e := os.Lstat(path); !os.IsNotExist(e) {
				return errors.Join(ErrBinding, e)
			}
		}
	}
	return a.ctx.Err()
}

// Close releases the original files and erases their retained bytes.
func (a *Creator) Close() error {
	if a == nil {
		return nil
	}
	a.closed = true
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
func (a *Creator) checkHooks() error {
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
func trustedInputDirectory(info os.FileInfo) bool {
	if info == nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == 0
}
func nodeAccess(info os.FileInfo, mode os.FileMode, uid, gid uint32, directory bool) bool {
	if info == nil || info.IsDir() != directory || info.Mode().Perm() != mode || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky|os.ModeSymlink) != 0 || !directory && !info.Mode().IsRegular() {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == uid && native.Gid == gid && (directory || native.Nlink == 1)
}
func syncInitialParent(path string) (returned error) {
	before, err := os.Lstat(filepath.Dir(path))
	if err != nil || !trustedInputDirectory(before) {
		return errors.Join(ErrBinding, err)
	}
	file, err := os.OpenFile(filepath.Dir(path), os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, file.Close()) }()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) || !trustedInputDirectory(opened) || before.Mode() != opened.Mode() {
		return errors.Join(ErrBinding, err)
	}
	if err := file.Sync(); err != nil {
		return err
	}
	after, err := os.Lstat(filepath.Dir(path))
	if err != nil || !os.SameFile(opened, after) || !trustedInputDirectory(after) || opened.Mode() != after.Mode() {
		return errors.Join(ErrBinding, err)
	}
	return nil
}

// AccountRecord is detached metadata from this Creator's original file.
type AccountRecord struct {
	Path     string
	Info     os.FileInfo
	SHA256   string
	Mode     os.FileMode
	UID, GID uint32
}

// ID returns the fixed candidate numeric user and group identifiers.
func (a *Creator) ID() (uint32, uint32) { return a.uid, a.gid }

func accountRecord(path string, f *accountFile) AccountRecord {
	if f == nil {
		return AccountRecord{Path: path}
	}
	st := f.info.Sys().(*syscall.Stat_t)
	sum := sha256.Sum256(f.raw)
	return AccountRecord{Path: path, Info: f.info, SHA256: hex.EncodeToString(sum[:]), Mode: f.info.Mode().Perm(), UID: st.Uid, GID: st.Gid}
}

// Preimages returns detached records in the original fixed journal order.
func (a *Creator) Preimages() ([]AccountRecord, error) {
	if a == nil || !a.prepared || a.closed {
		return nil, ErrInput
	}
	var records []AccountRecord
	for _, path := range []string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow"} {
		records = append(records, accountRecord(path, a.before[path]))
	}
	for _, path := range AuxiliaryPaths() {
		records = append(records, accountRecord(path, a.auxBefore[path]))
	}
	return records, nil
}

// SyncResult synchronizes and checks the original postimage for one fixed path.
// An absent auxiliary path has no file to sync and must still be journaled by the caller.
func (a *Creator) SyncResult(path string) (AccountRecord, string, bool, error) {
	if a == nil || !a.completed || a.closed {
		return AccountRecord{}, "", false, ErrInput
	}
	allowed := false
	for _, fixed := range append([]string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow"}, AuxiliaryPaths()...) {
		if fixed == path {
			allowed = true
			break
		}
	}
	if !allowed {
		return AccountRecord{}, "", false, ErrInput
	}
	f, phase := a.after[path], "account-born"
	if f == nil {
		f, phase = a.auxAfter[path], "account-auxiliary-result"
	}
	if f == nil {
		return AccountRecord{Path: path}, "account-auxiliary-cleared", false, nil
	}
	if err := errors.Join(f.file.Sync(), syncInitialParent(path), checkNodeAccountFile(f)); err != nil {
		return AccountRecord{}, "", false, err
	}
	return accountRecord(path, f), phase, true, nil
}
