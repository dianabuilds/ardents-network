//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// This observes a real failed read of the exact missing original-cut control
// file. Absence, another ENOENT or a generic final error cannot prove the cause.
type sourceMissingCut struct {
	journal, path, pin string
	journalInfo        os.FileInfo
	parents            map[string]os.FileInfo
	pending            map[int]sourceMissingCutCall
	observed           bool
}

type sourceMissingCutCall struct {
	address, flags uint64
}

func sourceMissingCutNativeCase(scenario string) bool {
	return scenario == "refusal-missing-cut" || scenario == "refusal-missing-cut-unattempted" || scenario == "refusal-missing-cut-foreign" || scenario == "refusal-missing-cut-path-only"
}

func newSourceMissingCut(root, scenario string, profile *sourceRefusalObserverProfile) *sourceMissingCut {
	journal, pin := filepath.Join(root, "original-journal"), ""
	if profile != nil {
		if !profile.MissingOriginalCut {
			return nil
		}
		journal, pin = profile.Arguments[7], profile.JournalSHA256
	} else if !sourceMissingCutNativeCase(scenario) {
		return nil
	}
	return &sourceMissingCut{journal: journal, path: journal + ".original-cut-journal", pin: pin,
		parents: map[string]os.FileInfo{}, pending: map[int]sourceMissingCutCall{}}
}

func (p *sourceMissingCut) capture() error {
	raw, info, err := readSourceCutObserverFile(p.journal, 4<<20)
	if err != nil || p.pin != "" && sourcePhysicalSHA(raw) != p.pin {
		return errors.Join(errors.New("original missing-cut journal/pin required"), err)
	}
	p.pin, p.journalInfo = sourcePhysicalSHA(raw), info
	for path := filepath.Dir(p.path); ; path = filepath.Dir(path) {
		info, err := os.Lstat(path)
		if err != nil || !trustedDirectory(info) {
			return errors.Join(errors.New("original missing-cut parent custody required"), err)
		}
		p.parents[path] = info
		if path == "/" {
			break
		}
	}
	return p.check()
}

func (p *sourceMissingCut) check() error {
	if p.journalInfo == nil || len(p.parents) == 0 {
		return errors.New("original missing-cut observation not admitted")
	}
	for path, before := range p.parents {
		after, err := os.Lstat(path)
		if err != nil || !trustedDirectory(after) || !os.SameFile(before, after) || before.Mode() != after.Mode() {
			return errors.Join(errors.New("original missing-cut parent changed"), err)
		}
	}
	if _, err := os.Lstat(p.path); !errors.Is(err, os.ErrNotExist) {
		return errors.Join(errors.New("exact original-cut control file must remain absent"), err)
	}
	raw, info, err := readSourceCutObserverFile(p.journal, 4<<20)
	if err != nil || !sameSourceFile(p.journalInfo, info) || sourcePhysicalSHA(raw) != p.pin {
		return errors.Join(errors.New("original journal changed during missing-cut refusal"), err)
	}
	return nil
}

// Read only the actual syscall's bounded NUL-terminated path. Honor a NUL in
// a partial read at a mapping boundary rather than reading beyond that string.
func sourceMissingCutPath(tid int, address uint64) (string, error) {
	if address == 0 {
		return "", errors.New("actual missing-cut syscall path required")
	}
	raw := make([]byte, 0, 4096)
	var chunk [64]byte
	for offset := uint64(0); offset < 4096; offset += uint64(len(chunk)) {
		if address+offset < address {
			return "", errors.New("original syscall path address overflow")
		}
		n, err := syscall.PtracePeekData(tid, uintptr(address+offset), chunk[:])
		if n > 0 {
			if ending := bytes.IndexByte(chunk[:n], 0); ending >= 0 {
				return string(append(raw, chunk[:ending]...)), nil
			}
			raw = append(raw, chunk[:n]...)
		}
		if err != nil || n != len(chunk) {
			return "", errors.Join(errors.New("original syscall path could not be completely observed"), err)
		}
	}
	return "", errors.New("original syscall path exceeds finite bound")
}

func (p *sourceMissingCut) syscall(tid int, result *sourceCutObservation) error {
	var registers syscall.PtraceRegs
	if err := syscall.PtraceGetRegs(tid, &registers); err != nil {
		return err
	}
	if registers.Orig_rax != syscall.SYS_OPENAT {
		return nil
	}
	if int64(registers.Rax) == -int64(syscall.ENOSYS) {
		flags := registers.Rdx
		// Accept precisely the real file owner's read flags. In particular,
		// O_PATH performs only metadata lookup and cannot prove a read attempt.
		if flags != syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC {
			return nil
		}
		path, err := sourceMissingCutPath(tid, registers.Rsi)
		if err != nil {
			return err
		}
		if path != p.path {
			return nil
		}
		if _, exists := p.pending[tid]; exists {
			return errors.New("original missing-cut read repeated without paired exit")
		}
		if err := p.check(); err != nil {
			return err
		}
		p.pending[tid] = sourceMissingCutCall{address: registers.Rsi, flags: flags}
		return nil
	}
	call, exists := p.pending[tid]
	if !exists {
		return nil
	}
	delete(p.pending, tid)
	if int64(registers.Rax) != -int64(syscall.ENOENT) {
		return fmt.Errorf("actual original-cut read returned %d instead of ENOENT", int64(registers.Rax))
	}
	path, err := sourceMissingCutPath(tid, registers.Rsi)
	if err != nil || path != p.path || registers.Rsi != call.address || registers.Rdx != call.flags {
		return errors.Join(errors.New("original-cut syscall arguments changed across paired read"), err)
	}
	if err := p.check(); err != nil {
		return err
	}
	p.observed = true
	parent := p.parents[filepath.Dir(p.path)].Sys().(*syscall.Stat_t)
	result.RefusalMissingCutObserved = true
	result.RefusalMissingCutParentDevice, result.RefusalMissingCutParentInode = uint64(parent.Dev), parent.Ino
	return nil
}

func (p *sourceMissingCut) verify() error {
	if !p.observed || len(p.pending) != 0 {
		return errors.New("original-cut read ENOENT not observed before return")
	}
	return p.check()
}
