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
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// Four qualification Readers share the Publisher's four-openings-per-second
// dispatch ceiling. A one-second per-Reader interval, phase-shifted below,
// keeps preparation ahead of the shared delivery pacer. Each Reader may
// overlap four unfinished Route setups, for at most the Publisher's sixteen
// admitted Introduction waiters across the cohort. The shared owner waits
// 300 ms after each delivery result before admitting the next submission.
const (
	qualificationIntroductionInterval   = time.Second
	qualificationReaderSetupParallelism = 4
)

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

type qualificationReaderOpening struct {
	index int
	bound streamqualification.BoundStream
	err   error
}

func openQualificationReaderStreams(
	ctx context.Context,
	count, parallelism int,
	first time.Time,
	interval time.Duration,
	open func(context.Context, int) (streamqualification.BoundStream, error),
) ([]streamqualification.BoundStream, error) {
	if ctx == nil || count < 1 || parallelism < 1 || parallelism > count || interval < 0 || open == nil {
		return nil, errors.New("qualification Reader setup unavailable")
	}
	setup, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan qualificationReaderOpening, count)
	slots := make(chan struct{}, parallelism)
	var openings sync.WaitGroup
	for index := 0; index < count; index++ {
		index := index
		openings.Add(1)
		go func() {
			defer openings.Done()
			due := first
			if !due.IsZero() {
				due = due.Add(time.Duration(index) * interval)
			}
			if err := waitQualificationIntroductionOpening(setup, due); err != nil {
				results <- qualificationReaderOpening{index: index, err: err}
				return
			}
			select {
			case slots <- struct{}{}:
			case <-setup.Done():
				results <- qualificationReaderOpening{index: index, err: setup.Err()}
				return
			}
			defer func() { <-slots }()
			if err := setup.Err(); err != nil {
				results <- qualificationReaderOpening{index: index, err: err}
				return
			}
			bound, err := open(setup, index)
			if err != nil {
				cancel()
			}
			results <- qualificationReaderOpening{index: index, bound: bound, err: err}
		}()
	}
	done := make(chan struct{})
	go func() {
		openings.Wait()
		close(done)
	}()
	streams := make([]streamqualification.BoundStream, count)
	var outcome error
	for received := 0; received < count; received++ {
		result := <-results
		streams[result.index] = result.bound
		outcome = errors.Join(outcome, result.err)
	}
	<-done
	return streams, outcome
}

