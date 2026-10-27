//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/spending"
)

// Linux is required for the genuine State, participation and Hosting durable
// owners. This exercises their existing public composition, not an installed
// role, a supplied successful authority or an installed bootstrap delegation.
func TestNetworkReceivingHeldForeignWorkRetainsOriginalCustody(t *testing.T) {
	for _, operation := range []string{"accept", "refill"} {
		for _, boundary := range []string{"observation", "reservation"} {
			for _, denial := range []string{"cancel", "close"} {
				t.Run(operation+"/"+boundary+"/"+denial, func(t *testing.T) {
					f := newNetworkAdmissionFixture(t)
					_, tokens := f.tokens(t, 3)
					budget := networkTestBudget(t)
					root := t.TempDir()
					deadline := time.Now().Add(20 * time.Second)
					entered, resume := make(chan struct{}), make(chan struct{})
					releasing, releaseJoin := make(chan struct{}), make(chan struct{})
					var resumeOnce, releaseOnce sync.Once
					defer resumeOnce.Do(func() { close(resume) })
					defer releaseOnce.Do(func() { close(releaseJoin) })
					var hold atomic.Bool
					owner, err := receiving.Open(root, f.receiver, func() (receiving.Observation, error) {
						v, err := f.authority.receiver(f.receiver, f.profile.NotAfter)
						if boundary == "observation" && hold.Load() {
							close(entered)
							<-resume
						}
						return v, err
					})
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := owner.Close(); err != nil {
							t.Error(err)
						}
					})
					var prior receiving.Grant
					if operation == "refill" {
						prior, err = owner.Accept(t.Context(), admission.ForwardClass, tokens[0], deadline, func() (func() error, error) { return networkTestReservation(t, budget, deadline) })
						if err != nil {
							t.Fatal(err)
						}
						defer prior.Release()
					}
					before, err := os.ReadFile(filepath.Join(root, "closed-token-spends"))
					if err != nil {
						t.Fatal(err)
					}
					baseline, err := budget.Observe(t.Context())
					if err != nil {
						t.Fatal(err)
					}
					ctx, cancel := context.WithCancel(t.Context())
					defer cancel()
					cleanupFailure := errors.New("original reservation late close failure")
					reserve := func() (func() error, error) {
						release, err := networkTestReservation(t, budget, deadline)
						if err != nil {
							return release, err
						}
						close(entered)
						<-resume
						return func() error { close(releasing); <-releaseJoin; return errors.Join(release(), cleanupFailure) }, nil
					}
					hold.Store(true)
					attempt := make(chan error, 1)
					go func() {
						var g receiving.Grant
						var err error
						if operation == "refill" {
							g, err = owner.Refill(ctx, prior, 1, tokens[1], reserve)
						} else {
							g, err = owner.Accept(ctx, admission.ForwardClass, tokens[1], deadline, reserve)
						}
						if g.Allowance().Bytes() != 0 {
							t.Error("late original result transferred a Grant")
							err = errors.Join(err, g.Release())
						}
						attempt <- err
					}()
					select {
					case <-entered:
					case err := <-attempt:
						t.Fatal("genuine held boundary was not reached", err)
					}
					held, err := budget.Observe(t.Context())
					if err != nil || boundary == "reservation" && held.ReservedBytes <= baseline.ReservedBytes || boundary == "observation" && held.ReservedBytes != baseline.ReservedBytes {
						t.Fatal("Hosting effect boundary differs", held, baseline, err)
					}
					closed := make(chan error, 1)
					probeCtx, probeCancel := context.WithTimeout(t.Context(), time.Second)
					defer probeCancel()
					if denial == "cancel" {
						cancel()
					} else {
						go func() { closed <- owner.Close() }()
					}
					if denial == "cancel" {
						probeCancel()
					}
					_, probeErr := owner.Accept(probeCtx, admission.ForwardClass, tokens[2], deadline, func() (func() error, error) { t.Error("denied probe reached Hosting"); return nil, nil })
					if probeErr == nil || denial == "close" && probeCtx.Err() != nil {
						t.Error("local denial waited for foreign work", probeErr)
					}
					hold.Store(false)
					resumeOnce.Do(func() { close(resume) })
					if boundary == "reservation" {
						<-releasing
						// Receiving must retain its spend lease and join the actual
						// reservation callback, including a failure after real release.
						select {
						case err := <-attempt:
							t.Fatal("attempt returned before release joined", err)
						default:
						}
						if denial == "close" {
							select {
							case err := <-closed:
								t.Fatal("Close returned before original release joined", err)
							default:
							}
						}
						retained, err := budget.Observe(t.Context())
						if err != nil || retained.ReservedBytes != held.ReservedBytes {
							t.Error("held original reservation returned early", err)
						}
						binding := spending.Binding{NetworkID: f.receiver.NetworkID, ProfileDigest: f.receiver.ProfileDigest, ReceiverNodeID: f.receiver.NodeID, ReceiverDutyGeneration: f.receiver.DutyGeneration}
						foreign, err := spending.OpenRetained(root, binding)
						if foreign != nil {
							_ = foreign.Close()
						}
						if err == nil {
							t.Error("spend lease released before original callback join")
						}
						releaseOnce.Do(func() { close(releaseJoin) })
					}
					attemptErr := <-attempt
					if attemptErr == nil || denial == "cancel" && !errors.Is(attemptErr, context.Canceled) || boundary == "reservation" && !errors.Is(attemptErr, cleanupFailure) {
						t.Error("original refusal/late failure lost", attemptErr)
					}
					if denial == "close" {
						if err := <-closed; err != nil {
							t.Fatal(err)
						}
					}
					if err := owner.Close(); err != nil {
						t.Fatal(err)
					}
					final, err := budget.Observe(t.Context())
					if err != nil || final.ReservedBytes != baseline.ReservedBytes {
						t.Error("refused attempt changed predecessor capacity", final, baseline, err)
					}
					after, err := os.ReadFile(filepath.Join(root, "closed-token-spends"))
					if err != nil || !bytes.Equal(before, after) {
						t.Error("late original result spent after denial", err)
					}
				})
			}
		}
	}
}
