package durable

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
)

// ErrConsumed denies a Credential whose generation is already in this root's
// reconciled history. It never supplies a substitute signing authority.
var ErrConsumed = errors.New("publication generation already consumed")

// Generation retains one exact Credential reconciliation and the original root
// lease. No caller-selected floor or generation can replace that binding.
type Generation struct {
	root       *Root
	credential publication.Credential
	advanced   bool
	record     []byte
	closed     bool
}

// Reserve checks the verified Credential against this original root, its actual
// floor and any retained predecessor validity. Close waits for this reservation.
func (root *Root) Reserve(ctx context.Context, credential publication.Credential) (*Generation, error) {
	if root == nil || ctx == nil {
		return nil, errors.New("publication root unavailable")
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.check(ctx); err != nil {
		return nil, err
	}
	value := credential.Delegation()
	now := time.Now()
	if value.Generation == 0 || value.Target != root.config.Target || value.Network != root.config.Network || now.Before(value.NotBefore) || !now.Before(value.NotAfter) {
		return nil, errors.New("publication Credential unavailable")
	}
	if root.reserved != nil {
		return nil, errors.New("publication generation retained")
	}
	floor, _, err := readFloor(root.path)
	if err != nil || floor != root.floor {
		return nil, errors.Join(errors.New("publication original floor changed"), err)
	}
	if floor >= value.Generation {
		return nil, ErrConsumed
	}
	if root.predecessor.NotAfter.After(value.NotBefore) {
		return nil, errors.New("publication successor overlaps predecessor")
	}
	generation := &Generation{root: root, credential: credential}
	root.reserved, root.drained = generation, make(chan struct{})
	return generation, nil
}

// Check reobserves the original reservation; copied public facts are insufficient.
func (generation *Generation) Check(ctx context.Context) error {
	if generation == nil || generation.root == nil || ctx == nil {
		return errors.New("publication generation unavailable")
	}
	root := generation.root
	root.mu.Lock()
	defer root.mu.Unlock()
	return generation.check(ctx)
}

func (generation *Generation) check(ctx context.Context) error {
	root := generation.root
	if generation.closed || root.reserved != generation {
		return errors.New("publication generation unavailable")
	}
	if err := root.check(ctx); err != nil {
		return err
	}
	floor, _, err := readFloor(root.path)
	if err != nil || floor != root.floor {
		return errors.Join(errors.New("publication original floor changed"), err)
	}
	value := generation.credential.Delegation()
	now := time.Now()
	if now.Before(value.NotBefore) || !now.Before(value.NotAfter) {
		return errors.New("publication generation expired")
	}
	return nil
}

// Advance durably burns this exact reserved generation before any readiness.
// An exact in-process retry has no second write or deadline; any ambiguous
// persistence disables all receipts on this owner until independent reopen.
func (generation *Generation) Advance(ctx context.Context) (result error) {
	if generation == nil || generation.root == nil || ctx == nil {
		return errors.New("publication generation unavailable")
	}
	root := generation.root
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := generation.check(ctx); err != nil {
		return err
	}
	if generation.advanced {
		return nil
	}
	defer func() {
		if result != nil {
			root.failure = errors.Join(root.failure, result)
		}
	}()
	if err := root.retireRecord(); err != nil {
		return err
	}
	value := generation.credential.Delegation().Generation
	if err := root.replaceLeaf(ctx, "floor", []byte(strconv.FormatUint(value, 10)+"\n")); err != nil {
		return err
	}
	root.floor = value
	if err := generation.check(ctx); err != nil {
		return err
	}
	generation.advanced = true
	return nil
}

func (root *Root) replaceLeaf(ctx context.Context, name string, raw []byte) (result error) {
	file, err := os.CreateTemp(root.path, ".publication-staging-")
	if err != nil {
		return err
	}
	path := file.Name()
	defer func() {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			result = errors.Join(result, err)
		}
	}()
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(raw)
	}
	if err == nil {
		err = file.Sync()
	}
	if err = errors.Join(err, file.Close()); err != nil {
		return err
	}
	if err = root.check(ctx); err != nil {
		return err
	}
	if err = os.Rename(path, filepath.Join(root.path, name)); err != nil {
		return err
	}
	if err = root.flush(root.path); err != nil {
		return err
	}
	return root.check(ctx)
}

// Close releases only the reservation. It neither erases the floor nor returns
// the root lease while another original owner still retains this generation.
func (generation *Generation) Close() error {
	if generation == nil || generation.root == nil {
		return nil
	}
	root := generation.root
	root.mu.Lock()
	defer root.mu.Unlock()
	if !generation.closed {
		generation.closed = true
		if root.reserved == generation {
			if len(generation.record) != 0 && root.failure == nil {
				err := root.checkOriginal()
				if err == nil {
					err = root.retireRecord()
				}
				root.failure = errors.Join(root.failure, err)
			}
			root.reserved = nil
			close(root.drained)
			root.drained = nil
		}
	}
	return root.failure
}
