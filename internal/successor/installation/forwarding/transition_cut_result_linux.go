package forwarding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"syscall"
)

type transitionCutResult struct {
	Schema        string            `json:"schema"`
	Boundary      transitionMessage `json:"boundary"`
	Origin        transitionOrigin  `json:"origin"`
	ControlDevice uint64            `json:"control_device"`
	ControlInode  uint64            `json:"control_inode"`
	Acknowledged  uint32            `json:"acknowledged"`
	Code          int               `json:"code"`
	Status        int               `json:"status"`
	Reaped        int               `json:"reaped"`
}

func transitionCutPaths(journal string) (string, string, string) {
	return journal + ".original-cut-journal", journal + ".original-cut-result", journal + ".original-cut-pending"
}

func transitionCutPending(message transitionMessage, origin transitionOrigin) []byte {
	raw, _ := json.Marshal(struct {
		Boundary transitionMessage `json:"boundary"`
		Origin   transitionOrigin  `json:"origin"`
	}{message, origin})
	return []byte("ardents-forwarding-original-cut-pending-v1\n" + digest(raw) + "\n")
}

// Historical original-process facts only. Recovery independently needs fresh
// Release/native admission, the absent original-cut guard and its own original
// exclusive lease. This byte decoder cannot grant an effect or domain join.
func decodeTransitionCut(control, raw, prefix []byte, controlInfo, resultInfo, originalInfo os.FileInfo) (transitionCutResult, error) {
	var result transitionCutResult
	if len(raw) == 0 || len(raw) > 256<<10 || json.Unmarshal(raw, &result) != nil {
		return result, ErrBinding
	}
	canonical, err := json.Marshal(result)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || !nodeAccess(controlInfo, 0600, 0, 0, false) || !nodeAccess(resultInfo, 0600, 0, 0, false) || !nodeAccess(originalInfo, 0600, 0, 0, false) || controlInfo.Size() != int64(len(control)) || resultInfo.Size() != int64(len(raw)) || originalInfo.Size() != int64(len(prefix)) {
		return result, errors.Join(ErrBinding, err)
	}
	c, _ := infoSys(controlInfo)
	r, _ := infoSys(resultInfo)
	o, _ := infoSys(originalInfo)
	b, p := result.Boundary, result.Origin.Process
	if result.Schema != "ardents-forwarding-original-cut-v1" || result.ControlDevice != uint64(c.Dev) || result.ControlInode != c.Ino || b.Sequence == 0 || result.Acknowledged != b.Sequence-1 || result.Code != 2 || result.Status != int(syscall.SIGKILL) || result.Reaped != 1 ||
		!transitionBoundarySchema(b) || !validStartDigest(b.Nonce) || b.Deadline != 0 || b.Records != b.Sequence || b.Records > transitionRecordLimit(b.Operation) || !validPath(b.Journal) || b.Device != uint64(o.Dev) || b.Inode != o.Ino || b.PrefixSHA256 != digest(prefix) || !validStartDigest(b.Generation) || !validStartDigest(b.BindingSHA256) || digest(b.Binding) != b.BindingSHA256 || len(b.NativeContext) == 0 || len(b.NativeContext) > 64<<10 || b.Manager == "" || len(b.Manager) > 288 ||
		!validStartDigest(result.Origin.ProgramSHA256) || result.Origin.ProgramDevice == 0 || result.Origin.ProgramInode == 0 || p.PID <= 1 || p.Parent <= 1 || p.PID == p.Parent || p.Device == 0 || p.Inode == 0 || p.Started == 0 {
		return result, ErrBinding
	}
	selection, err := decodeSelection(b.Binding)
	if err != nil || selection.binding.Generation != b.Generation {
		return result, errors.Join(ErrBinding, err)
	}
	before, err := decodeTransitionPrefix(prefix, b.Operation)
	if err != nil || uint32(len(before)) != b.Records || before[0].Path != b.Journal || before[0].Device != b.Device || before[0].Inode != b.Inode {
		return result, errors.Join(ErrBinding, err)
	}
	if err := checkTransitionContexts(b, before); err != nil {
		return result, err
	}
	if err := checkTransitionStartIntent(b, before); err != nil {
		return result, err
	}
	if len(control) == 0 || len(control) > 64<<10 || control[len(control)-1] != '\n' {
		return result, ErrBinding
	}
	lines := bytes.Split(control[:len(control)-1], []byte{'\n'})
	if len(lines) != 13 {
		return result, ErrBinding
	}
	records := make([]initialRecord, len(lines))
	for i, line := range lines {
		if json.Unmarshal(line, &records[i]) != nil {
			return result, ErrBinding
		}
		canonical, _ := json.Marshal(records[i])
		if !bytes.Equal(line, canonical) || records[i].Sequence != i {
			return result, ErrBinding
		}
	}
	controlPath, resultPath, pending := transitionCutPaths(b.Journal)
	pidPath := "/proc/" + strconv.FormatUint(uint64(p.PID), 10)
	boundary, _ := encodeTransitionMessage(b)
	sha, pendingSHA := digest(raw), digest(transitionCutPending(b, result.Origin))
	wanted := []initialRecord{
		{Phase: "forwarding-cut-v1", Path: controlPath, Device: uint64(c.Dev), Inode: c.Ino, Mode: 0600},
		{Phase: "file-intent", Path: pending, SHA256: pendingSHA, Mode: 0600},
		{Phase: "file-born", Path: pending, Mode: 0600},
		{Phase: "file-ready", Path: pending, SHA256: pendingSHA, Mode: 0600},
		{Phase: "coordinator-born-observed", Path: pidPath, Device: p.Device, Inode: p.Inode, SHA256: sha},
		{Phase: "boundary-held", Path: b.Journal, Device: b.Device, Inode: b.Inode, SHA256: digest(boundary)},
		{Phase: "next-effect-denied", Path: b.Journal, Device: b.Device, Inode: b.Inode, SHA256: digest(boundary)},
		{Phase: "native-joined", Path: pidPath, Device: p.Device, Inode: p.Inode, SHA256: sha},
		{Phase: "physical-closed", Path: b.Journal, Device: b.Device, Inode: b.Inode, SHA256: sha},
		{Phase: "file-intent", Path: resultPath, SHA256: sha, Mode: 0600},
		{Phase: "file-born", Path: resultPath, Device: uint64(r.Dev), Inode: r.Ino, Mode: 0600},
		{Phase: "file-ready", Path: resultPath, Device: uint64(r.Dev), Inode: r.Ino, SHA256: sha, Mode: 0600},
		{Phase: "original-cut-completed", Path: resultPath, Device: uint64(r.Dev), Inode: r.Ino, SHA256: sha, Mode: 0600},
	}
	if records[2].Device == 0 || records[2].Inode == 0 || records[2].Device != records[3].Device || records[2].Inode != records[3].Inode {
		return result, ErrBinding
	}
	wanted[2].Device, wanted[2].Inode = records[2].Device, records[2].Inode
	wanted[3].Device, wanted[3].Inode = records[3].Device, records[3].Inode
	for i := range records {
		wanted[i].Sequence = i
		if records[i] != wanted[i] {
			return result, ErrBinding
		}
	}
	return result, nil
}

