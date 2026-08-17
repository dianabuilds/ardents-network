package camouflage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	pinnedClientBytes  = 7690615
	pinnedClientSHA256 = "de581c8dd36193bb4168aee840406294af406bf8187817c10ac2bcd9464fd120"
)

// Client names the fixed external WebTunnel client runtime inputs.
type Client struct {
	Binary    string
	StateRoot string
	Deadline  time.Time
}

// OpenClient verifies the pinned supply before starting one WebTunnel client
// and returning its single owned carrier.
func OpenClient(ctx context.Context, config Config, client Client) (net.Conn, func() error, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, true, err
	}
	if config.commitment == ([32]byte{}) || client.StateRoot == "" || !filepath.IsAbs(client.StateRoot) ||
		!time.Now().Before(client.Deadline) {
		return nil, nil, true, errInvalidConfig
	}
	if err := verifyExecutable(client.Binary, pinnedClientBytes, pinnedClientSHA256); err != nil {
		return nil, nil, true, err
	}
	startupDeadline := client.Deadline
	if maximum := time.Now().Add(5 * time.Second); maximum.Before(startupDeadline) {
		startupDeadline = maximum
	}
	if err := os.Mkdir(client.StateRoot, 0o700); err != nil {
		return nil, nil, true, fmt.Errorf("adapter-state-invalid: %w", err)
	}
	child, listener, err := startClientProcess(ctx, client.Binary, client.StateRoot, startupDeadline, client.Deadline)
	if err != nil {
		deadline := cleanupDeadline(client.Deadline)
		stateErr := removeAndVerifyState(client.StateRoot, deadline)
		return nil, nil, stateErr == nil,
			errors.Join(fmt.Errorf("adapter-startup-failed: %w", err), cleanupFailure(stateErr))
	}
	carrier, err := openClientSOCKS(listener, config, startupDeadline)
	if err != nil {
		deadline := cleanupDeadline(client.Deadline)
		cleanupErr := errors.Join(child.closeBefore(deadline), removeAndVerifyState(client.StateRoot, deadline))
		return nil, nil, cleanupErr == nil, errors.Join(fmt.Errorf("adapter-socks-refused: %w", err),
			cleanupFailure(cleanupErr))
	}
	var once sync.Once
	var closeErr error
	stopped := make(chan struct{})
	cleanup := func() error {
		once.Do(func() {
			close(stopped)
			deadline := cleanupDeadline(client.Deadline)
			carrierErr := carrier.Close()
			if errors.Is(carrierErr, net.ErrClosed) {
				carrierErr = nil
			}
			closeErr = cleanupFailure(errors.Join(carrierErr, child.closeBefore(deadline),
				removeAndVerifyState(client.StateRoot, deadline)))
		})
		return closeErr
	}
	go func() {
		select {
		case <-ctx.Done():
			_ = cleanup()
		case <-stopped:
		}
	}()
	return carrier, cleanup, true, nil
}

func verifyExecutable(path string, size int64, expected string) error {
	if path == "" || !filepath.IsAbs(path) {
		return errors.New("adapter-supply-invalid")
	}
	file, err := os.Open(path)
	if err != nil {
		return errors.New("adapter-supply-invalid")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != size {
		return errors.New("adapter-supply-invalid")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil || hex.EncodeToString(hash.Sum(nil)) != expected {
		return errors.New("adapter-supply-invalid")
	}
	return nil
}
