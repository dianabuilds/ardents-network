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

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
)

type initialFlightTransport struct {
	*bytes.Reader
	reads, chunk int
	failure      error
}

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
				flight := initialFlightReader(setupInterruptedFlight{physical: physical, failure: failure.err}, wire.InitialResponseBytes)
				record, err := flight.readSetupRecord(setup)
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
		flight := initialFlightReader(initialSetupReadFunc(func([]byte) (int, error) { return 0, failure }), wire.InitialResponseBytes)
		record, err := flight.readSetupRecord(t.Context())
		if !errors.Is(err, failure) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, errIntegrity) || record.Proof != nil {
			t.Fatal("live setup changed an independent failure into cancellation, expiry or integrity", failure, err)
		}
	}
	// A fully observed invalid prefix remains a structural contradiction.
	malformed := append([]byte("ardents-service-connection-v3\x01"), 0, 0)
	flight := initialFlightReader(bytes.NewReader(malformed), wire.InitialResponseBytes)
	if _, err := flight.readSetupRecord(t.Context()); !errors.Is(err, wire.ErrInvalidRecord) || !errors.Is(err, errIntegrity) {
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
	flight := initialFlightReader(initialSetupReadFunc(func(body []byte) (int, error) {
		n, err := input.Read(body)
		cancel()
		return n, err
	}), wire.InitialResponseBytes)
	if record, err := flight.readSetupRecord(ctx); !errors.Is(err, context.Canceled) || errors.Is(err, errIntegrity) || record.Proof != nil {
		t.Fatal("late complete proof survived its original caller", record, err)
	}
	reads := 0
	flight = initialFlightReader(initialSetupReadFunc(func([]byte) (int, error) {
		reads++
		return 0, io.EOF
	}), wire.InitialResponseBytes)
	if _, err := flight.readSetupRecord(ctx); !errors.Is(err, context.Canceled) || reads != 0 {
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
		flight := initialFlightReader(initialSetupReadFunc(func([]byte) (int, error) {
			<-setup.Done()
			<-release
			return 0, physicalFailure
		}), wire.InitialResponseBytes)
		go func() {
			_, err := flight.readSetupRecord(setup)
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

func (r *initialFlightTransport) Read(body []byte) (int, error) {
	r.reads++
	if r.chunk > 0 && len(body) > r.chunk {
		body = body[:r.chunk]
	}
	n, err := r.Reader.Read(body)
	return n, errors.Join(err, r.failure)
}

// These are grammar-only flights, not authenticated Instance/Connection facts.
// The real production reader must not consume the following stream's bytes.
func TestInitialFlightReaderPreservesFollowingApplicationBytes(t *testing.T) {
	for _, response := range []bool{false, true} {
		first := wire.Record{Challenge: &wire.Challenge{Network: [32]byte{1}, Target: [32]byte{2},
			InstanceGeneration: 1, Context: [32]byte{3}, Nonce: [32]byte{4}}}
		role, size, independentSize := wire.RoleInitiator, wire.InitialRequestBytes, 443
		if response {
			first = wire.Record{Proof: &wire.Proof{ChallengeDigest: [32]byte{1}, Signature: [64]byte{2}}}
			role, size, independentSize = wire.RolePublisher, wire.InitialResponseBytes, 403
		}
		continuity, err := wire.NewContinuity([32]byte{1}, role, 1, 0, 0, 0, [32]byte{2}, [32]byte{3})
		if err != nil {
			t.Fatal(err)
		}
		var flight bytes.Buffer
		if err := wire.Write(&flight, first); err != nil {
			t.Fatal(err)
		}
		if err := wire.Write(&flight, wire.Record{Continuity: &continuity}); err != nil {
			t.Fatal(err)
		}
		if size != independentSize || flight.Len() != independentSize {
			t.Fatal("fixed v3 flight length changed", response, size, flight.Len())
		}
		for _, chunk := range []int{0, 7} {
			tail := []byte("following Application record")
			transport := &initialFlightTransport{Reader: bytes.NewReader(append(append([]byte(nil), flight.Bytes()...), tail...)), chunk: chunk}
			reader := initialFlightReader(transport, size)
			got, err := reader.readRecord()
			if err != nil || response && got.Proof == nil || !response && got.Challenge == nil {
				t.Fatal("first record differs", err)
			}
			got, err = reader.readRecord()
			if err != nil || got.Continuity == nil || *got.Continuity != continuity {
				t.Fatal("second record differs", err)
			}
			if chunk == 0 && transport.reads != 1 {
				t.Fatal("coalesced flight repeated transport observation", transport.reads)
			}
			if transport.Len() != len(tail) {
				t.Fatal("authentication consumed following Application bytes")
			}
			if _, err := reader.buffered.ReadByte(); !errors.Is(err, io.EOF) || transport.Len() != len(tail) {
				t.Fatal("flight limit allowed later bytes", err)
			}
		}
		truncated := initialFlightReader(bytes.NewReader(flight.Bytes()[:flight.Len()-1]), size)
		if _, err := truncated.readRecord(); err != nil {
			t.Fatal(err)
		}
		if _, err := truncated.readRecord(); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatal("truncated coalesced flight was not refused", err)
		}
		failure := errors.New("original transport refused with a complete flight")
		failed := initialFlightReader(&initialFlightTransport{Reader: bytes.NewReader(flight.Bytes()), failure: failure}, size)
		if record, err := failed.readRecord(); !errors.Is(err, failure) || record.Challenge != nil || record.Proof != nil {
			t.Fatal("buffering hid the failed original read", err)
		}
	}
}
