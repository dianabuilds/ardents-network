package issuance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"
	"github.com/dianabuilds/ardents-network/internal/successor/nodeidentity"
)

var profileFiles = []string{"profile.pin", "profile.bytes", "profile.lock"}

func profileBinding(v Inventory) issuerprofile.Binding {
	return issuerprofile.Binding{Network: v.Binding.Network, Issuer: v.Binding.Issuer, Signer: v.Binding.Signer, Start: v.Binding.Start, End: v.Binding.End}
}
func profileRequest(v Inventory) (issuerprofile.Request, error) {
	return issuerprofile.Prepare(profileBinding(v), v.Keys)
}
func validateProfile(raw []byte, v Inventory) error {
	verified, e := issuerprofile.Verify(raw, profileBinding(v))
	if e != nil {
		return ErrUnavailable
	}
	_, keys, ok := verified.Snapshot()
	if !ok || len(keys) != len(v.Keys) {
		return ErrUnavailable
	}
	for i, k := range keys {
		expected := v.Keys[i]
		if k.Window != expected.Window || k.Class != expected.Class || !bytes.Equal(k.SPKI, expected.SPKI) {
			return ErrUnavailable
		}
	}
	return nil
}
func profilePinBytes(b Binding, raw []byte, digest [32]byte) []byte {
	pin := append([]byte("ardents-issuer-profile-v1\n"), bindingBytes(b)...)
	pin = append(pin, digest[:]...)
	hash := sha256.Sum256(raw)
	return append(pin, hash[:]...)
}

// InitializeProfile signs the Store's exact inventory and commits a fresh root.
func InitializeProfile(ctx context.Context, path string, keys Store, identity nodeidentity.Store) error {
	if err := platform(); err != nil {
		return err
	}
	if ctx == nil {
		return ErrInvalid
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return ErrInvalid
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return ErrUnavailable
	}
	v, err := keys.Inventory()
	if err != nil {
		return err
	}
	b, err := identity.Public()
	if err != nil {
		return err
	}
	if b.Network != v.Binding.Network || b.Node != v.Binding.Issuer || b.Signer != v.Binding.Signer {
		return ErrInvalid
	}
	request, err := profileRequest(v)
	if err != nil {
		return ErrInvalid
	}
	raw, err := identity.SignIssuerProfile(ctx, request)
	if err != nil {
		return err
	}
	if err = validateProfile(raw, v); err != nil {
		return err
	}
	if _, err = keys.Inventory(); err != nil {
		return err
	}
	if _, err = identity.Public(); err != nil {
		return err
	}
	return initializeProfileStorage(ctx, path, v.Binding, raw, v.Digest, nil)
}

type profileState struct {
	mu                sync.Mutex
	files             *storeState
	keys              Store
	raw               []byte
	closed            bool
	failure, closeErr error
}

// ProfileStore copies share one immutable profile lifecycle and lifetime lease.
type ProfileStore struct{ state *profileState }

func OpenProfile(ctx context.Context, path string, keys Store) (ProfileStore, error) {
	if err := platform(); err != nil {
		return ProfileStore{}, err
	}
	if ctx == nil {
		return ProfileStore{}, ErrInvalid
	}
	if ctx.Err() != nil {
		return ProfileStore{}, ctx.Err()
	}
	v, err := keys.Inventory()
	if err != nil {
		return ProfileStore{}, err
	}
	return openProfileStorage(ctx, path, keys, v, nil)
}
func (s *profileState) check() error {
	_, err := s.keys.Inventory()
	return errors.Join(err, s.files.checkIdentity())
}
func (p ProfileStore) Bytes() ([]byte, error) {
	if p.state == nil {
		return nil, ErrClosed
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	if s.failure != nil {
		return nil, ErrUncertain
	}
	if err := s.check(); err != nil {
		s.failure = err
		return nil, ErrUncertain
	}
	return append([]byte(nil), s.raw...), nil
}
func (p ProfileStore) Close() error {
	if p.state == nil {
		return ErrClosed
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		s.closeErr = errors.Join(s.failure, s.check(), releaseLock(s.files.lock), s.files.root.Close())
	}
	return s.closeErr
}
