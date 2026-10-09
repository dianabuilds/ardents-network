package instance

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
)

const markerName = "instance-root.marker"
const marker = "ardents-service-instance-root-v3\n"
const stateName = "instance-root.json"
const lockName = ".instance-root.lock"

// Config contains only host-selected public request facts. It cannot supply
// private material, an Authority signer or an arbitrary Instance public key.
type Config struct {
	Root                string
	NetworkID           [32]byte
	NotBefore, NotAfter time.Time
}

// Root owns exactly one host-generated key and response transition. No method
// exposes a crypto.Signer, private bytes, generic signing or Authority custody.
type Root struct {
	mu                sync.Mutex
	path              string
	identity          os.FileInfo
	lease             rootLease
	flush             func(string) error
	state             generationState
	closed            bool
	failure, closeErr error
}

func Initialize(ctx context.Context, config Config) (*Root, error) {
	if config.NetworkID == [32]byte{} || !canonicalTime(config.NotBefore) || !canonicalTime(config.NotAfter) || !config.NotAfter.After(config.NotBefore) || config.NotAfter.Sub(config.NotBefore) > 24*time.Hour {
		return nil, ErrInvalid
	}
	root, err := open(ctx, config.Root, true)
	if err != nil {
		return nil, err
	}
	if root.state.phase != "" {
		request := root.state.request
		if request.NetworkID != config.NetworkID || request.NotBefore != config.NotBefore.Unix() || request.NotAfter != config.NotAfter.Unix() {
			return nil, errors.Join(ErrInvalid, root.Close())
		}
		return root, nil
	}
	next, err := generateState(config.NetworkID, config.NotBefore.Unix(), config.NotAfter.Unix())
	if err != nil {
		return nil, errors.Join(err, root.Close())
	}
	if err = root.writeState(ctx, next); err != nil {
		next.erase()
		return nil, errors.Join(err, root.Close())
	}
	root.state = next
	return root, nil
}

func Open(ctx context.Context, path string) (*Root, error) {
	root, err := open(ctx, path, false)
	if err != nil {
		return nil, err
	}
	if root.state.phase == "" {
		return nil, errors.Join(ErrInvalid, root.Close())
	}
	return root, nil
}

func open(ctx context.Context, path string, create bool) (*Root, error) {
	if !nativeSupported || ctx == nil || path == "" {
		return nil, ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if create {
		if err = prepareDirectory(abs); err != nil {
			return nil, err
		}
	}
	identity, err := os.Lstat(abs)
	if err != nil || !identity.IsDir() || identity.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInvalid
	}
	if err = privateAccess(identity); err != nil {
		return nil, err
	}
	if create {
		if err = writeExclusive(filepath.Join(abs, lockName), nil); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	lease, err := acquireLease(abs)
	if err != nil {
		return nil, err
	}
	root := &Root{path: abs, identity: identity, lease: lease, flush: syncDirectory}
	fail := func(cause error) (*Root, error) { return nil, errors.Join(cause, root.Close()) }
	if err = root.check(ctx); err != nil {
		return fail(err)
	}
	if err = root.prepareLayout(create); err != nil {
		return fail(err)
	}
	raw, err := readPrivate(filepath.Join(abs, stateName), 4096)
	if err == nil {
		defer clear(raw)
		root.state, err = unmarshalState(raw)
	} else if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	if err != nil {
		return fail(err)
	}
	if err = root.check(ctx); err != nil {
		return fail(err)
	}
	return root, nil
}

func (root *Root) check(ctx context.Context) error {
	if root.closed || root.failure != nil {
		return errors.Join(ErrUnavailable, root.failure)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Lstat(root.path)
	if err != nil || !info.IsDir() || !os.SameFile(root.identity, info) || info.Mode()&os.ModeSymlink != 0 {
		return ErrUnavailable
	}
	if err = privateAccess(info); err != nil {
		return err
	}
	return root.lease.check(root.path)
}

func (root *Root) Request(ctx context.Context) ([]byte, error) {
	if root == nil || ctx == nil {
		return nil, ErrUnavailable
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.check(ctx); err != nil {
		return nil, err
	}
	if root.state.phase == "" {
		return nil, ErrInvalid
	}
	return encodeRequest(root.state.request), nil
}

// Credential supplies only verified public evidence for floor reconciliation.
// A consumed persisted root cannot revive its original private lifetime.
func (root *Root) Credential(ctx context.Context) (publication.Credential, error) {
	if root == nil || ctx == nil {
		return publication.Credential{}, ErrUnavailable
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.check(ctx); err != nil {
		return publication.Credential{}, err
	}
	switch root.state.phase {
	case Pending:
		return publication.Credential{}, ErrPending
	case Consumed:
		return publication.Credential{}, ErrSuccessorRequired
	case Accepted:
		return verifyResponse(root.state.response, root.state.request)
	default:
		return publication.Credential{}, ErrUnavailable
	}
}

func (root *Root) Close() error {
	if root == nil {
		return nil
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if !root.closed {
		root.closed = true
		root.state.erase()
		root.closeErr = errors.Join(root.failure, root.lease.release())
	}
	return root.closeErr
}

func canonicalTime(value time.Time) bool {
	return !value.IsZero() && value.Unix() >= 0 && value.Equal(value.UTC().Truncate(time.Second))
}
