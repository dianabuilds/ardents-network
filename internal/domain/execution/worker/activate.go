package worker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

// Activate performs the installed activation sequence: parent-service and
// platform verification, artifact pinning, activation-socket inspection,
// instance baseline, socket dial, credential preparation, socket identity
// recheck, and new-instance observation. The caller owns the launch
// permission, the bounding context, and cleanup on failure.
func Activate(ctx context.Context, role string) (*Activation, error) {
	return activate(ctx, applicationInventory, role)
}

// ActivateStream is the trusted qualification activation entry for the Stream
// inventory. Ordinary Application jobs use Activate; the architecture gate restricts
// Stream callers to the qualification runner (no caller-visible selector).
func ActivateStream(ctx context.Context, role string) (*Activation, error) {
	return activate(ctx, streamInventory, role)
}

// activate is the portable orchestration skeleton shared by every platform.
// Each platform step stays behind a capability symbol: the Linux adapter
// implements them through the installed systemd/cgroup mechanism, while the
// macOS and non-Unix adapters refuse at the first parent-service verification,
// before any artifact, activation or cleanup pin (ADR-0129). The sequence itself
// grants no authority; it only orders the platform facts into one Activation.
func activate(ctx context.Context, inv inventory, role string) (*Activation, error) {
	if err := verifyEndpointService(ctx); err != nil {
		return nil, err
	}
	artifact, err := loadArtifact(inv)
	if err != nil {
		return nil, err
	}
	if err := verifyActivationSocket(ctx, inv, role); err != nil {
		return nil, err
	}
	path := inv.socket(role)
	socket, err := inspectActivationSocket(inv, path)
	if err != nil {
		return nil, err
	}
	before, err := listInstances(ctx, inv, role)
	if err != nil {
		return nil, err
	}
	// Even a failed dial can have queued an activation. Retain failure if its
	// exact invocation cannot be pinned; another job may not reuse that
	// ambiguity.
	activation := &Activation{Queued: true, Artifact: artifact}
	raw, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if err != nil {
		return activation, errors.New("document worker activation failed")
	}
	connection, ok := raw.(*net.UnixConn)
	if !ok {
		_ = raw.Close()
		return activation, errors.New("document worker attachment is not local")
	}
	activation.Attachment = newAttachment(connection, 0, 0)
	if err := prepareSocket(connection); err != nil {
		return activation, err
	}
	afterSocket, err := inspectActivationSocket(inv, path)
	if err != nil || !os.SameFile(socket, afterSocket) {
		return activation, errors.New("document worker activation socket changed")
	}
	observed, err := awaitInstance(ctx, inv, before, role)
	if err != nil {
		return activation, err
	}
	activation.Instance = observed
	activation.Attachment = newAttachment(connection, observed.PID, observed.UID)
	return activation, nil
}

func awaitInstance(ctx context.Context, inv inventory, before listing, role string) (Instance, error) {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var candidate string
	var observationErr error
	for {
		after, err := listInstances(ctx, inv, role)
		if err != nil {
			if ctx.Err() != nil && observationErr != nil {
				return Instance{}, fmt.Errorf("document worker activation did not become verifiable (last observation: %w)", observationErr)
			}
			return Instance{}, err
		}
		name, err := selectNewInstance(before, after, candidate)
		if err != nil {
			return Instance{}, err
		}
		if name != "" {
			candidate = name
			observed, err := observeInstance(ctx, inv, name, role)
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
				return Instance{}, fmt.Errorf("document worker activation did not become verifiable (last observation: %w)", observationErr)
			}
			return Instance{}, errors.New("document worker activation did not become verifiable")
		case <-ticker.C:
		}
	}
}
