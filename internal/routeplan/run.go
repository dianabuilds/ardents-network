package routeplan

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// Run executes one validated role-local sequence and optionally composes its
// sole client with a manifest-bound entry channel before serializing evidence.
func Run(ctx context.Context, sequence *Sequence, encode func(any) error,
	entryManifest [32]byte,
	openEntry func(context.Context, func(context.Context, net.Conn) (*tls.Conn, error)) (*tls.Conn, func() error, error),
) error {
	if sequence.Concurrent() {
		if openEntry != nil || entryManifest != [32]byte{} {
			return errors.New("concurrent Route sequence cannot receive an entry override")
		}
		return runConcurrent(ctx, sequence, encode)
	}
	for {
		step, ok, err := sequence.Next()
		if err != nil {
			return fmt.Errorf("construct Route Attachment: %w", err)
		}
		if !ok {
			return nil
		}
		if openEntry != nil {
			if step.Actor.Role != "client" || step.Actor.ManifestDigest != entryManifest {
				return errors.Join(errors.New("entry channel does not bind this client Route manifest"), step.Close())
			}
			step.Actor.OpenEntry = openEntry
		} else if entryManifest != [32]byte{} {
			return errors.Join(errors.New("entry channel is incomplete"), step.Close())
		}
		var readyErr error
		var ready func(route.Evidence)
		if step.Actor.Role != "client" {
			ready = func(value route.Evidence) {
				value.Attachment = step.Attachment
				readyErr = encode(value)
			}
		}
		result, runErr := route.Run(ctx, step.Actor, ready)
		runErr = errors.Join(runErr, readyErr)
		result.Attachment = step.Attachment
		closeErr := step.Close()
		if closeErr != nil {
			return errors.Join(runErr, fmt.Errorf("close Route Attachment %d: %w", step.Attachment, closeErr))
		}
		if runErr != nil && !step.More {
			result.Error = runErr.Error()
			return errors.Join(runErr, encode(result))
		}
		if err := encode(result); err != nil {
			return fmt.Errorf("encode Route Attachment %d evidence: %w", step.Attachment, err)
		}
	}
}
