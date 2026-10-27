package receiving

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
)

// These are Receiving boundary tests with genuine verification and spend
// storage. Supplied authority facts and held reservation callbacks establish
// neither signed Network authority nor actual Hosting/native completion.
func TestReceivingHeldForeignWorkDeniesBeforeJoin(t *testing.T) {
	for _, boundary := range []string{"observation", "reservation"} {
		t.Run(boundary, func(t *testing.T) {
			observation, tokens := receivingLifetimeTokens(t)
			raw := tokens[0]
			entered, resume := make(chan struct{}), make(chan struct{})
			var unblock sync.Once
			defer unblock.Do(func() { close(resume) })
			root := t.TempDir()
			calls := 0
			owner, err := Open(root, observation.Receiver, func() (Observation, error) {
				calls++
				if boundary == "observation" && calls == 2 {
					close(entered)
					<-resume
				}
				return observation, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = owner.Close() })
			before, err := os.ReadFile(filepath.Join(root, "closed-token-spends"))
			if err != nil {
				t.Fatal(err)
			}
			cleanup := errors.New("original reservation close failed")
			released := 0
			attempt := make(chan error, 1)
			go func() {
				g, err := owner.Accept(t.Context(), admission.ForwardClass, raw, observation.Now.Add(time.Minute), func() (func() error, error) {
					if boundary == "reservation" {
						close(entered)
						<-resume
					}
					return func() error { released++; return cleanup }, nil
				})
				if g.state != nil {
					err = errors.Join(err, errors.New("late grant transferred"), g.Release())
				}
				attempt <- err
			}()
			select {
			case <-entered:
			case err := <-attempt:
				t.Fatal("original admission refused before held boundary", err)
			case <-time.After(3 * time.Second):
				t.Fatal("original admission did not reach held boundary")
			}
			// A queued canceled attempt must not wait for foreign work or
			// consume a verification slot, capacity or a token.
			ctx, cancel := context.WithCancel(t.Context())
			queued := make(chan error, 1)
			go func() {
				_, err := owner.Accept(ctx, admission.ForwardClass, raw, observation.Now.Add(time.Minute), func() (func() error, error) { return nil, errors.New("queued reservation reached") })
				queued <- err
			}()
			cancel()
			queuedPrompt := false
			queuedJoined := false
			select {
			case err := <-queued:
				queuedJoined = true
				queuedPrompt = errors.Is(err, context.Canceled)
			case <-time.After(time.Second):
			}
			closed := make(chan error, 1)
			go func() { closed <- owner.Close() }()
			// The local denial latch must be observable while the original
			// foreign callback remains held; Close must still retain custody.
			denied := false
			limit := time.NewTimer(time.Second)
			tick := time.NewTicker(time.Millisecond)
		waiting:
			for {
				if owner.mu.TryLock() {
					denied = owner.closed
					owner.mu.Unlock()
					if denied {
						break
					}
				}
				select {
				case <-limit.C:
					break waiting
				case <-tick.C:
				}
			}
			limit.Stop()
			tick.Stop()
			var closeErr error
			closeJoined := false
			select {
			case err := <-closed:
				closeErr, closeJoined = err, true
				t.Errorf("Close returned before original join: %v", err)
			default:
			}
			unblock.Do(func() { close(resume) })
			attemptErr := <-attempt
			if !closeJoined {
				closeErr = <-closed
			}
			if !queuedPrompt {
				if !queuedJoined {
					<-queued
				}
				t.Error("canceled queued admission waited for foreign callback")
			}
			if !denied {
				t.Error("held foreign callback prevented synchronous receiving denial")
			}
			if attemptErr == nil || closeErr != nil {
				t.Errorf("late attempt / joined close: %v / %v", attemptErr, closeErr)
			}
			wantReleased := 0
			if boundary == "reservation" {
				wantReleased = 1
				if !errors.Is(attemptErr, cleanup) {
					t.Error("late cleanup cause lost", attemptErr)
				}
			}
			if released != wantReleased {
				t.Errorf("reservation releases %d, want %d", released, wantReleased)
			}
			after, err := os.ReadFile(filepath.Join(root, "closed-token-spends"))
			if err != nil || !bytes.Equal(before, after) {
				t.Error("late foreign result spent after denial", err)
			}
		})
	}
}

