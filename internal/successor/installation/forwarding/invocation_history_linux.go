package forwarding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// One parent observation owns the child leases. A Started view can check them
// but never closes them separately or makes them close their own parent.
type invocationChain struct {
	ctx              context.Context
	owner            *completedInitial
	snapshot         *startSnapshot
	starts           []*startSnapshot
	retirements      []*retirementSnapshot
	latestRetirement *retirementSnapshot
	entries          []*invocationEntry
	terminals        []*startTerminalSnapshot
	ioMu             sync.Mutex
	closed           bool
	terminal         error
}

type invocationEntry struct {
	number     attemptNumber
	selection  generationSelection
	start      *startSnapshot
	terminal   *startTerminalSnapshot
	retirement *retirementSnapshot
}

func (s *invocationChain) head() *invocationEntry {
	if s == nil || len(s.entries) == 0 {
		return nil
	}
	return s.entries[len(s.entries)-1]
}
func (e *invocationEntry) predecessor() (string, error) {
	if e == nil {
		return "", ErrBinding
	}
	if e.terminal != nil && e.terminal.sha256 != "" {
		return e.terminal.sha256, nil
	}
	if e.retirement != nil {
		return e.retirement.sha256, nil
	}
	return "", ErrBinding
}

func (s *invocationChain) checkEmpty() error {
	root := s.owner.binding.Request.InstallationRoot
	entries, err := historyDirectoryEntries(s.owner.files.dirs[root])
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "forwarding-startup" || strings.HasPrefix(name, "forwarding-invocation-") || strings.HasPrefix(name, "forwarding-retirement-") {
			return ErrBinding
		}
	}
	for n := attemptNumber(0); n < attemptLimit; n++ {
		for _, path := range []string{n.journal(root), n.retirementJournal(root)} {
			if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				return errors.Join(ErrBinding, err)
			}
		}
	}
	return s.ctx.Err()
}

