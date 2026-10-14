//go:build linux

package main

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
)

// Real Stock preparation supplies only exclusion, never successful permission,
// issuance or transport. The installed pair exercises the actual presentations.
func TestRouteTokenConcurrentPreparationWaitsAndRetainsCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		flow := &routeTokenFlow{holder: &stock.Owner{}}
		release, err := flow.beginPresentation(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		first, err := flow.holder.Prepare(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		canceled, cancel := context.WithCancel(t.Context())
		outcome := make(chan error, 1)
		go func() {
			unlock, err := flow.beginPresentation(canceled)
			if err == nil {
				unlock()
			}
			outcome <- err
		}()
		synctest.Wait()
		select {
		case err := <-outcome:
			t.Fatal("concurrent presentation did not wait for original preparation", err)
		default:
		}
		cancel()
		if err := <-outcome; !errors.Is(err, context.Canceled) {
			t.Fatal("queued presentation lost original cancellation", err)
		}
		go func() {
			unlock, err := flow.beginPresentation(t.Context())
			if err == nil {
				defer unlock()
				var next *stock.Preparation
				next, err = flow.holder.Prepare(t.Context())
				if err == nil {
					err = next.Close()
				}
			}
			outcome <- err
		}()
		synctest.Wait()
		select {
		case err := <-outcome:
			t.Fatal("canceled sibling released the active Stock preparation", err)
		default:
		}
		if err := first.Close(); err != nil {
			t.Fatal(err)
		}
		release()
		if err := <-outcome; err != nil {
			t.Fatal("next presentation failed after original preparation joined", err)
		}
	})
}
