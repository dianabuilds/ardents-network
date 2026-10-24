//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
)

type sourcePendingGuardProfile struct {
	Attempt uint32 `json:"attempt"`
	GID     uint32 `json:"gid"`
}

func (p *sourceRefusalObserverProfile) validStartupPendingObservation(root string) bool {
	return p != nil && p.EarlyInitialPrefix == 0 && p.StartupPending != nil && p.Schema == "source-startup-recovery-refusal-observer-v1" &&
		p.RetirementPending == nil &&
		p.StartupPending.Attempt < uint32(sourceAttemptLimit) && p.StartupPending.GID >= 100 && p.StartupPending.GID <= 999 &&
		len(p.Arguments) == 8 && p.Arguments[7] == sourceAttemptNumber(p.StartupPending.Attempt).journal(root) &&
		p.JournalMode == "0600" && p.RecoveryHold == nil && !p.MissingOriginalCut && !p.ExternalJournalLease && !p.ExternalReleaseLease
}

type sourcePendingGuardCall struct {
	path, stat uint64
}

// A successful actual nofollow Lstat of precisely the original retained guard
// is the causal event. A generic error or another successful stat cannot stand
// in for the product owner's explicit pending refusal.
type sourcePendingGuard struct {
	retirement         bool
	journal, path, pin string
	group              uint32
	journalInfo, guard os.FileInfo
	guardSHA           string
	parents            map[string]os.FileInfo
	calls              map[int]sourcePendingGuardCall
	observed           bool
}

func sourceStartupPendingNativeCase(scenario string) bool {
	return scenario == "refusal-startup-pending" || scenario == "refusal-startup-pending-unattempted" || scenario == "refusal-startup-pending-foreign" || scenario == "refusal-startup-pending-follow"
}

func newSourcePendingGuard(root, scenario string, profile *sourceRefusalObserverProfile) *sourcePendingGuard {
	journal, path, pin, group := filepath.Join(root, "original-journal"), filepath.Join(root, "source-start-pending"), "", uint32(0)
	retirement := sourceRetirementPendingNativeCase(scenario) || profile != nil && profile.RetirementPending != nil
	if profile != nil {
		pending := profile.StartupPending
		if retirement {
			pending = profile.RetirementPending
		}
		if pending == nil {
			return nil
		}
		journal, pin, group = profile.Arguments[7], profile.JournalSHA256, pending.GID
		path = filepath.Join(sourceAttemptNumber(pending.Attempt).directory(root), "source-start-pending")
	} else if !sourceStartupPendingNativeCase(scenario) && !retirement {
		return nil
	}
	if retirement {
		path = filepath.Join(filepath.Dir(path), "source-retirement-pending")
	}
	return &sourcePendingGuard{retirement: retirement, journal: journal, path: path, pin: pin, group: group, parents: map[string]os.FileInfo{}, calls: map[int]sourcePendingGuardCall{}}
}

func (p *sourcePendingGuard) parent(path string, info os.FileInfo) bool {
	if path == filepath.Dir(p.path) && p.group != 0 {
		return sourceAccess(info, 0750, 0, p.group, true)
	}
	return trustedDirectory(info)
}

func (p *sourcePendingGuard) capture() error {
	raw, info, err := readSourceCutObserverFile(p.journal, 4<<20)
	if err != nil || p.pin != "" && sourcePhysicalSHA(raw) != p.pin {
		return errors.Join(errors.New("original pending refusal journal pin differs"), err)
	}
	p.pin, p.journalInfo = sourcePhysicalSHA(raw), info
	guard, guardInfo, err := readSourceCutObserverFile(p.path, 4096)
	if err != nil {
		return err
	}
	p.guardSHA, p.guard = sourcePhysicalSHA(guard), guardInfo
	for path := filepath.Dir(p.path); ; path = filepath.Dir(path) {
		info, err := os.Lstat(path)
		if err != nil || !p.parent(path, info) {
			return errors.Join(errors.New("original pending guard parent identity/access required"), err)
		}
		p.parents[path] = info
		if path == "/" {
			break
		}
	}
	return p.check()
}

