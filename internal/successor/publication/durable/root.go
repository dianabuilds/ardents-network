package durable

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
)

const (
	markerName = ".ardents-service-publication-v3"
	marker     = "ardents-service-publication-v3\n"
	lockName   = ".ardents-service-publication-lock"
)

// Config identifies public evidence in one independently owned root.
type Config struct {
	Root            string
	Target, Network [32]byte
}

// Root retains original directory identity and an exclusive process lease.
// Its public floor is an observation, never authority to sign or accept work.
type Root struct {
	mu          sync.Mutex
	path        string
	identity    os.FileInfo
	config      Config
	lease       rootLease
	floor       uint64
	predecessor publication.Delegation
	current     bool
	flush       func(string) error
	failure     error
	reserved    *Generation
	drained     chan struct{}
	closing     chan struct{}
	closed      bool
	closeErr    error
}

// Open creates the owned layout or reconciles an existing v3 root. It never
// reads the retired separate decimal generation source or adopts another root.
func Open(ctx context.Context, config Config) (*Root, error) {
	return openWith(ctx, config, syncDirectory)
}

func openWith(ctx context.Context, config Config, flush func(string) error) (*Root, error) {
	if !platformSupported || ctx == nil || config.Root == "" || config.Target == [32]byte{} || config.Network == [32]byte{} {
		return nil, errors.New("publication root configuration unavailable")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := filepath.Abs(config.Root)
	if err != nil {
		return nil, err
	}
	if err = createDirectory(path, flush); err != nil {
		return nil, err
	}
	identity, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	entries, err := scan(path, 5)
	if err != nil {
		return nil, err
	}
	marked := false
	for _, entry := range entries {
		if entry.Name() == markerName {
			marked = true
		}
	}
	if !marked {
		for _, entry := range entries {
			if entry.Name() != lockName {
				return nil, errors.New("publication root is unowned and nonempty")
			}
		}
	}
	lock := filepath.Join(path, lockName)
	if err = exclusive(lock, nil); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	lease, err := acquireLease(path)
	if err != nil {
		return nil, err
	}
	fail := func(cause error) (*Root, error) { return nil, errors.Join(cause, lease.release()) }
	if !marked {
		// Reinspect after acquiring the lease; another initializer may have won.
		if raw, readErr := readRegular(filepath.Join(path, markerName), int64(len(marker))); readErr == nil {
			if string(raw) != marker {
				return fail(errors.New("publication marker invalid"))
			}
		} else if !errors.Is(readErr, os.ErrNotExist) {
			return fail(readErr)
		} else {
			entries, err = scan(path, 1)
			if err != nil || len(entries) != 1 || entries[0].Name() != lockName {
				return fail(errors.New("publication root changed before claim"))
			}
			if err = exclusive(filepath.Join(path, markerName), []byte(marker)); err != nil {
				return fail(err)
			}
		}
	}
	raw, err := readRegular(filepath.Join(path, markerName), int64(len(marker)))
	if err != nil || string(raw) != marker {
		return fail(errors.Join(errors.New("publication marker invalid"), err))
	}
	if err = resyncFile(filepath.Join(path, markerName)); err != nil {
		return fail(err)
	}
	generations := filepath.Join(path, "generations")
	if err = os.Mkdir(generations, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return fail(err)
	}
	info, err := os.Lstat(generations)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fail(errors.New("publication generations invalid"))
	}
	for _, directory := range []string{generations, path, filepath.Dir(path)} {
		if err = flush(directory); err != nil {
			return fail(err)
		}
	}
	root := &Root{path: path, identity: identity, config: config, lease: lease, flush: flush}
	if err = root.restore(flush); err != nil {
		return fail(err)
	}
	if err = root.check(ctx); err != nil {
		return fail(err)
	}
	return root, nil
}

func (root *Root) check(ctx context.Context) error {
	if root.closed || root.failure != nil {
		return errors.Join(errors.New("publication root unavailable"), root.failure)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return root.checkOriginal()
}

func (root *Root) checkOriginal() error {
	info, err := os.Lstat(root.path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, root.identity) {
		return errors.New("publication original root unavailable")
	}
	return root.lease.check(root.path)
}

// Floor returns detached monotonic history after checking the original root.
func (root *Root) Floor(ctx context.Context) (uint64, error) {
	if root == nil || ctx == nil {
		return 0, errors.New("publication root unavailable")
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.check(ctx); err != nil {
		return 0, err
	}
	return root.floor, nil
}

// Close retains the exact terminal lease-release result for all callers.
func (root *Root) Close() error {
	if root == nil {
		return nil
	}
	root.mu.Lock()
	if root.closing != nil {
		done := root.closing
		root.mu.Unlock()
		<-done
		root.mu.Lock()
		defer root.mu.Unlock()
		return root.closeErr
	}
	root.closed = true // Deny new effects before waiting for the original owner.
	root.closing = make(chan struct{})
	drained := root.drained
	root.mu.Unlock()
	if drained != nil {
		<-drained
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	root.closeErr = errors.Join(root.failure, root.lease.release())
	close(root.closing)
	return root.closeErr
}