// Parent Check serializes these physical observations with Started's chain
// checks. Namespace admission for an actual writer stays with that writer.
func (s *invocationChain) checkFiles() error {
	if s == nil {
		return nil
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	if s.closed {
		return errors.Join(ErrInput, s.terminal)
	}
	for _, h := range s.starts {
		if err := h.check(); err != nil {
			return err
		}
	}
	for _, h := range s.retirements {
		if err := h.check(); err != nil {
			return err
		}
	}
	for _, h := range s.terminals {
		if err := h.check(); err != nil {
			return err
		}
	}
	return s.ctx.Err()
}

func (s *invocationChain) close() error {
	if s == nil {
		return nil
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	if !s.closed {
		s.closed = true
		for n := len(s.retirements) - 1; n >= 0; n-- {
			s.terminal = errors.Join(s.terminal, s.retirements[n].close())
		}
		for n := len(s.starts) - 1; n >= 0; n-- {
			s.terminal = errors.Join(s.terminal, s.starts[n].close())
		}
		for n := len(s.terminals) - 1; n >= 0; n-- {
			s.terminal = errors.Join(s.terminal, s.terminals[n].close())
		}
		s.terminal = errors.Join(s.terminal, s.ctx.Err())
	}
	return s.terminal
}

// The initial and every start/retirement lease remain independently held. No
// missing predecessor, partial suffix or recycled InvocationID can be skipped.
func (s *invocationChain) readChain(bindingPath string) error {
	c := s.owner
	root := c.binding.Request.InstallationRoot
	entries, err := historyDirectoryEntries(c.files.dirs[root])
	if err != nil {
		return err
	}
	var names []string
	for _, entry := range entries {
		if entry.Name() == "forwarding-startup" || strings.HasPrefix(entry.Name(), "forwarding-invocation-") {
			if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				return ErrBinding
			}
			names = append(names, entry.Name())
		}
	}
	if original := c.files.interrupted; original != nil && original.cut.Boundary.Operation == "start" {
		number, err := startJournalNumber(root, original.cut.Boundary.Journal)
		if err != nil {
			return err
		}
		name := filepath.Base(number.directory(root))
		present := false
		for _, existing := range names {
			if existing == name {
				present = true
			}
		}
		if !present {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return s.checkEmpty()
	}
	latest, err := attemptDirectories(names)
	if err != nil {
		return err
	}
	initial := c.initialHistorySHA()
	predecessor := ""
	seen := map[[16]byte]bool{}
	retired := map[attemptNumber]*retirementSnapshot{}
	previousBinding := ""
	hadRuntime := false
	for n := attemptNumber(0); n <= latest; n++ {
		selected, err := s.selectionForAttempt(n)
		if err != nil {
			return err
		}
		bind := digest(selected.raw)
		if n == 0 && bind != digest(c.initial.raw) {
			return ErrBinding
		}
		if n != 0 && bind != previousBinding {
			r := selected.replacement
			if r == nil || s.snapshot == nil || s.latestRetirement == nil || r.PredecessorAttempt != uint32(s.snapshot.number) || r.PredecessorBindingSHA256 != previousBinding || r.PredecessorBindingSHA256 != s.snapshot.bindingSHA || r.RetirementSHA256 != s.latestRetirement.sha256 {
				return ErrBinding
			}
		}
		recovered, err := startRecoveryPresent(n.journal(root))
		if err != nil {
			return err
		}
		if recovered {
			h, err := readStartTerminal(s.ctx, c, selected, n, initial, predecessor, hadRuntime)
			if h != nil {
				s.terminals = append(s.terminals, h)
			}
			if err != nil {
				return err
			}
			s.entries = append(s.entries, &invocationEntry{number: n, selection: selected, terminal: h})
			predecessor = h.sha256
			previousBinding = bind
			continue
		}
		h, err := readStartSnapshotAt(s.ctx, selected.binding, n, bind, initial, predecessor)
		if h != nil {
			s.starts = append(s.starts, h)
		}
		if err != nil {
			return err
		}
		if h.extendedACK {
			expected := uint8(0)
			if hadRuntime {
				expected = 1
			}
			if h.historyOpening != expected {
				return ErrBinding
			}
		}
		hadRuntime = true
		if seen[h.id] {
			return ErrBinding
		}
		seen[h.id] = true
		s.snapshot = h
		s.latestRetirement = nil
		entry := &invocationEntry{number: n, selection: selected, start: h}
		s.entries = append(s.entries, entry)
		needsRetirement := n < latest
		if !needsRetirement {
			for _, path := range []string{n.retirement(root), n.retirementJournal(root)} {
				if _, err := os.Lstat(path); err == nil {
					needsRetirement = true
				} else if !errors.Is(err, os.ErrNotExist) {
					return err
				}
			}
		}
		if needsRetirement {
			r, err := readRetirementSnapshotContext(s.ctx, selected.binding, n, bind, h.sha256, h.archive, h.id, retirementCommitted, c)
			if r != nil {
				s.retirements = append(s.retirements, r)
			}
			if err != nil {
				return err
			}
			predecessor = r.sha256
			retired[n] = r
			entry.retirement = r
			s.latestRetirement = r
		}
		previousBinding = bind
	}
	for _, selected := range c.selectionOrder {
		r := selected.replacement
		if r == nil {
			continue
		}
		n := attemptNumber(r.PredecessorAttempt)
		retirement := retired[n]
		if n > latest || retirement == nil || s.entries[n].start == nil || s.entries[n].start.bindingSHA != r.PredecessorBindingSHA256 || retirement.sha256 != r.RetirementSHA256 {
			return ErrBinding
		}
	}
	return s.checkNamespace(nil, nil)
}

// This bounded header selects only a known immutable binding. The independent
// snapshot reader subsequently verifies the entire exact guard/transcript and
// its original inodes; a header alone never grants historical Start or join.
func (s *invocationChain) selectionForAttempt(number attemptNumber) (generationSelection, error) {
	c := s.owner
	journal := number.journal(c.binding.Request.InstallationRoot)
	recovered, err := startRecoveryPresent(journal)
	if err != nil {
		return generationSelection{}, err
	}
	if recovered {
		_, result, _ := transitionCutPaths(journal)
		raw, _, err := readInitialOriginal(s.ctx, result, 256<<10, 0600, 0, 0, false)
		if err != nil {
			return generationSelection{}, err
		}
		var header transitionCutResult
		if json.Unmarshal(raw, &header) != nil || header.Boundary.Journal != journal || header.Boundary.Operation != "start" {
			return generationSelection{}, ErrBinding
		}
		selected, ok := c.selections[digest(header.Boundary.Binding)]
		if !ok || !bytes.Equal(selected.raw, header.Boundary.Binding) {
			return generationSelection{}, ErrBinding
		}
		return selected, nil
	}
	path := filepath.Join(number.directory(c.binding.Request.InstallationRoot), "start-guard.json")
	raw, _, err := readInitialOriginal(s.ctx, path, 64<<10, 0600, 0, 0, false)
	if err != nil {
		return generationSelection{}, err
	}
	var header struct {
		Binding string `json:"binding_sha256"`
	}
	if json.Unmarshal(raw, &header) != nil || !validStartDigest(header.Binding) {
		return generationSelection{}, ErrBinding
	}
	selected, found := c.selections[header.Binding]
	if !found {
		return generationSelection{}, ErrBinding
	}
	return selected, nil
}

func historyDirectoryEntries(directory *inputDirectory) ([]os.DirEntry, error) {
	if directory == nil || directory.file == nil {
		return nil, ErrBinding
	}
	if _, err := directory.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	entries, err := directory.file.ReadDir(historyDirectoryLimit)
	if err != nil && err != io.EOF || len(entries) >= historyDirectoryLimit {
		return nil, errors.Join(ErrBinding, err)
	}
	return entries, nil
}

func (s *invocationChain) checkChain(start *startStorage, retirement *retirementStorage) error {
	if len(s.entries) == 0 {
		return ErrBinding
	}
	if err := s.checkNamespace(start, retirement); err != nil {
		return err
	}
	for _, h := range s.starts {
		if err := h.check(); err != nil {
			return err
		}
	}
	for _, h := range s.retirements {
		if err := h.check(); err != nil {
			return err
		}
	}
	for _, h := range s.terminals {
		if err := h.check(); err != nil {
			return err
		}
	}
	return s.checkNamespace(start, retirement)
}

// A root producer may include ONLY its own actual newborn storage at the next
// start or latest stop boundary. All older files retain their separate pins.
func (s *invocationChain) checkNamespace(start *startStorage, retirement *retirementStorage) error {
	if len(s.entries) == 0 || s.owner == nil || start != nil && retirement != nil {
		return ErrBinding
	}
	c := s.owner
	head := s.head()
	root, latest := c.binding.Request.InstallationRoot, head.number
	if latest >= attemptLimit || len(s.entries) != int(latest)+1 {
		return ErrBinding
	}
	if start != nil {
		if _, err := head.predecessor(); err != nil {
			return err
		}
		if latest+1 >= attemptLimit || start.number != latest+1 || start.path != (latest+1).directory(root) {
			return ErrBinding
		}
		if err := start.check(); err != nil {
			return err
		}
	}
	if retirement != nil {
		if head.start == nil || head.retirement != nil || retirement.number != latest || retirement.path != latest.retirement(root) {
			return ErrBinding
		}
		if err := retirement.check(); err != nil {
			return err
		}
	}
	expected := map[string]bool{}
	for _, entry := range s.entries {
		present := true
		if terminal := entry.terminal; terminal != nil && terminal.directory == nil {
			p := c.files.recoveryPublishing
			present = p != nil && p.births != nil && p.births.dirs[terminal.path] != nil
		}
		if present {
			expected[filepath.Base(entry.number.directory(root))] = true
		}
		if entry.retirement != nil {
			expected[filepath.Base(entry.number.retirement(root))] = true
		}
	}
	if start != nil && start.dir != nil {
		expected[filepath.Base(start.path)] = true
	}
	if retirement != nil && retirement.dir != nil {
		expected[filepath.Base(retirement.path)] = true
	}
	entries, err := historyDirectoryEntries(c.files.dirs[root])
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name != "forwarding-startup" && !strings.HasPrefix(name, "forwarding-invocation-") && !strings.HasPrefix(name, "forwarding-retirement-") {
			continue
		}
		if !expected[name] || !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return ErrBinding
		}
		delete(expected, name)
	}
	if len(expected) != 0 {
		return ErrBinding
	}
	for n := attemptNumber(0); n < attemptLimit; n++ {
		startPresent := n <= latest || start != nil && n == start.number
		retirePresent := n <= latest && s.entries[n].retirement != nil || retirement != nil && n == retirement.number
		for _, item := range []struct {
			path    string
			present bool
		}{{n.journal(root), startPresent}, {n.retirementJournal(root), retirePresent}} {
			_, err := os.Lstat(item.path)
			if item.present {
				if err != nil {
					return errors.Join(ErrBinding, err)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				return errors.Join(ErrBinding, err)
			}
		}
	}
	return s.ctx.Err()
}