func (p *sourcePendingGuard) check() error {
	if p.guard == nil || p.journalInfo == nil || len(p.parents) == 0 {
		return errors.New("pending guard observation has no original custody")
	}
	for path, original := range p.parents {
		info, err := os.Lstat(path)
		if err != nil || !p.parent(path, info) || !os.SameFile(original, info) || original.Mode() != info.Mode() {
			return errors.Join(errors.New("original pending guard parent changed"), err)
		}
	}
	for _, original := range []struct {
		path, pin string
		info      os.FileInfo
	}{{p.journal, p.pin, p.journalInfo}, {p.path, p.guardSHA, p.guard}} {
		raw, info, err := readSourceCutObserverFile(original.path, 4<<20)
		if err != nil || !sameSourceFile(original.info, info) || sourcePhysicalSHA(raw) != original.pin {
			return errors.Join(errors.New("original pending guard/journal changed"), err)
		}
	}
	return nil
}

func (p *sourcePendingGuard) syscall(tid int, result *sourceCutObservation) error {
	var registers syscall.PtraceRegs
	if err := syscall.PtraceGetRegs(tid, &registers); err != nil {
		return err
	}
	if registers.Orig_rax != syscall.SYS_NEWFSTATAT {
		return nil
	}
	const nofollow = 0x100 // Linux amd64 AT_SYMLINK_NOFOLLOW used by os.Lstat.
	if int64(registers.Rax) == -int64(syscall.ENOSYS) {
		if int64(registers.Rdi) != -100 || registers.R10 != nofollow {
			return nil
		}
		path, err := sourceMissingCutPath(tid, registers.Rsi)
		if err != nil || path != p.path {
			return err
		}
		if _, exists := p.calls[tid]; exists {
			return errors.New("pending guard Lstat entered without paired exit")
		}
		if err := p.check(); err != nil {
			return err
		}
		p.calls[tid] = sourcePendingGuardCall{path: registers.Rsi, stat: registers.Rdx}
		return nil
	}
	call, exists := p.calls[tid]
	if !exists {
		return nil
	}
	delete(p.calls, tid)
	if registers.Rax != 0 || int64(registers.Rdi) != -100 || registers.Rsi != call.path || registers.Rdx != call.stat || registers.R10 != nofollow {
		return errors.New("actual original pending guard Lstat did not return its paired success")
	}
	path, err := sourceMissingCutPath(tid, call.path)
	if err != nil || path != p.path {
		return errors.Join(errors.New("original pending guard syscall path changed"), err)
	}
	// Read the returned Linux stat words using the same-source inspected ABI;
	// no unsafe, rewritten registers or supplied syscall result is involved.
	typeOf := reflect.TypeFor[syscall.Stat_t]()
	word := func(name string) (uint64, error) {
		field, ok := typeOf.FieldByName(name)
		if !ok {
			return 0, fmt.Errorf("actual Linux stat field %s unavailable", name)
		}
		return sourcePhysicalWord(tid, call.stat, field.Offset)
	}
	device, err := word("Dev")
	if err != nil {
		return err
	}
	inode, err := word("Ino")
	if err != nil {
		return err
	}
	links, err := word("Nlink")
	if err != nil {
		return err
	}
	modeUID, err := word("Mode")
	if err != nil {
		return err
	}
	group, err := word("Gid")
	if err != nil {
		return err
	}
	original := p.guard.Sys().(*syscall.Stat_t)
	if device != uint64(original.Dev) || inode != original.Ino || links != 1 || uint32(modeUID) != syscall.S_IFREG|0600 || uint32(modeUID>>32) != 0 || uint32(group) != 0 {
		return errors.New("actual returned pending guard stat differs from retained original")
	}
	if err := p.check(); err != nil {
		return err
	}
	p.observed = true
	if p.retirement {
		result.RefusalRetirementPendingObserved = true
		result.RefusalRetirementPendingDevice, result.RefusalRetirementPendingInode, result.RefusalRetirementPendingSHA256 = device, inode, p.guardSHA
	} else {
		result.RefusalStartupPendingObserved = true
		result.RefusalStartupPendingDevice, result.RefusalStartupPendingInode, result.RefusalStartupPendingSHA256 = device, inode, p.guardSHA
	}
	return nil
}

func (p *sourcePendingGuard) verify() error {
	if !p.observed || len(p.calls) != 0 {
		return errors.New("original pending guard Lstat success not observed before return")
	}
	return p.check()
}
