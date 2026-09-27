//go:build linux

package worker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Activation is the mechanism result of one installed launch attempt. It is
// not a launch receipt: readiness exchange, job authority and Grant binding
// belong to the Endpoint composition that called Activate.
type Activation struct {
	// Queued reports that the activation socket was dialed, so a failed
	// attempt may still have queued a worker activation whose process cleanup
	// the caller must treat as unverified.
	Queued bool
	// Artifact is the pinned root-installed artifact reference.
	Artifact *Artifact
	// Instance is the newly observed manager-owned activation identity; it is
	// only set on a successful attempt.
	Instance Instance
	// Attachment carries the accepted socket. On a failed attempt after the
	// dial it is bound with zero process identity and exists only so the
	// caller closes the socket at its exact cleanup point; on success it is
	// bound to the observed instance identity.
	Attachment *Attachment
}

// Activate performs the installed activation sequence: parent-service and
// platform verification, artifact pinning, activation-socket inspection,
// instance baseline, socket dial, credential preparation, socket identity
// recheck, and new-instance observation. The caller owns the launch
// permission, the bounding context, and cleanup on failure.
func Activate(ctx context.Context, inventory Inventory, role string) (*Activation, error) {
	if err := VerifyEndpointService(ctx); err != nil {
		return nil, err
	}
	artifact, err := LoadArtifact(inventory)
	if err != nil {
		return nil, err
	}
	path := inventory.socket(role)
	socket, err := inspectActivationSocket(path)
	if err != nil {
		return nil, err
	}
	before, err := ListInstances(ctx, role, inventory)
	if err != nil {
		return nil, err
	}
	// Even a failed dial can have queued an activation. Retain failure if its
	// exact invocation cannot be pinned; another job may not reuse that
	// ambiguity.
	activation := &Activation{Queued: true, Artifact: artifact}
	raw, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if err != nil {
		return activation, errors.New("text worker activation failed")
	}
	connection, ok := raw.(*net.UnixConn)
	if !ok {
		_ = raw.Close()
		return activation, errors.New("text worker attachment is not local")
	}
	activation.Attachment = NewAttachment(connection, 0, 0)
	if err := prepareSocket(connection); err != nil {
		return activation, err
	}
	afterSocket, err := inspectActivationSocket(path)
	if err != nil || !os.SameFile(socket, afterSocket) {
		return activation, errors.New("text worker activation socket changed")
	}
	observed, err := awaitInstance(ctx, before, role, inventory)
	if err != nil {
		return activation, err
	}
	activation.Instance = observed
	activation.Attachment = NewAttachment(connection, observed.PID, observed.UID)
	return activation, nil
}

func inspectActivationSocket(path string) (os.FileInfo, error) {
	if path != Text.socket("reader") && path != Text.socket("publisher") && path != Stream.socket("reader") && path != Stream.socket("publisher") || os.Geteuid() == 0 {
		return nil, errors.New("text worker activation address is unavailable")
	}
	if _, err := installedPath(filepath.Dir(path), true); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, errors.New("text worker activation socket is unavailable")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() != 0600 || stat.Uid != uint32(os.Geteuid()) || stat.Gid != uint32(os.Getegid()) {
		return nil, errors.New("text worker activation socket ownership is invalid")
	}
	return info, nil
}

func awaitInstance(ctx context.Context, before Listing, role string, inventory Inventory) (Instance, error) {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var candidate string
	var observationErr error
	for {
		after, err := ListInstances(ctx, role, inventory)
		if err != nil {
			if ctx.Err() != nil && observationErr != nil {
				return Instance{}, fmt.Errorf("text worker activation did not become verifiable (last observation: %w)", observationErr)
			}
			return Instance{}, err
		}
		name, err := selectNewInstance(before, after, candidate)
		if err != nil {
			return Instance{}, err
		}
		if name != "" {
			candidate = name
			observed, err := ObserveInstance(ctx, name, role)
			if err == nil {
				return observed, nil
			}
			// Property checks return fixed, non-secret categories. Keep the
			// latest failure so a timed-out activation remains diagnosable.
			observationErr = err
		}
		select {
		case <-ctx.Done():
			if observationErr != nil {
				return Instance{}, fmt.Errorf("text worker activation did not become verifiable (last observation: %w)", observationErr)
			}
			return Instance{}, errors.New("text worker activation did not become verifiable")
		case <-ticker.C:
		}
	}
}
