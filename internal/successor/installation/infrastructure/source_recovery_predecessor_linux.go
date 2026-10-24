package infrastructure

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Owns these newly opened read-only resources on every outcome. The fresh
// recovery owner retains its distinct original-prefix lease and complete cut
// proof. This never constructs an old writer or substitutes a retirement.
func openSourceRecoveryPredecessor(v *release.Verifier, old *sourceSelection, history *sourceHistory, prefix *sourceRecoveryPrefix, candidate *sourceReplacementCandidate) (result *sourceRetirement, err error) {
	if old == nil || history == nil || prefix == nil || prefix.cut == nil || candidate == nil || v == nil {
		return nil, errors.Join(ErrInput, old.close(), history.close())
	}
	s := &sourceInitial{ctx: old.ctx, verifier: v, selection: old, history: history, recovery: prefix}
	transferred := false
	defer func() {
		if !transferred {
			err = errors.Join(err, s.close())
		}
	}()
	b := candidate.binding.Configuration
	root := old.binding.Request.InstallationRoot
	if b.Request.InstallationRoot != root || prefix.operation != "replacement" || prefix.journal.path != sourceReplacementJournal(root, b.Generation) || prefix.cut.Boundary.Generation != b.Generation || prefix.cut.Boundary.BindingSHA256 != digest(candidate.raw) {
		return nil, ErrBinding
	}
	admitted, err := history.selectedCommitment(v, candidate.raw)
	if err != nil {
		return nil, err
	}
	bodies := sourceReplacementRecoveryBodies(candidate)
	hashes := map[string]string{}
	for path, raw := range bodies {
		hashes[path] = digest(raw)
	}
	if err = validateSourceReplacementPrefix(prefix.journal.raw, candidate.binding, hashes, admitted, old.binding.UnitSHA256); err != nil {
		return nil, err
	}
	count := len(prefix.records)
	if count < 11 || count > 55 || prefix.records[count-1].Phase == "file-intent" {
		return nil, ErrSourceRepairRequired
	}
	selected, unit := filepath.Join(root, "binding.json"), "/etc/systemd/system/ardents-source.service"
	for _, row := range []struct {
		path          string
		before, after []byte
		written       int
		record        sourceRecord
	}{
		{selected, old.bindingBytes(), candidate.raw, 50, prefix.records[2]},
		{unit, old.files[filepath.Join(old.sealed, "rendered-source.service")].raw, candidate.unit, 52, prefix.records[3]},
	} {
		wanted := row.before
		if count > row.written {
			wanted = row.after
		}
		raw, e := old.read(row.path, 32<<10, 0644, 0, 0)
		if e != nil || !bytes.Equal(raw, wanted) {
			return nil, errors.Join(ErrBinding, e)
		}
		info := old.files[row.path].info.Sys().(*syscall.Stat_t)
		if row.record.Device != uint64(info.Dev) || row.record.Inode != info.Ino {
			return nil, ErrBinding
		}
	}
	pending := filepath.Join(root, "source-replacement-pending")
	if _, err = old.read(pending, 1024, 0600, 0, 0); err != nil {
		return nil, err
	}
	s.recoveryPending = old.files[pending]
	if err = s.checkRecoveryPredecessorPending(); err != nil {
		return nil, err
	}
	if old.replacement != nil {
		err = s.readReplacementLineage()
	} else {
		err = s.readInitialBirth(old)
	}
	if err != nil {
		return nil, err
	}
	if err = s.check(); err != nil {
		return nil, err
	}
	transferred = true
	chain, err := readSourceHistoryChain(s)
	if err != nil {
		return nil, err
	}
	retired, err := readSourceRetirementChain(chain)
	if err != nil {
		return nil, err
	}
	if retired.sha256 != candidate.binding.RetirementSHA256 || retired.chain.latest.number != sourceAttemptNumber(candidate.binding.PredecessorAttempt) || retired.chain.latestSelection != retired.chain.initial.selection {
		return nil, errors.Join(ErrBinding, retired.close())
	}
	return retired, nil
}

func (s *sourceInitial) checkRecoveryPredecessorPending() error {
	if s.recovery == nil || s.recovery.cut == nil || s.recoveryPending == nil || len(s.recovery.records) < 8 {
		return ErrBinding
	}
	f := s.recoveryPending
	r := s.recovery.records[7]
	if r.Phase != "file-ready" || r.Path != f.path || !bytes.Equal(f.raw, sourceReplacementPending(s.recovery.cut.Boundary.BindingSHA256)) || digest(f.raw) != r.SHA256 || !sourceAccess(f.info, 0600, 0, 0, false) {
		return ErrBinding
	}
	native := f.info.Sys().(*syscall.Stat_t)
	named, err := os.Lstat(f.path)
	if err != nil || !sameSourceFile(f.info, named) || r.Device != uint64(native.Dev) || r.Inode != native.Ino {
		return errors.Join(ErrBinding, err)
	}
	return s.recovery.check()
}

func (s *sourceInitial) knownRecoveryPredecessorPath(path string) bool {
	p := s.recovery
	if p == nil || p.cut == nil {
		return false
	}
	if path == p.journal.path || p.cutControl != nil && path == p.cutControl.path || p.cutResult != nil && path == p.cutResult.path {
		return true
	}
	return s.recoveryWriter != nil && s.recoveryWriter.recovery != nil && s.recoveryWriter.recovery.prefix == p && s.recoveryWriter.journal != nil && path == s.recoveryWriter.journal.path
}

func sourceReplacementRecoveryBodies(candidate *sourceReplacementCandidate) map[string][]byte {
	b := candidate.binding.Configuration
	root := b.Request.InstallationRoot
	bodies := map[string][]byte{
		filepath.Join(root, "binding.json"):               candidate.raw,
		"/etc/systemd/system/ardents-source.service":      candidate.unit,
		filepath.Join(root, "source-replacement-pending"): sourceReplacementPending(digest(candidate.raw)),
	}
	for name, body := range candidate.files {
		bodies[filepath.Join(root, "generations", b.Generation, name)] = body
	}
	return bodies
}