func TestReceivingOriginalAttemptCancellationAndRefill(t *testing.T) {
	observation, tokens := receivingLifetimeTokens(t)
	for _, operation := range []string{"accept", "refill"} {
		for _, boundary := range []string{"initial-observation", "reservation", "commit-observation", "spent-observation"} {
			for _, denial := range []string{"cancel", "close"} {
				t.Run(operation+"/"+boundary+"/"+denial, func(t *testing.T) {
					root := t.TempDir()
					entered, resume := make(chan struct{}), make(chan struct{})
					var unhold sync.Once
					defer unhold.Do(func() { close(resume) })
					calls, holdAt := 0, 0
					owner, err := Open(root, observation.Receiver, func() (Observation, error) {
						calls++
						if calls == holdAt {
							close(entered)
							<-resume
						}
						return observation, nil
					})
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = owner.Close() })
					deadline := observation.Now.Add(time.Minute)
					var prior Grant
					priorReleased := 0
					if operation == "refill" {
						prior, err = owner.Accept(t.Context(), admission.ForwardClass, tokens[0], deadline, func() (func() error, error) { return func() error { priorReleased++; return nil }, nil })
						if err != nil {
							t.Fatal(err)
						}
						defer prior.Release()
					}
					switch boundary {
					case "initial-observation":
						holdAt = calls + 1
					case "commit-observation":
						holdAt = calls + 2
					case "spent-observation":
						holdAt = calls + 3
					}
					ctx, cancel := context.WithCancel(t.Context())
					defer cancel()
					cleanup := errors.New("held original reservation failed to close")
					releases, reservations := 0, 0
					reserve := func() (func() error, error) {
						reservations++
						if boundary == "reservation" {
							close(entered)
							<-resume
						}
						return func() error { releases++; return cleanup }, nil
					}
					finished := make(chan error, 1)
					go func() {
						var g Grant
						var err error
						if operation == "refill" {
							g, err = owner.Refill(ctx, prior, 1, tokens[1], reserve)
						} else {
							g, err = owner.Accept(ctx, admission.ForwardClass, tokens[1], deadline, reserve)
						}
						if g.state != nil {
							err = errors.Join(errors.New("late Grant transferred"), g.Release())
							t.Error("denied original attempt transferred a Grant")
						}
						finished <- err
					}()
					select {
					case <-entered:
					case err := <-finished:
						t.Fatal("original attempt refused before held boundary", err)
					case <-time.After(3 * time.Second):
						t.Fatal("original attempt did not reach held boundary")
					}
					closed := make(chan error, 1)
					if denial == "cancel" {
						cancel()
					} else {
						go func() { closed <- owner.Close() }()
						select {
						case <-owner.denied:
						case <-time.After(time.Second):
							t.Error("Close could not deny held original attempt")
						}
					}
					// A held callback remains an original user; denial is not join.
					select {
					case <-finished:
						t.Fatal("held original callback was not joined")
					default:
					}
					unhold.Do(func() { close(resume) })
					attemptErr := <-finished
					if attemptErr == nil || denial == "cancel" && !errors.Is(attemptErr, context.Canceled) {
						t.Error("original refusal cause lost", attemptErr)
					}
					if boundary == "initial-observation" {
						if reservations != 0 || releases != 0 {
							t.Error("late canceled/closed observation acquired capacity")
						}
					} else if reservations != 1 || releases != 1 || !errors.Is(attemptErr, cleanup) {
						t.Errorf("original reservation cleanup: %d/%d %v", reservations, releases, attemptErr)
					}
					if denial == "close" {
						if err := <-closed; err != nil {
							t.Fatal(err)
						}
					}
					if err := owner.Close(); err != nil {
						t.Fatal(err)
					}
					if priorReleased != 0 {
						t.Error("Receiving released a previously transferred reservation")
					}
					// Reopen the actual original spend history: only the attempt
					// held after durable I/O must remain burned after refusal.
					binding := spending.Binding{NetworkID: observation.Receiver.NetworkID, ProfileDigest: observation.Receiver.ProfileDigest, ReceiverNodeID: observation.Receiver.NodeID, ReceiverDutyGeneration: observation.Receiver.DutyGeneration}
					ledger, err := spending.OpenRetained(root, binding)
					if err != nil {
						t.Fatal(err)
					}
					defer ledger.Close()
					err = ledger.Spend(tokens[1], observation.Now.Truncate(time.Hour), observation.Now)
					if (err != nil) != (boundary == "spent-observation") {
						t.Error("retained burn differs from actual effect boundary", err)
					}
				})
			}
		}
	}
}

