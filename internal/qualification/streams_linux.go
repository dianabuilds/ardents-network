//go:build linux

package qualification

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

	"github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// Four qualification Readers share the Publisher's four-openings-per-second
// dispatch ceiling. A one-second per-Reader interval, phase-shifted below,
// keeps preparation ahead of the shared delivery pacer. Each Reader may
// overlap four unfinished Route setups, for at most the Publisher's sixteen
// admitted Introduction waiters across the cohort. The shared owner waits
// 300 ms after each delivery result before admitting the next submission.
const (
	IntroductionInterval   = time.Second
	ReaderSetupParallelism = 4
)

// IssuerReserveMinimum keeps issuer admissions outside the next
// Introduction's ten-second wire lifetime. Refilling at the completed-stream
// boundary retains the exact permission and durable-spend rules without
// putting a large refill in the capsule's latency-critical path. Reader
// reserves are deliberately separated by nine tokens, so their
// three-token-per-opening paths do not recreate the refill herd that their
// opening phases avoid.
const IssuerReserveMinimum = 8

// ReaderOpeningDelay phase-shifts the four independent Readers.
func ReaderOpeningDelay(reader int) time.Duration {
	return time.Duration(reader) * IntroductionInterval / 4
}

func waitIntroductionOpening(ctx context.Context, next time.Time) error {
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

type readerOpening struct {
	index int
	bound streamqualification.BoundStream
	err   error
}

// OpenReaderStreams paces count setup openings across at most parallelism
// unfinished Route setups, starting at first and spaced by interval; the
// first open failure cancels the remaining openings.
func OpenReaderStreams(
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
	results := make(chan readerOpening, count)
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
			if err := waitIntroductionOpening(setup, due); err != nil {
				results <- readerOpening{index: index, err: err}
				return
			}
			select {
			case slots <- struct{}{}:
			case <-setup.Done():
				results <- readerOpening{index: index, err: setup.Err()}
				return
			}
			defer func() { <-slots }()
			if err := setup.Err(); err != nil {
				results <- readerOpening{index: index, err: err}
				return
			}
			bound, err := open(setup, index)
			if err != nil {
				cancel()
			}
			results <- readerOpening{index: index, bound: bound, err: err}
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

// StreamID is the fixed workload binding identifier for one Reader stream.
func StreamID(reader, index int) uint32 {
	if index < 16 {
		return uint32(reader*32 + index*2 + 1)
	}
	return uint32(129 + reader*96 + (index-16)*2)
}

// Hello is the fixed workload binding written on one Reader stream.
func Hello(profile streamqualification.Profile, seed [32]byte, id uint32) []byte {
	body := make([]byte, 45)
	copy(body, "ARDTQS01")
	body[8] = byte(profile)
	binary.BigEndian.PutUint32(body[9:13], id)
	digest := sha256.Sum256(seed[:])
	copy(body[13:], digest[:])
	return body
}

func runReader(ctx context.Context, session Session, destination targetlink.Link, reader int) (report streamqualification.Report, outcome error) {
	if session == nil || reader < 0 || reader >= 4 {
		return report, errors.New("qualification Reader unavailable")
	}
	run := session.Run()
	if run == nil {
		return report, errors.New("qualification Reader unavailable")
	}
	bounded, finish, err := session.BeginOperation(ctx)
	if err != nil {
		return report, err
	}
	defer func() { finish(); outcome = errors.Join(outcome, session.Close()) }()
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
	until := session.Now().UTC().Add(15 * time.Minute).Unix()
	verified, err := session.ResolveIntroduction(bounded, destination)
	if err != nil {
		return report, fmt.Errorf("qualification Reader %d resolution: %w", reader, err)
	}
	// Keep the four independent Readers out of phase. Without this offset they
	// exhaust and replenish their issuer stocks together, turning an otherwise
	// bounded two-openings-per-second workload into a bootstrap refill herd.
	// Reader validation above keeps the four phases evenly distributed within
	// each qualification interval.
	firstOpening := time.Now().Add(ReaderOpeningDelay(reader))
	var ownerWork sync.Mutex
	streams, err = OpenReaderStreams(bounded, 64, ReaderSetupParallelism, firstOpening, IntroductionInterval,
		func(setup context.Context, index int) (bound streamqualification.BoundStream, outcome error) {
			ownerWork.Lock()
			var release sync.Once
			releaseOwner := func() { release.Do(ownerWork.Unlock) }
			defer releaseOwner()
			recipients, err := session.ReserveRecipients()
			if err != nil {
				return bound, fmt.Errorf("qualification Reader %d stream %d JOIN reserve prefix unavailable", reader, index)
			}
			if recipients.JoinErr != nil {
				return bound, fmt.Errorf("qualification Reader %d stream %d JOIN reserve recipient: %w", reader, index, recipients.JoinErr)
			}
			if err := session.EnsureTokenReserve(setup, recipients.Join, 2, ReaderSetupParallelism); err != nil {
				return bound, fmt.Errorf("qualification Reader %d stream %d JOIN reserve: %w", reader, index, err)
			}
			if recipients.SubmissionErr != nil {
				return bound, fmt.Errorf("qualification Reader %d stream %d submission reserve recipient: %w", reader, index, recipients.SubmissionErr)
			}
			if err := session.EnsureTokenReserve(setup, recipients.Submission, 1, ReaderSetupParallelism); err != nil {
				return bound, fmt.Errorf("qualification Reader %d stream %d submission reserve: %w", reader, index, err)
			}
			releaseSetup := func() {}
			if releaseSetup == nil {
				releaseSetup, err = run.AcquireSetup(setup)
				if err != nil {
					return bound, fmt.Errorf("qualification Reader %d stream %d setup admission: %w", reader, index, err)
				}
			}
			defer releaseSetup()
			// Token issuance and admission waits can take seconds on the shaped
			// five-Node Route. Complete them before sealing the Introduction: its
			// ten-second wire lifetime belongs only to delivery and JOIN.
			preparation, err := session.PrepareIntroduction(setup, destination, [3]int64{until, until, until}, verified)
			if err != nil {
				return bound, fmt.Errorf("qualification Reader %d stream %d preparation: %w", reader, index, err)
			}
			// The helper's setup context stops sibling preparation after an error.
			// The returned Service Connection belongs to the enclosing Reader
			// operation and must therefore retain that operation's lifetime.
			service, err := session.OpenJoinedService(bounded, preparation, releaseOwner)
			if err != nil {
				return bound, fmt.Errorf("qualification Reader %d stream %d join: %w", reader, index, err)
			}
			releaseSetup()
			id := StreamID(reader, index)
			bound = streamqualification.BoundStream{ID: id, Stream: service}
			if _, err := service.Write(Hello(run.Init().Profile, run.Init().Seed, id)); err != nil {
				return bound, fmt.Errorf("qualification Reader %d stream %d hello: %w", reader, index, err)
			}
			// Setup traffic consumes the same finite forwarding allowances as the
			// measured workload. Refill at this completed-operation boundary so the
			// initial 32 MiB authority cannot expire before the retained set exists.
			ownerWork.Lock()
			defer ownerWork.Unlock()
			if err := ReplenishStreams(setup, session, reader); err != nil {
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
	return runQualifiedStreams(bounded, session, reader, streams)
}

// ServePublisher binds delivered Service streams to the fixed workload and
// runs the retained connection set to its report.
func ServePublisher(ctx, bounded context.Context, worker PublisherWorker, finish func(), produce func(context.Context, chan<- connection.Stream) error) (outcome error) {
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
		if !CancellationOnly(producerErr) || ctx.Err() != nil {
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
				return publisherIncompleteError(len(streams))
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
			expected := Hello(worker.Run().Init().Profile, worker.Run().Init().Seed, id)
			if id == 0 || id > 511 || id%2 != 1 || seen[id] || string(hello[:]) != string(expected) {
				return errors.Join(errors.New("qualification workload binding invalid"), stream.Close())
			}
			seen[id] = true
			streams = append(streams, streamqualification.BoundStream{ID: id, Stream: stream})
			if err := ReplenishStreams(network, worker, 0); err != nil {
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
	report, err := runQualifiedStreams(network, worker, 0, streams)
	worker.Run().PublishReport(report)
	return err
}

func publisherIncompleteError(streams int) error {
	return fmt.Errorf("qualification Publisher producer ended at %d/256 streams", streams)
}

// CancellationOnly reports whether an error tree contains only context
// cancellations.
func CancellationOnly(err error) bool {
	if err == nil || err == context.Canceled {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !CancellationOnly(cause) {
				return false
			}
		}
		return true
	}
	if cause := errors.Unwrap(err); cause != nil {
		return CancellationOnly(cause)
	}
	return false
}

func runQualifiedStreams(ctx context.Context, worker PublisherWorker, reader int, streams []streamqualification.BoundStream) (report streamqualification.Report, outcome error) {
	bounded, cancel := context.WithCancel(ctx)
	stopped := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-bounded.Done():
				stopped <- nil
				return
			case <-ticker.C:
				if err := ReplenishStreams(bounded, worker, reader); err != nil {
					stopped <- err
					cancel()
					return
				}
			}
		}
	}()
	run := worker.Run()
	attachment := NewAttachment(worker.WorkerAttachment(), run.StopSamples)
	report, outcome = streamqualification.RunConnections(bounded, attachment, run.Init(), streams, run.Observe)
	cancel()
	outcome = errors.Join(outcome, <-stopped)
	return report, outcome
}

// ReplenishStreams performs one authorized refill boundary: the live-job
// inspection, the issuer reserve, the prefix replenishments, and the retained
// joins, each rechecking its own authority in Endpoint.
func ReplenishStreams(ctx context.Context, worker PublisherWorker, reader int) error {
	snapshot := worker.RefillSnapshot()
	if !snapshot.Live {
		return errors.New("qualification refill job retired")
	}
	// Busy issuance delays inspection; it cannot spend another token or refill.
	if snapshot.Busy {
		return nil
	}
	run := worker.Run()
	issuerReserve := IssuerReserveMinimum
	if run.Init().Role == streamqualification.ReaderRole {
		issuerReserve += (3 - reader) * 9
	}
	if err := worker.EnsureIssuerReserve(ctx, issuerReserve); err != nil {
		return err
	}
	// Observe the prefixes after reserve inspection: a Source prefix retired
	// on its finite post-work interval may have just been reopened by that
	// inspection. Snapshotting before it would replenish a known-dead parent.
	present := func(hello ardp.Hello, class uint8) ([]byte, error) {
		return worker.PresentRefill(ctx, hello, class)
	}
	if err := worker.ReplenishPrefixes(ctx, present); err != nil {
		return err
	}
	for _, joined := range snapshot.Joins {
		if err := joined.Replenish(ctx, present); err != nil {
			retained := run.Retains(joined)
			// A retiring transport's Service owner retains its terminal cause.
			if retained {
				return err
			}
		}
	}
	return nil
}
