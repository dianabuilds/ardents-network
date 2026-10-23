package infrastructure

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/completion"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

type sourceHistoryChain struct {
	initial *sourceInitial
	latest  *sourceStartHistory
	prior   string
}

type sourceRetirement struct {
	chain  *sourceHistoryChain
	result sourceRetirementResult
	sha256 string
}

func openSourceHistoryChain(ctx context.Context, v *release.Verifier, binding, history string) (result *sourceHistoryChain, err error) {
	chain := &sourceHistoryChain{}
	defer func() {
		if err != nil {
			err = errors.Join(err, chain.close())
			result = nil
		}
	}()
	chain.initial, err = openSourceInitial(ctx, v, binding, history)
	if err != nil {
		return nil, err
	}
	s := chain.initial.selection
	root := s.binding.Request.InstallationRoot
	// The original first journal remains immutable and supplies the one shared
	// native coordination lease for all later root stop/restart operations. An
	// independent descriptor excludes the original writer until it has closed.
	if _, err = s.read(sourceAttemptNumber(0).journal(root), 128<<10, 0600, 0, 0); err != nil {
		return nil, err
	}
	journal := s.files[sourceAttemptNumber(0).journal(root)]
	if err = syscall.Flock(int(journal.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, err
	}
	last, err := latestSourceAttempt(s)
	if err != nil {
		return nil, err
	}
	initialHistory, err := chain.initial.history.commitment(v, s.files[binding].raw)
	if err != nil {
		return nil, err
	}
	for n := sourceAttemptNumber(0); n <= last; n++ {
		chain.latest, err = readSourceStartHistory(s, n, initialHistory, chain.prior)
		if err != nil {
			return nil, err
		}
		if n != last {
			retired, e := readSourceRetirement(s, chain.latest)
			if e != nil {
				return nil, e
			}
			chain.prior = retired.sha256
		}
	}
	return chain, chain.initial.check()
}

func openSourceRetirement(ctx context.Context, v *release.Verifier, binding, history string) (retired *sourceRetirement, err error) {
	chain, err := openSourceHistoryChain(ctx, v, binding, history)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, chain.close())
			retired = nil
		}
	}()
	retired, err = readSourceRetirement(chain.initial.selection, chain.latest)
	if err != nil {
		return nil, err
	}
	retired.chain = chain
	return retired, retired.check()
}

func readSourceRetirement(s *sourceSelection, started *sourceStartHistory) (*sourceRetirement, error) {
	if s == nil || started == nil {
		return nil, ErrInput
	}
	blocker := filepath.Join(started.number.directory(s.binding.Request.InstallationRoot), "source-retirement-pending")
	if _, err := os.Lstat(blocker); !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Join(ErrBinding, err)
	}
	retired, err := readSourceRetirementProof(s, started)
	if err != nil {
		return nil, err
	}
	if _, err = os.Lstat(blocker); !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Join(ErrBinding, err)
	}
	return retired, nil
}

// Proof readback is also consumed by the still-blocked producer before its
// final removal. It supplies no committed retirement or live start right.
func readSourceRetirementProof(s *sourceSelection, started *sourceStartHistory) (*sourceRetirement, error) {
	if s == nil || started == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	b := s.binding
	directory := started.number.directory(b.Request.InstallationRoot)
	path := filepath.Join(directory, "source-retirement.json")
	raw, err := s.readPinned(path, 64<<10, 0600, 0, 0)
	if err != nil {
		return nil, err
	}
	r := &sourceRetirement{sha256: digest(raw)}
	if !sourceCanonicalRecord(raw, &r.result) {
		return nil, ErrBinding
	}
	f := r.result
	a := started.archive
	binding := digest(s.files[filepath.Join(b.Request.InstallationRoot, "binding.json")].raw)
	if f.Schema != "ardents-source-retirement-v1" || f.Attempt != uint32(started.number) || f.Generation != b.Generation || f.BindingSHA256 != binding || f.StartSHA256 != started.sha256 || f.InvocationID != a.InvocationID || f.PID != a.PID || f.Manager != a.Manager || f.StartedRealtime != a.StartedRealtime || f.StartedMonotonic != a.StartedMonotonic || f.ExitedRealtime < f.StartedRealtime || f.ExitedMonotonic < f.StartedMonotonic || f.Code != 1 || f.Status != 0 {
		return nil, ErrBinding
	}
	challenge, err := sourceRetirementChallenge(f.Challenge)
	if err != nil {
		return nil, err
	}
	start, err := completion.Encode(started.id, b.Generation, binding)
	if err != nil {
		return nil, err
	}
	frame := sourceRetirementFrame(start, challenge)
	if f.CompletionSHA256 != digest(frame[:]) {
		return nil, ErrBinding
	}
	journalPath := filepath.Join(directory, "source-retirement-journal")
	journal, err := s.readPinned(journalPath, 128<<10, 0600, 0, 0)
	if err != nil {
		return nil, err
	}
	owned, err := decodeSourceRetirementRecords(journal, directory, started.sha256, digest(challenge[:]), digest(frame[:]), digest(raw), b.GID)
	if err != nil {
		return nil, err
	}
	for path, record := range owned {
		var info os.FileInfo
		if path == filepath.Join(directory, "stop-completion.socket") {
			info, err = os.Lstat(path)
			if err != nil || !sameSourceSocket(info, info, 0660, b.GID) {
				return nil, errors.Join(ErrBinding, err)
			}
		} else {
			if s.files[path] == nil {
				return nil, ErrBinding
			}
			info = s.files[path].info
		}
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok || uint64(native.Dev) != record.Device || native.Ino != record.Inode || native.Uid != record.UID || native.Gid != record.GID || uint32(info.Mode().Perm()) != record.Mode {
			return nil, ErrBinding
		}
	}
	return r, s.observe()
}

func (r *sourceRetirement) check() error {
	if r == nil || r.chain == nil || r.chain.latest == nil || !sourceInitialSHA(r.sha256) {
		return ErrBinding
	}
	if err := r.chain.initial.check(); err != nil {
		return err
	}
	s := r.chain.initial.selection
	blocker := filepath.Join(r.chain.latest.number.directory(s.binding.Request.InstallationRoot), "source-retirement-pending")
	if _, err := os.Lstat(blocker); !errors.Is(err, os.ErrNotExist) {
		return errors.Join(ErrBinding, err)
	}
	return nil
}

func (r *sourceRetirement) stopped(facts systemd.SourceObservation) error {
	if err := r.check(); err != nil {
		return err
	}
	s := r.chain.initial.selection
	if facts.Manager != r.result.Manager {
		return ErrBinding
	}
	// A collected, newly loaded inactive object has no execution facts. The
	// original complete durable retirement remains mandatory in either case.
	if verifySourceStopped(facts.Unit, facts.Service, s.binding.Request, s.binding.Generation) == nil {
		return nil
	}
	return verifySourceCompletedStop(facts.Unit, facts.Service, s.binding.Request, s.binding.Generation, r.chain.latest, r.result.execution())
}

func (c *sourceHistoryChain) close() error {
	if c == nil {
		return nil
	}
	return c.initial.close()
}
func (r *sourceRetirement) close() error {
	if r == nil {
		return nil
	}
	return r.chain.close()
}
