//go:build linux

package endpoint

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// Four qualification Readers share the Publisher's four-openings-per-second
// dispatch ceiling. A 1.25-second per-Reader interval, phase-shifted below,
// yields 3.2 openings per second and completes the 64-opening setup in about
// 80 seconds, within the retained Source lifetime. Forwarding byte authority
// is replenished from actual accounted traffic rather than reduced opening
// frequency.
const qualificationIntroductionInterval = 1250 * time.Millisecond

func qualificationReaderOpeningDelay(reader int) time.Duration {
	return time.Duration(reader) * qualificationIntroductionInterval / 4
}

func waitQualificationIntroductionOpening(ctx context.Context, next time.Time) error {
	if wait := time.Until(next); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (owner *textContext) streamConnectionLimitLocked() int {
	if owner.job != nil && owner.job.qualification != nil {
		schedule, err := owner.job.qualification.Profile.Definition(owner.job.qualification.Role)
		if err == nil {
			return int(schedule.OpenConnections)
		}
	}
	return 16
}

func (owner *textContext) streamExchangeLimitLocked() int {
	if owner.job != nil && owner.job.qualification != nil {
		// Retained transports plus finite simultaneous introduction/recovery work.
		return owner.streamConnectionLimitLocked() + 16
	}
	return 16
}

func qualificationStreamID(reader, index int) uint32 {
	if index < 16 {
		return uint32(reader*32 + index*2 + 1)
	}
	return uint32(129 + reader*96 + (index-16)*2)
}

func qualificationHello(profile streamqualification.Profile, seed [32]byte, id uint32) []byte {
	body := make([]byte, 45)
	copy(body, "ARDTQS01")
	body[8] = byte(profile)
	binary.BigEndian.PutUint32(body[9:13], id)
	digest := sha256.Sum256(seed[:])
	copy(body[13:], digest[:])
	return body
}

func (worker *qualifiedTextWorker) runQualificationReader(ctx context.Context, destination targetlink.Link, reader int) (report streamqualification.Report, outcome error) {
	if worker == nil || worker.job == nil || worker.job.qualification == nil || reader < 0 || reader >= 4 {
		return report, errors.New("qualification Reader unavailable")
	}
	bounded, finish, err := worker.beginOperation(ctx, broker.Connection)
	if err != nil {
		return report, err
	}
	defer func() { finish(); outcome = errors.Join(outcome, worker.Close()) }()
	owner := worker.job.owner
	var streams []streamqualification.BoundStream
	defer func() {
		for _, bound := range streams {
			outcome = errors.Join(outcome, bound.Stream.Close())
		}
	}()
	// Setup has its own finite budget; it does not consume the ten-minute useful
	// workload interval and cannot extend the immutable forwarding lease.
	until := owner.endpoint.clock().UTC().Add(15 * time.Minute).Unix()
	// Keep the four independent Readers out of phase. Without this offset they
	// exhaust and replenish their issuer stocks together, turning an otherwise
	// bounded two-openings-per-second workload into a bootstrap refill herd.
	// Reader validation above keeps the four phases evenly distributed within
	// each qualification interval.
	nextOpening := time.Now().Add(qualificationReaderOpeningDelay(reader))
	for index := 0; index < 64; index++ {
		// Four independent Readers share the Publisher's four-per-second
		// cryptographic-opening allowance. A per-Reader interval greater than one
		// second bounds every sliding second to at most one opening per Reader,
		// without weakening the Publisher's hostile-input rate limit.
		if err := waitQualificationIntroductionOpening(bounded, nextOpening); err != nil {
			return report, fmt.Errorf("qualification Reader %d stream %d pacing: %w", reader, index, err)
		}
		attempt, err := owner.prepareTextIntroduction(bounded, worker.job, destination, [3]int64{until, until, until})
		if err != nil {
			return report, fmt.Errorf("qualification Reader %d stream %d preparation: %w", reader, index, err)
		}
		service, err := owner.openTextJoinedService(bounded, worker.job, attempt)
		if err != nil {
			return report, fmt.Errorf("qualification Reader %d stream %d join: %w", reader, index, err)
		}
		id := qualificationStreamID(reader, index)
		streams = append(streams, streamqualification.BoundStream{ID: id, Stream: service})
		if _, err := service.Write(qualificationHello(worker.job.qualification.Profile, worker.job.qualification.Seed, id)); err != nil {
			return report, fmt.Errorf("qualification Reader %d stream %d hello: %w", reader, index, err)
		}
		// Setup traffic consumes the same finite forwarding allowances as the
		// measured workload. Refill at this completed-operation boundary so the
		// initial 32 MiB authority cannot expire before the retained set exists.
		if err := worker.replenishStreams(bounded); err != nil {
			return report, fmt.Errorf("qualification Reader %d stream %d replenishment: %w", reader, index, err)
		}
		nextOpening = time.Now().Add(qualificationIntroductionInterval)
	}
	for _, bound := range streams {
		var ready [1]byte
		if _, err := io.ReadFull(bound.Stream, ready[:]); err != nil || ready[0] != 1 {
			return report, errors.Join(err, errors.New("qualification Publisher set not ready"))
		}
	}
	return worker.runQualifiedStreams(bounded, streams)
}

func (worker *qualifiedTextWorker) serveQualification(ctx, bounded context.Context, finish func(), produce func(context.Context, chan<- connection.Stream) error) (outcome error) {
	network, cancel := context.WithCancel(bounded)
	delivered := make(chan connection.Stream)
	produced := make(chan error, 1)
	go func() { err := produce(network, delivered); close(delivered); produced <- err }()
	var streams []streamqualification.BoundStream
	defer func() {
		cancel()
		for _, bound := range streams {
			outcome = errors.Join(outcome, bound.Stream.Close())
		}
		producerErr := <-produced
		if !qualificationCancellationOnly(producerErr) || ctx.Err() != nil {
			outcome = errors.Join(outcome, producerErr)
		}
		finish()
		outcome = errors.Join(outcome, worker.Close())
	}()
	seen := make(map[uint32]bool, 256)
	for len(streams) < 256 {
		select {
		case stream, ok := <-delivered:
			if !ok {
				return qualificationPublisherIncompleteError(len(streams))
			}
			// A connected peer must supply the fixed workload binding promptly.
			helloCtx, stop := context.WithTimeout(network, 10*time.Second)
			stopped := make(chan struct{})
			interrupt := context.AfterFunc(helloCtx, func() { defer close(stopped); _ = stream.Close() })
			var hello [45]byte
			_, readErr := io.ReadFull(stream, hello[:])
			if !interrupt() {
				<-stopped
			}
			stop()
			if readErr != nil {
				return errors.Join(readErr, stream.Close())
			}
			id := binary.BigEndian.Uint32(hello[9:13])
			expected := qualificationHello(worker.job.qualification.Profile, worker.job.qualification.Seed, id)
			if id == 0 || id > 511 || id%2 != 1 || seen[id] || string(hello[:]) != string(expected) {
				return errors.Join(errors.New("qualification workload binding invalid"), stream.Close())
			}
			seen[id] = true
			streams = append(streams, streamqualification.BoundStream{ID: id, Stream: stream})
			if err := worker.replenishStreams(network); err != nil {
				return err
			}
		case <-network.Done():
			return network.Err()
		}
	}
	sort.Slice(streams, func(i, j int) bool { return streams[i].ID < streams[j].ID })
	for _, bound := range streams {
		if _, err := bound.Stream.Write([]byte{1}); err != nil {
			return err
		}
	}
	report, err := worker.runQualifiedStreams(network, streams)
	if worker.job.qualificationReport != nil {
		*worker.job.qualificationReport = report
	}
	return err
}

func qualificationPublisherIncompleteError(streams int) error {
	return fmt.Errorf("qualification Publisher producer ended at %d/256 streams", streams)
}

func qualificationCancellationOnly(err error) bool {
	if err == nil || err == context.Canceled {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !qualificationCancellationOnly(cause) {
				return false
			}
		}
		return true
	}
	if cause := errors.Unwrap(err); cause != nil {
		return qualificationCancellationOnly(cause)
	}
	return false
}
