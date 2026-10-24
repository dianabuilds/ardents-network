package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// Called only after the genuine original controller's callback, process scope,
// packet channel and retained program physically closed successfully. The
// original interrupted caller still returns its failure. Publication records
// historical observed/denied/joined facts, not a new Source accepting result.
func publishSourceCutResult(ctx context.Context, cut *sourceTransitionCut, scope *sourceTransitionScope, origin sourceTransitionOrigin) (err error) {
	if ctx == nil || cut == nil || cut.prefix == nil || !cut.prefix.exclusive || scope == nil || !scope.joined || scope.terminal != nil || !scope.status.Signaled() || scope.status.Signal() != syscall.SIGKILL || scope.reaped != 1 || scope.birth != origin.Process || scope.processInfo == nil {
		return errors.Join(ErrInput, cut.close())
	}
	defer func() { err = errors.Join(err, cut.close()) }()
	admit := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := cut.check(); err != nil {
			return err
		}
		manager, err := sourceManagerIdentity()
		if err != nil || manager != origin.NativeManager {
			return errors.Join(ErrBinding, err)
		}
		return nil
	}
	if err = admit(); err != nil {
		return err
	}
	controlPath, resultPath, pendingPath := sourceCutPaths(cut.message.Journal)
	for _, path := range []string{controlPath, resultPath, pendingPath} {
		if _, e := os.Lstat(path); !errors.Is(e, os.ErrNotExist) {
			return errors.Join(ErrBinding, e)
		}
	}
	parent := cut.prefix.identities[filepath.Dir(pendingPath)]
	if parent == nil {
		return ErrBinding
	}
	writer, err := openSourceBirthsWithPhase(ctx, controlPath, map[string]bool{resultPath: true, pendingPath: true}, "source-cut")
	defer func() { err = errors.Join(err, writer.close()) }()
	if err != nil {
		return err
	}
	writer.afterSync = cut.afterSync
	c := writer.journal.info.Sys().(*syscall.Stat_t)
	result := sourceCutResult{Schema: "ardents-source-original-cut-v1", Boundary: cut.message, Origin: origin, ControlDevice: uint64(c.Dev), ControlInode: c.Ino, Acknowledged: cut.message.Sequence - 1, Code: 2, Status: int(syscall.SIGKILL), Reaped: scope.reaped}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err = writer.write(pendingPath, sourceCutPending(cut.message, origin), 0600, 0, 0, admit); err != nil {
		return err
	}
	pending := writer.files[len(writer.files)-1]
	pidPath := "/proc/" + strconv.Itoa(scope.pid)
	boundaryRaw, _ := json.Marshal(cut.message)
	for _, row := range []struct {
		phase, path string
		info        os.FileInfo
		sha         string
	}{
		{"coordinator-born-observed", pidPath, scope.processInfo, digest(raw)},
		{"boundary-held", cut.message.Journal, cut.prefix.journal.info, digest(boundaryRaw)},
		{"next-effect-denied", cut.message.Journal, cut.prefix.journal.info, digest(boundaryRaw)},
		{"native-joined", pidPath, scope.processInfo, digest(raw)},
		{"physical-closed", cut.message.Journal, cut.prefix.journal.info, digest(raw)},
	} {
		if err = admit(); err != nil {
			return err
		}
		if err = writer.record(row.phase, row.path, row.info, row.sha, 0, 0, 0); err != nil {
			return err
		}
	}
	if err = writer.write(resultPath, raw, 0600, 0, 0, admit); err != nil {
		return err
	}
	resultFile := writer.files[len(writer.files)-1]
	if err = writer.record("original-cut-completed", resultPath, resultFile.info, digest(raw), 0600, 0, 0); err != nil {
		return err
	}
	if err = admit(); err != nil {
		return err
	}
	if err = writer.check(); err != nil {
		return err
	}
	if _, err = decodeSourceCutResult(writer.journal.body, raw, cut.prefix.journal.raw, writer.journal.info, resultFile.info, cut.prefix.journal.info, cut.message.Journal, cut.message.Operation, cut.message.Generation, cut.message.BindingSHA256, origin.ProgramSHA256); err != nil {
		return err
	}
	// Complete-looking bytes remain blocked until every required writer and
	// original-prefix observer Close is known. No sync follows the last unlink.
	if err = errors.Join(writer.close(), cut.close(), ctx.Err()); err != nil {
		return err
	}
	namedParent, e := os.Lstat(filepath.Dir(pendingPath))
	named, ne := os.Lstat(pendingPath)
	if e != nil || ne != nil || !os.SameFile(parent, namedParent) || !sourceAccess(namedParent, parent.Mode().Perm(), 0, 0, true) || !sameSourceFile(pending.info, named) {
		return errors.Join(ErrBinding, e, ne)
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return os.Remove(pendingPath)
}
