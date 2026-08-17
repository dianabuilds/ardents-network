package camouflage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	pinnedServerBytes  = 5899325
	pinnedServerSHA256 = "5fe32f8ab736ed54fc66027775761084e68f0e1ec9b5fea7c3417c6617255336"
)

// Server names the fixed external WebTunnel server and TLS front inputs.
type Server struct {
	Binary          string
	StateRoot       string
	Certificate     string
	Key             string
	NextLeg         string
	Deadline        time.Time
	ResourceProfile string
}

// ServerControl is the bounded Bridge-side Adapter lifecycle and evidence seam.
type ServerControl interface {
	Protect(bool)
	Admission() (uint64, uint64, uint64)
	Close() error
}

type serving struct {
	front   *tlsFront
	child   *candidateChild
	state   string
	parent  time.Time
	stopped chan struct{}
	once    sync.Once
	stopErr error
}

// Serve verifies the pinned supply before starting one Bridge-side WebTunnel
// server and its standard-library TLS/HTTP front.
func Serve(ctx context.Context, config Config, server Server) (*serving, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := verifyExecutable(server.Binary, pinnedServerBytes, pinnedServerSHA256); err != nil {
		return nil, err
	}
	if config.commitment == ([32]byte{}) || server.StateRoot == "" || !filepath.IsAbs(server.StateRoot) ||
		!validNextLeg(server.NextLeg) || !time.Now().Before(server.Deadline) {
		return nil, errInvalidConfig
	}
	capacity, err := serverCapacity(server.ResourceProfile)
	if err != nil {
		return nil, err
	}
	certificate, err := loadServerCertificate(config, server.Certificate, server.Key)
	if err != nil {
		return nil, err
	}
	url, err := publicURL(config)
	if err != nil {
		return nil, err
	}
	startupDeadline := server.Deadline
	if maximum := time.Now().Add(5 * time.Second); maximum.Before(startupDeadline) {
		startupDeadline = maximum
	}
	if err := os.Mkdir(server.StateRoot, 0o700); err != nil {
		return nil, fmt.Errorf("adapter-state-invalid: %w", err)
	}
	bind, err := reserveLoopbackAddress()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("adapter-startup-failed: %w", err),
			cleanupFailure(removeAndVerifyState(server.StateRoot, cleanupDeadline(server.Deadline))))
	}
	child, err := startServerProcess(ctx, server.Binary, server.StateRoot, bind, server.NextLeg, url,
		startupDeadline, server.Deadline)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("adapter-startup-failed: %w", err),
			cleanupFailure(removeAndVerifyState(server.StateRoot, cleanupDeadline(server.Deadline))))
	}
	front, err := startTLSFront(config, certificate, bind, capacity)
	if err != nil {
		deadline := cleanupDeadline(server.Deadline)
		cleanupErr := errors.Join(child.closeBefore(deadline), removeAndVerifyState(server.StateRoot, deadline))
		return nil, errors.Join(fmt.Errorf("adapter-startup-failed: %w", err), cleanupFailure(cleanupErr))
	}
	serving := &serving{front: front, child: child, state: server.StateRoot, parent: server.Deadline,
		stopped: make(chan struct{})}
	go func() {
		select {
		case <-ctx.Done():
			_ = serving.Close()
		case <-serving.stopped:
		}
	}()
	return serving, nil
}

type bridgeCapacity struct {
	sessions   uint16
	rawSockets uint16
}

func serverCapacity(profile string) (bridgeCapacity, error) {
	switch profile {
	case "h3-s-v1":
		return bridgeCapacity{sessions: 4, rawSockets: 32}, nil
	case "h3-s-v1-strong":
		return bridgeCapacity{sessions: 16, rawSockets: 128}, nil
	default:
		return bridgeCapacity{}, errInvalidConfig
	}
}

// Protect rejects new front admissions while preserving established work.
func (serving *serving) Protect(enabled bool) { serving.front.protect(enabled) }

// Admission reports the exact authenticated-session counters for external resource evidence.
func (serving *serving) Admission() (uint64, uint64, uint64) {
	return serving.front.session.snapshot()
}

// Close stops admissions, closes all owned resources, and is idempotent.
func (serving *serving) Close() error {
	serving.once.Do(func() {
		close(serving.stopped)
		serving.front.protect(true)
		deadline := cleanupDeadline(serving.parent)
		serving.stopErr = cleanupFailure(errors.Join(serving.front.closeBefore(deadline),
			serving.child.closeBefore(deadline), removeAndVerifyState(serving.state, deadline)))
	})
	return serving.stopErr
}
