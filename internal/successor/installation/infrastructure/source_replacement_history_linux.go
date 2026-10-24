package infrastructure

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type sourceReplacementLink struct {
	selected, previous *sourceSelection
	provenance         sourceReplacementBinding
}

// Root alone reads the complete native lineage. Each archived generation keeps
// its own descriptors; no old source root/lease or Authorization is borrowed.
// A failed acquisition remains owned by sourceInitial until its joined Close.
func (s *sourceInitial) readReplacementLineage() error {
	current := s.selection
	var links []sourceReplacementLink
	seen := map[string]bool{current.binding.Generation: true}
	for current.replacement != nil {
		if len(links) >= int(sourceAttemptLimit)-1 {
			return ErrBinding
		}
		r := *current.replacement
		if seen[r.Configuration.Predecessor] {
			return ErrBinding
		}
		previous, err := openSourceArchivedSelection(s.ctx, current.binding.Request.InstallationRoot, r.Configuration.Predecessor)
		if previous != nil {
			s.lineage = append(s.lineage, previous)
		}
		if err != nil {
			return err
		}
		seen[previous.binding.Generation] = true
		if err := checkSourceReplacementNeighbor(previous, current); err != nil {
			return err
		}
		links = append(links, sourceReplacementLink{current, previous, r})
		current = previous
	}
	if current.binding.Generation != s.selection.replacement.InitialGeneration {
		return ErrBinding
	}
	if err := s.readInitialBirth(current); err != nil {
		return err
	}
	s.replacementJournals = map[string]bool{}
	var next sourceAttemptNumber
	var prior string
	for i := len(links) - 1; i >= 0; i-- {
		link := links[i]
		r := link.provenance
		if uint32(next) > r.PredecessorAttempt {
			return ErrBinding
		}
		commitment, err := s.history.selectedCommitment(s.verifier, link.previous.bindingBytes())
		if err != nil {
			return err
		}
		var retirement *sourceRetirement
		for ; uint32(next) <= r.PredecessorAttempt; next++ {
			started, err := readSourceStartHistory(link.previous, next, commitment, prior)
			if err != nil {
				return err
			}
			retirement, err = readSourceRetirement(link.previous, started)
			if err != nil {
				return err
			}
			prior = retirement.sha256
			s.previousStart, s.previousSelection = started, link.previous
		}
		if retirement == nil || retirement.sha256 != r.RetirementSHA256 {
			return ErrBinding
		}
		if err := s.readReplacementTranscript(link); err != nil {
			return err
		}
	}
	s.firstAttempt, s.priorRetirement = next, prior
	return s.checkReplacementJournals()
}

func checkSourceReplacementNeighbor(previous, current *sourceSelection) error {
	if previous == nil || current == nil || current.replacement == nil {
		return ErrInput
	}
	b, p := current.binding, previous.binding
	r := current.replacement
	if b.Predecessor != p.Generation || r.PredecessorBindingSHA256 != digest(previous.bindingBytes()) || b.Account != p.Account || b.UID != p.UID || b.GID != p.GID ||
		b.Request.InstallationRoot != p.Request.InstallationRoot || b.Request.StateRoot != p.Request.StateRoot || b.Request.ParticipationRoot != p.Request.ParticipationRoot ||
		b.Request.CredentialDirectory != p.Request.CredentialDirectory || b.Request.Clock != p.Request.Clock || b.Request.Profile != p.Request.Profile {
		return ErrBinding
	}
	if previous.replacement == nil {
		if r.InitialGeneration != p.Generation {
			return ErrBinding
		}
	} else if r.InitialGeneration != previous.replacement.InitialGeneration || r.PredecessorAttempt <= previous.replacement.PredecessorAttempt {
		return ErrBinding
	}
	get := func(s *sourceSelection, name string) []byte { return s.files[filepath.Join(s.sealed, name)].raw }
	old, err := DecodeDescriptor(get(previous, DescriptorName))
	if err != nil {
		return err
	}
	next, err := DecodeDescriptor(get(current, DescriptorName))
	if err != nil || next.ReleaseVersion <= old.ReleaseVersion || (b.ProgramSHA256 == p.ProgramSHA256 && b.Request.Address == p.Request.Address) {
		return errors.Join(ErrBinding, err)
	}
	if err := checkSourceForwardPlan(get(previous, "source-plan.json"), get(current, "source-plan.json")); err != nil {
		return err
	}
	for _, name := range []string{"certificate.pem", "private-key.pem", "client-ca.pem"} {
		path := filepath.Join(p.Request.CredentialDirectory, name)
		a, b := previous.files[path], current.files[path]
		if a == nil || b == nil || !os.SameFile(a.info, b.info) || !bytes.Equal(a.raw, b.raw) {
			return ErrBinding
		}
	}
	return nil
}

