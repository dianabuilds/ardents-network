package main

import (
	"context"
	"errors"
	"time"
)

// runOpenedSource owns the ready-to-terminal event sequence. Plan and State
// admission failures happen before this sequence and retain their no-output
// refusal contract.
func runOpenedSource(ctx context.Context, store sourceStore, events *eventOutput) error {
	snapshot, err := store.Current()
	if err == nil {
		err = events.encode(map[string]any{
			"schema": "ardents-source-event-v1", "kind": "source-ready", "at": time.Now().UTC(),
			"generation": snapshot.Generation, "epoch": snapshot.Epoch,
		})
	}
	if err != nil {
		_ = store.Close()
		return err
	}
	waitErr := store.Wait(ctx)
	closeErr := store.Close()
	if waitErr != nil {
		return errors.Join(waitErr, events.encode(map[string]any{
			"schema": "ardents-source-event-v1", "kind": "source-failed", "at": time.Now().UTC(),
			"reason": "background-work",
		}))
	}
	if closeErr != nil {
		return errors.Join(closeErr, events.encode(map[string]any{
			"schema": "ardents-source-event-v1", "kind": "source-failed", "at": time.Now().UTC(),
			"reason": "cleanup",
		}))
	}
	return nil
}
