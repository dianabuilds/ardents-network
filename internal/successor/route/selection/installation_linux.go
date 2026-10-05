//go:build linux

package selection

import (
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
)

// InstallationConfig binds shared Entry history to an installation observer
// and installation-wide controlled identities. It does not borrow a role's
// observer or private history.
type InstallationConfig struct {
	EntryRoot  string
	Current    func() (network.RuntimeView, error)
	Exclusions []route.Member
}

// RoleConfig names one independent Interior history and local role exclusions.
type RoleConfig struct {
	InteriorRoot string
	Domain       uint8
	Exclusions   []route.Member
}

// Installation exclusively owns Entry history until every role borrower has
// returned. Close seals acquisition and selection before waiting for borrowers.
type Installation struct {
	mu         sync.Mutex
	entries    *ClosedSets
	root       string
	network    [32]byte
	current    func() (network.RuntimeView, error)
	exclusions []route.Member
	borrowers  map[string]bool
	changed    *sync.Cond
	closing    bool
	done       chan struct{}
	failure    error
}

func OpenInstallation(config InstallationConfig) (*Installation, error) {
	if config.Current == nil || config.EntryRoot == "" {
		return nil, errors.New("route installation selection unavailable")
	}
	root, err := selectionRoot(config.EntryRoot)
	if err != nil {
		return nil, err
	}
	initial, err := config.Current()
	if err != nil {
		return nil, err
	}
	if err := initial.Check(initial.ObservedAt()); err != nil {
		return nil, err
	}
	i := &Installation{root: root, network: initial.Profile().Network, current: config.Current, exclusions: append([]route.Member(nil), config.Exclusions...), borrowers: make(map[string]bool), done: make(chan struct{})}
	i.changed = sync.NewCond(&i.mu)
	i.entries, err = openClosedSets(ClosedSetConfig{Root: root, NetworkID: i.network, Current: func() (ClosedSetView, error) {
		view, err := i.observe()
		if err != nil {
			return ClosedSetView{}, err
		}
		return ClosedSetView{NetworkID: view.Profile().Network, Now: view.ObservedAt(), Candidates: selectionCandidates(view, 1, i.exclusions)}, nil
	}})
	if err != nil {
		return nil, err
	}
	return i, nil
}

func (i *Installation) available() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.closing {
		return errors.New("route installation selection closed")
	}
	return nil
}

func (i *Installation) observe() (network.RuntimeView, error) {
	if err := i.available(); err != nil {
		return network.RuntimeView{}, err
	}
	view, err := i.current()
	if err != nil {
		return network.RuntimeView{}, err
	}
	if view.Profile().Network != i.network {
		return network.RuntimeView{}, errors.New("route installation Network changed")
	}
	if err := view.Check(view.ObservedAt()); err != nil {
		return network.RuntimeView{}, err
	}
	if err := i.available(); err != nil {
		return network.RuntimeView{}, err
	}
	return view, nil
}

func (i *Installation) Borrow(config RoleConfig) (_ *Owner, result error) {
	if i == nil || config.InteriorRoot == "" || closedAdjacentIndex(config.Domain) < 0 {
		return nil, errors.New("route role selection unavailable")
	}
	root, err := selectionRoot(config.InteriorRoot)
	if err != nil {
		return nil, err
	}
	i.mu.Lock()
	if i.closing || rootsOverlap(i.root, root) {
		i.mu.Unlock()
		return nil, errors.New("route selection roots unavailable")
	}
	for existing := range i.borrowers {
		if rootsOverlap(existing, root) {
			i.mu.Unlock()
			return nil, errors.New("route Interior root already borrowed")
		}
	}
	i.borrowers[root] = true
	i.mu.Unlock()
	var interior *interiorStore
	defer func() {
		if result != nil {
			var cleanup error
			if interior != nil {
				cleanup = interior.close()
				result = errors.Join(result, cleanup)
			}
			i.returnBorrow(root, cleanup)
		}
	}()
	before, err := i.observe()
	if err != nil {
		return nil, err
	}
	interior, err = openInteriorStore(root, i.network, config.Domain)
	if err != nil {
		return nil, err
	}
	after, err := i.observe()
	if err != nil {
		return nil, err
	}
	if before.Profile().ProfileBinding != after.Profile().ProfileBinding {
		return nil, errors.New("route role opening authority changed")
	}
	o := &Owner{entries: i.entries, interior: interior, current: i.observe, domain: config.Domain, exclusions: append(append([]route.Member(nil), i.exclusions...), config.Exclusions...), installation: i, root: root, closeDone: make(chan struct{})}
	// Publication and seal share this lock: a late opening is never returned.
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.closing {
		return nil, errors.New("route installation selection closed")
	}
	return o, nil
}

func (i *Installation) returnBorrow(root string, cleanup error) {
	i.mu.Lock()
	i.failure = errors.Join(i.failure, cleanup)
	delete(i.borrowers, root)
	i.changed.Broadcast()
	i.mu.Unlock()
}

func (i *Installation) Close() error {
	if i == nil {
		return nil
	}
	i.mu.Lock()
	if i.closing {
		done := i.done
		i.mu.Unlock()
		<-done
		i.mu.Lock()
		result := i.failure
		i.mu.Unlock()
		return result
	}
	i.closing = true
	for len(i.borrowers) > 0 {
		i.changed.Wait()
	}
	i.mu.Unlock()
	result := i.entries.Close()
	i.mu.Lock()
	i.failure = errors.Join(i.failure, result)
	result = i.failure
	close(i.done)
	i.mu.Unlock()
	return result
}

// Resolve existing ancestors too, so a symlink alias cannot place one durable
// owner inside another. Missing roots are compared before either is created.
func selectionRoot(root string) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if info, err := os.Lstat(absolute); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("route selection root is a symlink")
	}
	ancestor := absolute
	var suffix []string
	for {
		_, err := os.Lstat(ancestor)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", err
		}
		suffix = append(suffix, filepath.Base(ancestor))
		ancestor = parent
	}
	resolved, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", err
	}
	for n := len(suffix) - 1; n >= 0; n-- {
		resolved = filepath.Join(resolved, suffix[n])
	}
	return filepath.Clean(resolved), nil
}

func rootsOverlap(first, second string) bool {
	inside := func(root, path string) bool {
		relative, err := filepath.Rel(root, path)
		return err == nil && relative != ".." && !filepath.IsAbs(relative) && (relative == "." || len(relative) < 3 || relative[:3] != ".."+string(filepath.Separator))
	}
	return inside(first, second) || inside(second, first)
}