func TestReceivingRefillRetainsDeadlineAndExactlyOnceTransfer(t *testing.T) {
	observation, tokens := receivingLifetimeTokens(t)
	owner, err := Open(t.TempDir(), observation.Receiver, func() (Observation, error) { return observation, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	releases := 0
	reserve := func() (func() error, error) { return func() error { releases++; return nil }, nil }
	first, err := owner.Accept(t.Context(), admission.ForwardClass, tokens[0], observation.Now.Add(time.Minute), reserve)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	refill, err := owner.Refill(t.Context(), first, 1, tokens[1], reserve)
	if err != nil {
		t.Fatal(err)
	}
	defer refill.Release()
	if refill.Allowance().Deadline() != first.Allowance().Deadline() || refill.Allowance().Bytes() != admission.ForwardClass.ByteLimit() {
		t.Fatal("refill changed original allowance contract")
	}
	if _, err := owner.Refill(t.Context(), first, 1, tokens[1], reserve); err == nil || releases != 1 {
		t.Fatal("duplicate spend accepted or reservation lost", err, releases)
	}
	if err := owner.Close(); err != nil || releases != 1 {
		t.Fatal("Close returned accepted reservation", err, releases)
	}
	copy := refill
	_ = refill.Release()
	_ = copy.Release()
	_ = first.Release()
	if releases != 3 {
		t.Fatal("reservation ownership was multiplied", releases)
	}
}

func TestReceivingCloseDuringVerificationDeniesReservation(t *testing.T) {
	observation, tokens := receivingLifetimeTokens(t)
	root := t.TempDir()
	calls := 0
	wanted := observation.Now.Add(time.Second)
	owner, err := Open(root, observation.Receiver, func() (Observation, error) {
		calls++
		v := observation
		if calls == 2 {
			v.Now = wanted
		}
		return v, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	before, err := os.ReadFile(filepath.Join(root, "closed-token-spends"))
	if err != nil {
		t.Fatal(err)
	}
	owner.gate.mu.Lock()
	locked := true
	defer func() {
		if locked {
			owner.gate.mu.Unlock()
		}
	}()
	reserved := false
	finished := make(chan error, 1)
	go func() {
		g, err := owner.Accept(t.Context(), admission.ForwardClass, tokens[0], observation.Now.Add(time.Minute), func() (func() error, error) { reserved = true; return nil, nil })
		if g.state != nil {
			t.Error("retired verification returned Grant")
			_ = g.Release()
		}
		finished <- err
	}()
	// The changed floor proves that the successful original observation has
	// committed and the operation is blocked at VerificationGate, not observe.
	limit := time.NewTimer(3 * time.Second)
	defer limit.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		owner.mu.Lock()
		observed := owner.floor.Equal(wanted)
		owner.mu.Unlock()
		if observed {
			break
		}
		select {
		case err := <-finished:
			t.Fatal("verification boundary not reached", err)
		case <-limit.C:
			t.Fatal("original observation did not commit")
		case <-tick.C:
		}
	}
	closed := make(chan error, 1)
	go func() { closed <- owner.Close() }()
	select {
	case <-owner.denied:
	case <-limit.C:
		t.Fatal("Close did not seal admission")
	}
	owner.gate.mu.Unlock()
	locked = false
	if err := <-finished; err == nil {
		t.Error("denied verification was accepted")
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	if reserved {
		t.Error("verification admitted a new reservation after Close denial")
	}
	after, err := os.ReadFile(filepath.Join(root, "closed-token-spends"))
	if err != nil || !bytes.Equal(before, after) {
		t.Error("denied verification spent", err)
	}
}

func receivingLifetimeTokens(t *testing.T) (Observation, [][]byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	spki, err := encodeSelectedRSAPSSSPKI(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	window := time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour)
	receiver := Receiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4}, NodeID: [32]byte{5}, DutyGeneration: 6}
	profile := admission.AuthorityFacts{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, Digest: receiver.ProfileDigest, IssuerNodeID: [32]byte{7}, IssuanceAuthorityKey: [32]byte{8}, IssuerDutyGeneration: 9, NotBefore: window, NotAfter: window.Add(time.Hour), TokenKeyCount: 3}
	for i := range 3 {
		public := spki
		if i != 1 {
			other, err := rsa.GenerateKey(rand.Reader, 2048)
			if err != nil {
				t.Fatal(err)
			}
			public, err = encodeSelectedRSAPSSSPKI(&other.PublicKey)
			if err != nil {
				t.Fatal(err)
			}
		}
		profile.TokenKeys[i] = admission.TokenKey{WindowStart: window, Class: uint8(i + 1)}
		copy(profile.TokenKeys[i].SPKI[:], public)
	}
	binding := token.ClosedTokenContext{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, IssuerNodeID: profile.IssuerNodeID, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration, Class: 2, WindowStart: window}
	var tokens [][]byte
	for n := byte(1); n <= 3; n++ {
		_, input, _, err := token.ClosedTokenChallenge(binding, spki, [32]byte{n})
		if err != nil {
			t.Fatal(err)
		}
		digest := sha512.Sum384(input)
		signature, err := rsa.SignPSS(rand.Reader, key, crypto.SHA384, digest[:], &rsa.PSSOptions{SaltLength: crypto.SHA384.Size(), Hash: crypto.SHA384})
		if err != nil {
			t.Fatal(err)
		}
		tokens = append(tokens, append(input, signature...))
	}
	return Observation{Profile: profile, Receiver: receiver, Now: window.Add(time.Minute), NotAfter: profile.NotAfter}, tokens
}