func (owner *textContext) streamConnectionLimitLocked() int {
	if owner.job != nil && owner.job.qualification != nil {
		schedule, err := owner.job.qualification.Init().Profile.Definition(owner.job.qualification.Init().Role)
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
	worker.qualificationReader = reader
	bounded, finish, err := worker.beginOperation(ctx, broker.Connection)
	if err != nil {
		return report, err
	}
	defer func() { finish(); outcome = errors.Join(outcome, worker.Close()) }()
	owner := worker.job.owner
	var streams []streamqualification.BoundStream
	defer func() {
		for _, bound := range streams {
			if bound.Stream != nil {
				outcome = errors.Join(outcome, bound.Stream.Close())
			}
		}
	}()
	// Setup has its own finite budget; it does not consume the ten-minute useful
	// workload interval and cannot extend the immutable forwarding lease.
	until := owner.endpoint.clock().UTC().Add(15 * time.Minute).Unix()
	verified, err := owner.resolveTextIntroduction(bounded, worker.job, destination)
	if err != nil {
		return report, fmt.Errorf("qualification Reader %d resolution: %w", reader, err)
	}
	// Keep the four independent Readers out of phase. Without this offset they
	// exhaust and replenish their issuer stocks together, turning an otherwise
	// bounded two-openings-per-second workload into a bootstrap refill herd.
	// Reader validation above keeps the four phases evenly distributed within
	// each qualification interval.
	firstOpening := time.Now().Add(qualificationReaderOpeningDelay(reader))
	var ownerWork sync.Mutex
	streams, err = openQualificationReaderStreams(bounded, 64, qualificationReaderSetupParallelism, firstOpening, qualificationIntroductionInterval,
		func(setup context.Context, index int) (bound streamqualification.BoundStream, outcome error) {
			ownerWork.Lock()
			var release sync.Once
			releaseOwner := func() { release.Do(ownerWork.Unlock) }
			defer releaseOwner()
			owner.mu.Lock()
			prefix := owner.currentTextSourceLocked()
			owner.mu.Unlock()
			if prefix == nil {
				return bound, fmt.Errorf("qualification Reader %d stream %d JOIN reserve prefix unavailable", reader, index)
			}
			joinReceiver, _, _, err := prefix.dataJoinRecipient()
			if err != nil {
				return bound, fmt.Errorf("qualification Reader %d stream %d JOIN reserve recipient: %w", reader, index, err)
			}
			if err := owner.ensureQualificationTokenReserve(setup, joinReceiver, 2, qualificationReaderSetupParallelism); err != nil {
				return bound, fmt.Errorf("qualification Reader %d stream %d JOIN reserve: %w", reader, index, err)
			}
			submissionReceiver, err := prefix.submissionRecipient()
			if err != nil {
				return bound, fmt.Errorf("qualification Reader %d stream %d submission reserve recipient: %w", reader, index, err)
			}
			if err := owner.ensureQualificationTokenReserve(setup, submissionReceiver, 1, qualificationReaderSetupParallelism); err != nil {
				return bound, fmt.Errorf("qualification Reader %d stream %d submission reserve: %w", reader, index, err)
			}
			releaseSetup := func() {}
			if releaseSetup == nil {
				releaseSetup, err = worker.job.qualification.AcquireSetup(setup)
				if err != nil {
					return bound, fmt.Errorf("qualification Reader %d stream %d setup admission: %w", reader, index, err)
				}
			}
			defer releaseSetup()
			// Token issuance and admission waits can take seconds on the shaped
			// five-Node Route. Complete them before sealing the Introduction: its
			// ten-second wire lifetime belongs only to delivery and JOIN.
			attempt, err := owner.prepareResolvedTextIntroduction(setup, worker.job, destination, [3]int64{until, until, until}, verified)
			if err != nil {
				return bound, fmt.Errorf("qualification Reader %d stream %d preparation: %w", reader, index, err)
			}
			// The helper's setup context stops sibling preparation after an error.
			// The returned Service Connection belongs to the enclosing Reader
			// operation and must therefore retain that operation's lifetime.
			service, err := owner.openTextJoinedServiceAfterSetup(bounded, worker.job, attempt, releaseOwner)
			if err != nil {
				return bound, fmt.Errorf("qualification Reader %d stream %d join: %w", reader, index, err)
			}
			releaseSetup()
			id := qualificationStreamID(reader, index)
			bound = streamqualification.BoundStream{ID: id, Stream: service}
			if _, err := service.Write(qualificationHello(worker.job.qualification.Init().Profile, worker.job.qualification.Init().Seed, id)); err != nil {
				return bound, fmt.Errorf("qualification Reader %d stream %d hello: %w", reader, index, err)
			}
			// Setup traffic consumes the same finite forwarding allowances as the
			// measured workload. Refill at this completed-operation boundary so the
			// initial 32 MiB authority cannot expire before the retained set exists.
			ownerWork.Lock()
			defer ownerWork.Unlock()
			if err := worker.replenishStreams(setup); err != nil {
				return bound, fmt.Errorf("qualification Reader %d stream %d replenishment: %w", reader, index, err)
			}
			return bound, nil
		})
	if err != nil {
		return report, err
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
			expected := qualificationHello(worker.job.qualification.Init().Profile, worker.job.qualification.Init().Seed, id)
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
	worker.job.qualification.PublishReport(report)
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
