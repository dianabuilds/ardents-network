package connection

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/recordio"
	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

// This is the failed authentication-record boundary, not successful Service
// authority. The installed reproduction separately supplies real original
// Startup, Job, JOIN and TLS owners. Virtual time preserves their 60s/30s bounds
// while the same setup callback interrupts the physical reader.
func TestInitialSetupReadRetainsDeadlineBeforeAttachmentCancellation(t *testing.T) {
	for _, failure := range []struct {
		name string
		err  error
	}{{"original-io-timeout", os.ErrDeadlineExceeded}, {"cancellation-only", nil}} {
		t.Run(failure.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				original, cancelOriginal := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancelOriginal()
				setup, cancelSetup := context.WithTimeout(original, 30*time.Second)
				defer cancelSetup()
				logical, cancelLogical, physical, cancelPhysical := initiatorLifetimes(original, time.Now().Add(60*time.Second))
				defer cancelLogical()
				defer cancelPhysical()
				callbackDone := make(chan struct{})
				stop := context.AfterFunc(setup, func() {
					defer close(callbackDone)
					cancelLogical()
				})
				defer func() {
					if !stop() {
						<-callbackDone
					}
				}()
				flight := recordio.NewInitialResponseReader(setupInterruptedFlight{physical: physical, failure: failure.err})
				record, err := readInitialResponseRecord(setup, flight)
				if original.Err() != nil || logical.Err() != context.Canceled || setup.Err() != context.DeadlineExceeded {
					t.Fatal("fixture lost the original caller/setup/physical distinction", original.Err(), setup.Err(), logical.Err())
				}
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatal("setup deadline became only Attachment cancellation", err)
				}
				if failure.err != nil && !errors.Is(err, failure.err) || !errors.Is(err, context.Canceled) || record.Proof != nil || record.Continuity != nil {
					t.Fatal("failed original read or refusal was lost", record, err)
				}
			})
		})
	}
}

type setupInterruptedFlight struct {
	physical context.Context
	failure  error
}

func (reader setupInterruptedFlight) Read([]byte) (int, error) {
	<-reader.physical.Done()
	return 0, errors.Join(reader.failure, reader.physical.Err())
}

func TestInitialSetupReadKeepsIndependentPhysicalAndGrammarFailures(t *testing.T) {
	authorityLoss := errors.New("independent current authority loss")
	for _, failure := range []error{io.EOF, io.ErrUnexpectedEOF, io.ErrClosedPipe, authorityLoss, errors.Join(io.ErrClosedPipe, authorityLoss)} {
		flight := recordio.NewInitialResponseReader(initialSetupReadFunc(func([]byte) (int, error) { return 0, failure }))
		record, err := readInitialResponseRecord(t.Context(), flight)
		if !errors.Is(err, failure) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, errIntegrity) || record.Proof != nil {
			t.Fatal("live setup changed an independent failure into cancellation, expiry or integrity", failure, err)
		}
	}
	// A fully observed invalid prefix remains a structural contradiction.
	malformed := append([]byte("ardents-service-connection-v3\x01"), 0, 0)
	flight := recordio.NewInitialResponseReader(bytes.NewReader(malformed))
	if _, err := readInitialResponseRecord(t.Context(), flight); !errors.Is(err, wire.ErrInvalidRecord) || !errors.Is(err, errIntegrity) {
		t.Fatal("complete malformed record lost its integrity failure", err)
	}
}

func TestInitialSetupReadRefusesLateCompleteProofAndAlreadyCanceledCaller(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	var encoded bytes.Buffer
	if err := wire.Write(&encoded, wire.Record{Proof: &wire.Proof{ChallengeDigest: [32]byte{1}, Signature: [64]byte{2}}}); err != nil {
		t.Fatal(err)
	}
	input := bytes.NewReader(encoded.Bytes())
	flight := recordio.NewInitialResponseReader(initialSetupReadFunc(func(body []byte) (int, error) {
		n, err := input.Read(body)
		cancel()
		return n, err
	}))
	if record, err := readInitialResponseRecord(ctx, flight); !errors.Is(err, context.Canceled) || errors.Is(err, errIntegrity) || record.Proof != nil {
		t.Fatal("late complete proof survived its original caller", record, err)
	}
	reads := 0
	flight = recordio.NewInitialResponseReader(initialSetupReadFunc(func([]byte) (int, error) {
		reads++
		return 0, io.EOF
	}))
	if _, err := readInitialResponseRecord(ctx, flight); !errors.Is(err, context.Canceled) || reads != 0 {
		t.Fatal("already canceled setup performed physical input", reads, err)
	}
}

func TestInitialSetupReadEarlierOriginalDeadlineAndLatePhysicalFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		original, cancelOriginal := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelOriginal()
		setup, cancelSetup := context.WithTimeout(original, 30*time.Second)
		defer cancelSetup()
		physicalFailure := errors.New("original late read failure")
		release := make(chan struct{})
		done := make(chan error, 1)
		flight := recordio.NewInitialResponseReader(initialSetupReadFunc(func([]byte) (int, error) {
			<-setup.Done()
			<-release
			return 0, physicalFailure
		}))
		go func() {
			_, err := readInitialResponseRecord(setup, flight)
			done <- err
		}()
		time.Sleep(10 * time.Second)
		synctest.Wait()
		select {
		case err := <-done:
			t.Fatal("deadline was treated as joined original input", err)
		default:
		}
		close(release)
		err := <-done
		if original.Err() != context.DeadlineExceeded || !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, physicalFailure) || errors.Is(err, context.Canceled) {
			t.Fatal("earlier original deadline or late physical failure was changed", original.Err(), err)
		}
	})
}

type initialSetupReadFunc func([]byte) (int, error)

func (read initialSetupReadFunc) Read(body []byte) (int, error) { return read(body) }