func (s *sourceInitial) readReplacementTranscript(link sourceReplacementLink) error {
	current, previous := link.selected, link.previous
	b := current.binding
	journal := sourceReplacementJournal(b.Request.InstallationRoot, b.Generation)
	raw, err := current.read(journal, 4<<20, 0600, 0, 0)
	if err != nil {
		return err
	}
	// Complete-looking bytes cannot bypass the still-live writer's original
	// lease. A shared read lock stays held until this reader physically closes.
	if s.transition != nil && s.transition.recovery != nil && s.transition.recovery.prefix.journal.path == journal {
		if !os.SameFile(s.transition.recovery.prefix.journal.info, current.files[journal].info) || !s.transition.recovery.finished {
			return ErrBinding
		}
		if err := s.transition.check(); err != nil {
			return err
		}
	} else if s.transition != nil && s.transition.journal != nil && s.transition.journal.path == journal {
		if !os.SameFile(s.transition.journal.info, current.files[journal].info) {
			return ErrBinding
		}
		if err := s.transition.journal.check(); err != nil {
			return err
		}
	} else if err := syscall.Flock(int(current.files[journal].file.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
		return err
	}
	admitted, err := s.history.selectedCommitment(s.verifier, current.bindingBytes())
	if err != nil {
		return err
	}
	hashes := map[string]string{}
	for path, file := range current.files {
		hashes[path] = digest(file.raw)
	}
	selected, unit := filepath.Join(b.Request.InstallationRoot, "binding.json"), "/etc/systemd/system/ardents-source.service"
	hashes[selected], hashes[unit] = digest(current.bindingBytes()), b.UnitSHA256
	recorded, err := decodeSourceReplacementRecords(raw, link.provenance, hashes, admitted, previous.binding.UnitSHA256)
	_, completionErr := os.Lstat(journal + ".recovery-completion")
	if err != nil || !errors.Is(completionErr, os.ErrNotExist) {
		transcript, e := s.readReplacementCompletion(current, journal)
		if e != nil {
			return e
		}
		recorded, err = decodeSourceReplacementRecords(transcript, link.provenance, hashes, admitted, previous.binding.UnitSHA256)
	}
	if err != nil {
		return err
	}
	for path, record := range recorded {
		info := current.identities[path]
		if info == nil {
			file := current.files[path]
			if path == selected || path == unit {
				file = s.selection.files[path]
			}
			if file == nil {
				file = previous.files[path]
			}
			if file == nil {
				return ErrBinding
			}
			info = file.info
		}
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok || uint64(native.Dev) != record.Device || native.Ino != record.Inode || native.Uid != record.UID || native.Gid != record.GID || uint32(info.Mode().Perm()) != record.Mode {
			return ErrBinding
		}
	}
	s.replacementJournals[journal] = true
	return nil
}

// Unknown partial journals, including a prefix before pending-file birth,
// refuse a subsequent operation. This never repairs or adopts that prefix.
func (s *sourceInitial) checkReplacementJournals() error {
	root := s.selection.binding.Request.InstallationRoot
	pending := filepath.Join(root, "source-replacement-pending")
	if _, err := os.Lstat(pending); !errors.Is(err, os.ErrNotExist) {
		var original *sourceBirth
		if s.transition != nil {
			for _, file := range s.transition.files {
				if file.path == pending {
					original = file
				}
			}
		}
		if original == nil {
			if s.recovery == nil {
				return errors.Join(ErrSourceRepairRequired, err)
			}
			if err := s.checkRecoveryPredecessorPending(); err != nil {
				return err
			}
		} else {
			if err := original.check(); err != nil {
				return err
			}
		}
	}
	if s.transition != nil {
		if err := s.transition.check(); err != nil {
			return err
		}
	}
	if s.recovery != nil {
		if err := s.checkRecoveryPredecessorPending(); err != nil {
			return err
		}
	}
	parent := s.selection.dirs[filepath.Dir(root)]
	if parent == nil {
		return ErrBinding
	}
	if _, err := parent.Seek(0, io.SeekStart); err != nil {
		return err
	}
	names, err := parent.Readdirnames(4097)
	if err != nil && err != io.EOF || len(names) > 4096 {
		return errors.Join(ErrBinding, err)
	}
	prefix := filepath.Base(root) + ".source-replacement-"
	seen := map[string]bool{}
	for _, name := range names {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		path := filepath.Join(filepath.Dir(root), name)
		known := s.replacementJournals[path]
		if s.recovery != nil && s.knownRecoveryPredecessorPath(path) {
			known = true
		}
		if s.transition != nil && s.transition.journal != nil && s.transition.journal.path == path {
			known = true
		}
		if !known {
			return ErrSourceRepairRequired
		}
		seen[path] = true
	}
	for path := range s.replacementJournals {
		if !seen[path] {
			return ErrBinding
		}
	}
	return s.ctx.Err()
}
