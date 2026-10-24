//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

func sameSourcePhysicalImage(before, after *sourcePhysicalImage) bool {
	if before == nil || after == nil {
		return false
	}
	same := func(a, b os.FileInfo) bool {
		if a == nil || b == nil {
			return a == nil && b == nil
		}
		return sameSourceFile(a, b)
	}
	return same(before.journalInfo, after.journalInfo) && same(before.payloadInfo, after.payloadInfo) && same(before.pendingInfo, after.pendingInfo) &&
		before.journalSHA == after.journalSHA && before.payloadSHA == after.payloadSHA && before.pendingSHA == after.pendingSHA
}

// Outside control retains a real entry until its parent has pinned the
// separately owned Source process/scope. A cut directive grants no domain
// completion: it only requests SIGKILL through the tracer's original pidfd.
type sourcePhysicalCutControl struct {
	tid     int
	command []byte
}

func (c *sourcePhysicalCutControl) ready(tid int, physical *sourcePhysicalCut, before *sourcePhysicalImage, result *sourceCutObservation) error {
	if c.tid != 0 || result.ExternalCutReady || !physical.root || !physical.killRoot || !physical.externalCut {
		return errors.New("one original held physical cut readiness required")
	}
	if physical.startup != nil {
		now, archive, err := physical.startup.image(true)
		if err != nil || !sameSourcePhysicalImage(before, now) {
			return errors.Join(errors.New("startup image changed before external retention"), err)
		}
		result.StartupArchive = archive
	}
	if physical.retirement != nil {
		now, err := physical.retirement.image()
		if err != nil || !sameSourcePhysicalImage(before, now.image) {
			return errors.Join(errors.New("retirement image changed before external original join observation"), err)
		}
		result.StartupArchive, result.RetirementResult = now.archive, now.result
	}
	c.tid, result.HeldRootThread, result.ExternalCutReady = tid, tid, true
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, "SOURCE_PHYSICAL_CUT_READY "+string(raw))
	return err
}

func (c *sourcePhysicalCutControl) poll(physical *sourcePhysicalCut, process *os.Process, journal, root string, before *sourcePhysicalImage, started time.Time, result *sourceCutObservation) error {
	if c.tid == 0 || result.ExternalCutRequested {
		return nil
	}
	if time.Since(started) >= 2*time.Minute {
		return errors.New("external cut exceeded original command selection bound")
	}
	var input [8]byte
	n, err := syscall.Read(int(os.Stdin.Fd()), input[:])
	if err != nil && err != syscall.EAGAIN && err != syscall.EWOULDBLOCK && err != syscall.EINTR {
		return err
	}
	if n == 0 && err == nil {
		return errors.New("external physical control closed before cut")
	}
	if n > 0 {
		c.command = append(c.command, input[:n]...)
		if !bytes.HasPrefix([]byte("cut\n"), c.command) {
			return errors.New("exact one external physical cut directive required")
		}
	}
	if string(c.command) != "cut\n" {
		return nil
	}
	var registers syscall.PtraceRegs
	if err := syscall.PtraceGetRegs(c.tid, &registers); err != nil || registers.Rip != result.Entry {
		return errors.Join(errors.New("original held physical entry moved"), err)
	}
	matched, now, err := physical.match(c.tid, &registers, journal, root)
	if err != nil || !matched || !sameSourcePhysicalImage(before, now) {
		return errors.Join(errors.New("original physical image changed before directed cut"), err)
	}
	if err := process.Kill(); err != nil {
		return err
	}
	result.ExternalCutRequested, result.RootKillRequested = true, true
	return nil
}