// Scope join records kernel/process completion only, never a Node domain join.
// Interrupted original operations retain every original blocker. The distinct
// proof's guard is removed only after all its own writers and readers close.
func publishTransitionCut(ctx context.Context, prefix *transitionPrefix, scope *transitionScope, origin transitionOrigin) (returned error) {
	if ctx == nil || prefix == nil || !prefix.exclusive || scope == nil || !scope.joined || scope.terminal != nil || !scope.status.Signaled() || scope.status.Signal() != syscall.SIGKILL || scope.reaped != 1 || scope.birth != origin.Process || scope.processInfo == nil {
		return errors.Join(ErrInput, prefix.close())
	}
	defer func() { returned = errors.Join(returned, prefix.close()) }()
	check := func() error { return errors.Join(ctx.Err(), prefix.check()) }
	if err := check(); err != nil {
		return err
	}
	controlPath, resultPath, pendingPath := transitionCutPaths(prefix.message.Journal)
	for _, path := range []string{controlPath, resultPath, pendingPath} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrBinding, err)
		}
	}
	parent := prefix.parent()
	if parent == nil {
		return ErrBinding
	}
	pidPath := "/proc/" + strconv.Itoa(scope.pid)
	writer, err := openForwardingBirths(ctx, controlPath, map[string]bool{resultPath: true, pendingPath: true, prefix.message.Journal: true, pidPath: true}, "forwarding-cut-v1")
	defer func() { returned = errors.Join(returned, writer.close()) }()
	if err != nil {
		return err
	}
	writer.beforeClose = prefix.beforeProofClose
	native, _ := infoSys(writer.journal.info)
	result := transitionCutResult{"ardents-forwarding-original-cut-v1", prefix.message, origin, uint64(native.Dev), native.Ino, prefix.message.Sequence - 1, 2, int(syscall.SIGKILL), scope.reaped}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := writer.write(pendingPath, transitionCutPending(prefix.message, origin), 0600, 0, 0, check); err != nil {
		return err
	}
	pending := writer.files[len(writer.files)-1]
	boundary, err := encodeTransitionMessage(prefix.message)
	if err != nil {
		return err
	}
	for _, row := range []struct {
		phase, path string
		info        os.FileInfo
		sha         string
	}{
		{"coordinator-born-observed", pidPath, scope.processInfo, digest(raw)},
		{"boundary-held", prefix.message.Journal, prefix.journal.info, digest(boundary)},
		{"next-effect-denied", prefix.message.Journal, prefix.journal.info, digest(boundary)},
		{"native-joined", pidPath, scope.processInfo, digest(raw)},
		{"physical-closed", prefix.message.Journal, prefix.journal.info, digest(raw)},
	} {
		if err := check(); err != nil {
			return err
		}
		if err := writer.record(row.phase, row.path, row.info, row.sha, 0, 0, 0); err != nil {
			return err
		}
	}
	if err := writer.write(resultPath, raw, 0600, 0, 0, check); err != nil {
		return err
	}
	resultFile := writer.files[len(writer.files)-1]
	if err := writer.record("original-cut-completed", resultPath, resultFile.info, digest(raw), 0600, 0, 0); err != nil {
		return err
	}
	if _, err := decodeTransitionCut(writer.journal.body, raw, prefix.journal.body, writer.journal.info, resultFile.info, prefix.journal.info); err != nil {
		return err
	}
	if err := errors.Join(check(), writer.check()); err != nil {
		return err
	}
	if err := errors.Join(writer.close(), prefix.close(), ctx.Err()); err != nil {
		return err
	}
	independent, err := readOriginalCutBack(ctx, prefix.message.Journal, pending)
	if err != nil {
		return errors.Join(err, independent.close())
	}
	if err := errors.Join(independent.check(), independent.close(), ctx.Err()); err != nil {
		return err
	}
	return publishInitialPending(writer, pending, parent)
}
