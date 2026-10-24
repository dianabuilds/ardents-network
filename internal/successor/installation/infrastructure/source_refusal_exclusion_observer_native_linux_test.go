//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// Observe the real Linux amd64 syscall, not a Go error string or a detached
// external lease. Capture an exact target fd at FLOCK_EX|FLOCK_NB entry and
// require its paired -EWOULDBLOCK exit while the same original inode is live.
// No first-party unsafe, instruction rewrite or supplied syscall result.
type sourceRefusalExclusion struct {
	path, kind, pin string
	mode            os.FileMode
	info            os.FileInfo
	pending         map[int]int
	observed        bool
}

func newSourceRefusalExclusion(root, scenario string, profile *sourceRefusalObserverProfile) *sourceRefusalExclusion {
	journal := scenario == "refusal-held" || scenario == "refusal-held-unattempted"
	release := scenario == "refusal-held-release" || scenario == "refusal-held-both"
	path, lock, pin, mode := filepath.Join(root, "original-journal"), filepath.Join(root, "release-lock"), "", os.FileMode(0600)
	if profile != nil {
		journal, release = profile.ExternalJournalLease, profile.ExternalReleaseLease
		path, lock, pin = profile.Arguments[7], filepath.Join(profile.Arguments[3], ".ardents-release-decision-lock"), profile.JournalSHA256
		if profile.JournalMode == "0640" {
			mode = 0640
		}
	}
	// Canonical Release acquires its own exclusion before Source journal use.
	// If both external owners exist, require the actual first Release refusal;
	// do not pretend that this caller reached the later journal acquisition.
	if release {
		return &sourceRefusalExclusion{path: lock, kind: "release", mode: 0600, pending: map[int]int{}}
	}
	if journal {
		return &sourceRefusalExclusion{path: path, kind: "journal", pin: pin, mode: mode, pending: map[int]int{}}
	}
	return nil
}

func (p *sourceRefusalExclusion) capture() (returned error) {
	if err := probeSourceRefusalLease(p.path, p.pin, p.mode, true); err != nil {
		return err
	}
	f, err := os.OpenFile(p.path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, f.Close()) }()
	p.info, err = f.Stat()
	if err != nil || !sourceAccess(p.info, p.mode, 0, 0, false) {
		return errors.Join(errors.New("original externally held exclusion inode required"), err)
	}
	return nil
}

func (p *sourceRefusalExclusion) syscall(pid, tid int, result *sourceCutObservation) error {
	var registers syscall.PtraceRegs
	if err := syscall.PtraceGetRegs(tid, &registers); err != nil {
		return err
	}
	if registers.Orig_rax != syscall.SYS_FLOCK {
		return nil
	}
	// On native x86-64 syscall-entry stops RAX is -ENOSYS. For this paired
	// supported FLOCK syscall only -EWOULDBLOCK at exit proves exclusion.
	// An unsupported/ambiguous syscall cannot produce an accepting result.
	if int64(registers.Rax) == -int64(syscall.ENOSYS) {
		if registers.Rsi != syscall.LOCK_EX|syscall.LOCK_NB {
			return nil
		}
		fd := int(registers.Rdi)
		actual, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid), "fd", strconv.Itoa(fd)))
		if err != nil || p.info == nil || !os.SameFile(p.info, actual) {
			return nil // An unrelated lock/invalid fd cannot prove this exclusion.
		}
		if _, exists := p.pending[tid]; exists {
			return errors.New("original exclusion syscall entry repeated without paired exit")
		}
		p.pending[tid] = fd
		return nil
	}
	fd, exists := p.pending[tid]
	if !exists {
		return nil
	}
	delete(p.pending, tid)
	if int64(registers.Rax) != -int64(syscall.EWOULDBLOCK) {
		return fmt.Errorf("original exclusive flock returned %d instead of actual EWOULDBLOCK", int64(registers.Rax))
	}
	actual, fdErr := os.Stat(filepath.Join("/proc", strconv.Itoa(pid), "fd", strconv.Itoa(fd)))
	named, namedErr := os.Lstat(p.path)
	if fdErr != nil || namedErr != nil || !sameSourceFile(p.info, actual) || !sameSourceFile(p.info, named) {
		return errors.Join(errors.New("original exclusion fd/inode changed across actual syscall"), fdErr, namedErr)
	}
	p.observed = true
	native := p.info.Sys().(*syscall.Stat_t)
	result.RefusalExclusionObserved, result.RefusalExclusionKind = true, p.kind
	result.RefusalExclusionDevice, result.RefusalExclusionInode = uint64(native.Dev), native.Ino
	return nil
}

func (p *sourceRefusalExclusion) verify() error {
	if !p.observed || p.info == nil || len(p.pending) != 0 {
		return errors.New("original exclusive flock refusal not observed before return")
	}
	return nil
}
