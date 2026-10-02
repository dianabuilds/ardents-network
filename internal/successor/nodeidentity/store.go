package nodeidentity

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"
)

var (
	ErrInvalid     = errors.New("invalid-input")
	ErrUnavailable = errors.New("storage-unavailable")
	ErrUncertain   = errors.New("storage-uncertain")
	ErrBusy        = errors.New("busy")
	ErrClosed      = errors.New("closed")
	ErrUnsupported = errors.New("unsupported-platform")
)

// Binding holds independently selected public identity facts, never State duty.
type Binding struct{ Network, Node, Signer [32]byte }

func (b Binding) valid() bool {
	return b.Network != ([32]byte{}) && b.Node != ([32]byte{}) && b.Signer != ([32]byte{})
}
func parseKey(raw []byte, b Binding) (ed25519.PrivateKey, error) {
	parsed, err := x509.ParsePKCS8PrivateKey(raw)
	if err != nil {
		return nil, ErrInvalid
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok || len(key) != ed25519.PrivateKeySize || !bytes.Equal(key.Public().(ed25519.PublicKey), b.Signer[:]) {
		clear(key)
		return nil, ErrInvalid
	}
	canonical, err := x509.MarshalPKCS8PrivateKey(key)
	defer clear(canonical)
	if err != nil || !bytes.Equal(raw, canonical) {
		clear(key)
		return nil, ErrInvalid
	}
	return key, nil
}
func decodePEM(raw []byte, b Binding) ([]byte, error) {
	if len(raw) > 64<<10 || !b.valid() {
		return nil, ErrInvalid
	}
	block, rest := pem.Decode(raw)
	normalized := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	defer clear(normalized)
	if block == nil || block.Type != "PRIVATE KEY" || len(block.Headers) != 0 || len(rest) != 0 || !bytes.Equal(normalized, pem.EncodeToMemory(block)) {
		return nil, ErrInvalid
	}
	defer clear(block.Bytes)
	key, err := parseKey(block.Bytes, b)
	defer clear(key)
	if err != nil {
		return nil, err
	}
	return x509.MarshalPKCS8PrivateKey(key)
}
func pinBytes(b Binding, raw []byte) []byte {
	pin := append([]byte("ardents-node-identity-v1\n"), b.Network[:]...)
	pin = append(pin, b.Node[:]...)
	pin = append(pin, b.Signer[:]...)
	digest := sha256.Sum256(raw)
	return append(pin, digest[:]...)
}

type identityState struct {
	mu                sync.Mutex
	root              *os.Root
	path              string
	identity          os.FileInfo
	files             map[string]os.FileInfo
	lock              *os.File
	binding           Binding
	key               ed25519.PrivateKey
	closed            bool
	failure, closeErr error
}

// Store copies share the private owner, lease, terminal state and close result.
type Store struct{ state *identityState }

func Import(ctx context.Context, path, source string, b Binding) error {
	if err := platform(); err != nil {
		return err
	}
	if ctx == nil || !b.valid() {
		return ErrInvalid
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if !filepath.IsAbs(source) || filepath.Clean(source) != source {
		return ErrInvalid
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return ErrInvalid
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return ErrUnavailable
	}
	root, directory, err := ownedRoot(filepath.Dir(source))
	if err != nil {
		return err
	}
	raw, _, readErr := readOwned(root, filepath.Base(source), 64<<10)
	current, pathErr := os.Lstat(filepath.Dir(source))
	resolved, resolveErr := filepath.EvalSymlinks(filepath.Dir(source))
	if pathErr != nil || resolveErr != nil || resolved != filepath.Dir(source) || !privateFile(current, true) || !os.SameFile(directory, current) {
		readErr = errors.Join(readErr, ErrUnavailable)
	}
	closeErr := root.Close()
	defer clear(raw)
	if err = errors.Join(readErr, closeErr); err != nil {
		return err
	}
	der, err := decodePEM(raw, b)
	defer clear(der)
	if err != nil {
		return err
	}
	return initialize(ctx, path, b, der, nil)
}
func Open(ctx context.Context, path string, b Binding) (Store, error) {
	if err := platform(); err != nil {
		return Store{}, err
	}
	if ctx == nil || !b.valid() {
		return Store{}, ErrInvalid
	}
	if ctx.Err() != nil {
		return Store{}, ctx.Err()
	}
	return openStore(ctx, path, b, nil)
}
func (s Store) Public() (Binding, error) {
	if s.state == nil {
		return Binding{}, ErrClosed
	}
	v := s.state
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return Binding{}, ErrClosed
	}
	if v.failure != nil {
		return Binding{}, ErrUncertain
	}
	if err := v.check(); err != nil {
		v.failure = err
		return Binding{}, ErrUncertain
	}
	return v.binding, nil
}
func (s Store) SignIssuerProfile(ctx context.Context, request issuerprofile.Request) ([]byte, error) {
	if s.state == nil || ctx == nil {
		return nil, ErrInvalid
	}
	v := s.state
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return nil, ErrClosed
	}
	if v.failure != nil {
		return nil, ErrUncertain
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	b, transcript, ok := request.Snapshot()
	if !ok || b.Network != v.binding.Network || b.Issuer != v.binding.Node || b.Signer != v.binding.Signer {
		return nil, ErrInvalid
	}
	if err := v.check(); err != nil {
		v.failure = err
		return nil, ErrUncertain
	}
	signature := ed25519.Sign(v.key, transcript)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err := v.check(); err != nil {
		v.failure = err
		return nil, ErrUncertain
	}
	return request.Complete(signature)
}
func (s Store) Close() error {
	if s.state == nil {
		return ErrClosed
	}
	v := s.state
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.closed {
		v.closed = true
		v.closeErr = errors.Join(v.failure, v.check(), releaseLock(v.lock), v.root.Close())
		clear(v.key)
	}
	return v.closeErr
}
