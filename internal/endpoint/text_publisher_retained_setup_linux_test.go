//go:build linux

package endpoint

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// The qualification setup is one retained 256-Connection Publisher set fed by
// four independent 64-Connection Readers. Exercise that exact ownership shape
// through the production producer before spending another one-shot VPS fixture.
func TestTextPublisherBuildsRetainedQualificationSetAcrossFourReaders(t *testing.T) {
	const readerCount, streamsPerReader = 4, 64
	// Use the smaller authorized #60 provider allowance. The ledger accounts
	// actual tx+rx for all sixteen local Nodes in addition to both retained JOIN
	// sides and their termination reservations; the ordinary one-GiB journey
	// fixture remains intentionally too small for this maximum workload.
	qualificationHosting := textNetworkHostingRootWithQuantity(t, 1024)
	reader, publisher, destination := textJoinedNetworkFixtureWithReaderMaximaAndRegistration(t, route.ClosedCarrierTCP, [3]uint32{512, 512, 0}, 9*time.Minute, func(index int, config *node.Config) {
		// All sixteen Nodes share one provider period for the same physical host.
		// Separate one-GiB fixture ledgers would each charge the host-wide loopback
		// counters and make parallel package tests interfere with qualification.
		config.HostingRoot = qualificationHosting
		config.ClosedForwarding.HostingRoot = qualificationHosting
		config.ClosedDataJoin.HostingRoot = qualificationHosting
	})
	source, ok := reader.endpoint.closedState.(*textSourceStateFixture)
	if !ok {
		t.Fatal("text qualification State fixture unavailable")
	}
	readers := []*textContext{reader}
	for index := 1; index < readerCount; index++ {
		readers = append(readers, independentTextReaderFixture(t, reader.endpoint.network, source, [3]uint32{512, 512, 0}))
	}
	// Bootstrap output is duty-wide and intentionally rate-limited. The
	// installed qualification's offline permission exchange supplies this
	// spacing before Reader work begins; preserve it here so this test isolates
	// the retained Introduction/JOIN set rather than the bootstrap refill gate.
	time.Sleep(10 * time.Second)
	for index, owner := range readers {
		if _, err := owner.openTextPrefix(t.Context()); err != nil {
			t.Fatalf("Reader %d prefix: %v", index, err)
		}
		if index+1 < len(readers) {
			time.Sleep(5 * time.Second)
		}
	}
	readerJobs := make([]*textJobIdentity, len(readers))
	// The installed runner gives every Reader the same final-opening pacer.
	// Independent preparation loops can drift together under a constrained
	// scheduler, so their initial phase offsets alone do not enforce the
	// Publisher's rolling four-openings-per-second admission boundary.
	qualificationPacer := &StreamQualificationMeasurements{}
	for index, owner := range readers {
		job := liveTextCapsuleJob(t, owner)
		job.qualification = &streamqualification.Init{Role: streamqualification.ReaderRole,
			Profile: streamqualification.ClientToPublisher, Nonce: fixtureID(byte(247 + index)), Seed: fixtureID(246)}
		job.qualificationAcquireIntroduction = qualificationPacer.acquireIntroductionOpening
		readerJobs[index] = job
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	publisherJob := liveTextCapsuleJob(t, publisher)
	publisherJob.qualification = &streamqualification.Init{Role: streamqualification.PublisherRole,
		Profile: streamqualification.ClientToPublisher, Nonce: fixtureID(245), Seed: fixtureID(246)}
	publisherWorker := &qualifiedTextWorker{job: publisherJob}
	delivered := make(chan connection.Stream)
	producerDone := make(chan error, 1)
	go func() { producerDone <- publisherWorker.produceNetwork(ctx, delivered) }()

	type readerResult struct {
		index   int
		streams []*textServiceStream
		err     error
	}
	readerDone := make(chan readerResult, len(readers))
	for index, owner := range readers {
		go func(index int, owner *textContext, job *textJobIdentity) {
			result := readerResult{index: index}
			worker := &qualifiedTextWorker{job: job, qualificationReader: index}
			until := time.Now().UTC().Add(15 * time.Minute).Unix()
			nextOpening := time.Now().Add(qualificationReaderOpeningDelay(index))
			for streamIndex := 0; streamIndex < streamsPerReader; streamIndex++ {
				if err := waitQualificationIntroductionOpening(ctx, nextOpening); err != nil {
					result.err = err
					break
				}
				attempt, err := owner.prepareTextIntroduction(ctx, job, destination, [3]int64{until, until, until})
				if err != nil {
					owner.mu.Lock()
					state := fmt.Sprintf("prefix=%t resolution=%t opening=%t issuance=%t permission=%t closed=%t",
						owner.prefix != nil, owner.resolution != nil, owner.prefixOpening != nil, owner.issuance != nil,
						owner.permission != nil, owner.closed)
					owner.mu.Unlock()
					result.err = fmt.Errorf("Reader %d Introduction %d (%s): %w", index, streamIndex, state, err)
					break
				}
				stream, err := owner.openTextJoinedService(ctx, job, attempt)
				if err != nil {
					owner.mu.Lock()
					prefix := owner.prefix
					owner.mu.Unlock()
					if prefix != nil {
						err = errors.Join(err, prefix.Close())
					}
					result.err = fmt.Errorf("Reader %d JOIN %d: %w", index, streamIndex, err)
					break
				}
				result.streams = append(result.streams, stream)
				id := qualificationStreamID(index, streamIndex)
				if _, err := stream.Write(qualificationHello(streamqualification.ClientToPublisher, fixtureID(246), id)); err != nil {
					result.err = fmt.Errorf("Reader %d hello %d: %w", index, streamIndex, err)
					break
				}
				if err := worker.replenishStreams(ctx); err != nil {
					result.err = fmt.Errorf("Reader %d setup refill %d: %w", index, streamIndex, err)
					break
				}
				nextOpening = time.Now().Add(qualificationIntroductionInterval)
			}
			readerDone <- result
		}(index, owner, readerJobs[index])
	}

	var readerStreams []*textServiceStream
	var publisherStreams []connection.Stream
	readerStreamsByID := make(map[uint32]connection.Stream, readerCount*streamsPerReader)
	publisherStreamsByID := make(map[uint32]connection.Stream, readerCount*streamsPerReader)
	readersFinished := 0
	producerFinished := false
	var setupErr error
	wanted := readerCount * streamsPerReader
	for setupErr == nil && (readersFinished < len(readers) || len(publisherStreams) < wanted) {
		select {
		case result := <-readerDone:
			readersFinished++
			readerStreams = append(readerStreams, result.streams...)
			for streamIndex, stream := range result.streams {
				readerStreamsByID[qualificationStreamID(result.index, streamIndex)] = stream
			}
			setupErr = errors.Join(setupErr, result.err)
		case stream := <-delivered:
			if stream == nil {
				setupErr = errors.New("Publisher producer ended before retained set")
				continue
			}
			var hello [45]byte
			if _, err := io.ReadFull(stream, hello[:]); err != nil {
				setupErr = errors.Join(setupErr, err)
			} else {
				id := binary.BigEndian.Uint32(hello[9:13])
				if id == 0 || publisherStreamsByID[id] != nil {
					setupErr = errors.Join(setupErr, errors.New("Publisher received duplicate or empty qualification stream ID"))
				} else {
					publisherStreamsByID[id] = stream
				}
			}
			publisherStreams = append(publisherStreams, stream)
			if err := publisherWorker.replenishStreams(ctx); err != nil {
				publisher.mu.Lock()
				state := fmt.Sprintf("prefix=%t resolution=%t opening=%t issuance=%t permission=%t closed=%t",
					publisher.prefix != nil, publisher.resolution != nil, publisher.prefixOpening != nil, publisher.issuance != nil,
					publisher.permission != nil, publisher.closed)
				prefix := publisher.prefix
				publisher.mu.Unlock()
				prefixDone := prefix == nil
				if !prefixDone {
					select {
					case <-prefix.Done():
						prefixDone = true
					default:
					}
				}
				setupErr = errors.Join(setupErr, fmt.Errorf("Publisher setup refill %d (%s prefixDone=%t): %w", len(publisherStreams)-1, state, prefixDone, err))
			}
		case err := <-producerDone:
			producerFinished = true
			publisher.mu.Lock()
			registration := publisher.registration
			publisher.mu.Unlock()
			reason := route.ClosedIntroductionEndUnknown
			if registration != nil {
				reason = registration.channel.EndReason()
			}
			setupErr = errors.Join(setupErr, fmt.Errorf("Publisher producer ended before retained set: registration=%s", reason), err)
		case <-ctx.Done():
			setupErr = errors.Join(setupErr, ctx.Err())
		}
	}
	if setupErr != nil {
		cancel()
		for readersFinished < len(readers) {
			result := <-readerDone
			readersFinished++
			readerStreams = append(readerStreams, result.streams...)
			for streamIndex, stream := range result.streams {
				readerStreamsByID[qualificationStreamID(result.index, streamIndex)] = stream
			}
			setupErr = errors.Join(setupErr, result.err)
		}
		for !producerFinished {
			select {
			case stream := <-delivered:
				if stream != nil {
					publisherStreams = append(publisherStreams, stream)
				}
			case <-producerDone:
				producerFinished = true
			}
		}
	}
	var pairIDs []uint32
	if setupErr == nil && len(publisherStreamsByID) == wanted && len(readerStreamsByID) == wanted {
		for readerIndex := 0; readerIndex < readerCount; readerIndex++ {
			for streamIndex := 0; streamIndex < streamsPerReader; streamIndex++ {
				id := qualificationStreamID(readerIndex, streamIndex)
				_, publisherOK := publisherStreamsByID[id]
				_, readerOK := readerStreamsByID[id]
				if !publisherOK || !readerOK {
					setupErr = errors.Join(setupErr, fmt.Errorf("retained stream pair %d unavailable", id))
					continue
				}
				pairIDs = append(pairIDs, id)
			}
		}
		// The installed Publisher declares its complete set ready before either
		// owner starts the workload. Exercise that real barrier instead of leaving
		// the earliest setup streams idle until teardown.
		for _, id := range pairIDs {
			if _, err := publisherStreamsByID[id].Write([]byte{1}); err != nil {
				setupErr = errors.Join(setupErr, fmt.Errorf("Publisher ready %d: %w", id, err))
				break
			}
			var ready [1]byte
			if _, err := io.ReadFull(readerStreamsByID[id], ready[:]); err != nil || ready[0] != 1 {
				setupErr = errors.Join(setupErr, err, fmt.Errorf("Reader ready %d unavailable", id))
				break
			}
		}
	}
	// The acceptance boundary is the complete authenticated set and its ready
	// barrier. The installed workload owns graceful per-stream EOF. This setup
	// fixture cancels its bounded lifetime once; the registered job, context and
	// Endpoint owners then join their retained streams in ownership order.
	cancel()
	for !producerFinished {
		select {
		case stream := <-delivered:
			if stream != nil {
				setupErr = errors.Join(setupErr, stream.Close())
			}
		case <-producerDone:
			producerFinished = true
		}
	}
	if setupErr != nil {
		t.Fatalf("retained setup reached %d Publisher and %d Reader streams: %v", len(publisherStreams), len(readerStreams), setupErr)
	}
	if len(publisherStreams) != wanted || len(readerStreams) != wanted {
		t.Fatalf("retained setup = %d Publisher / %d Reader streams", len(publisherStreams), len(readerStreams))
	}
}

func TestQualificationReopensRetiredSourcePrefixForIssuerReserve(t *testing.T) {
	endpoint, owner, source := startTextIssuanceNetwork(t, route.ClosedCarrierTCP)
	defer func() {
		if err := endpoint.Close(); err != nil {
			t.Error(err)
		}
	}()
	prefix, err := owner.openTextPrefix(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// Fund the retained members' class-2 admission stock, as the participant
	// bootstrap does, so a prefix rebirth can open from finalized stock.
	selection := selectTextSource(t, owner)
	for _, receiver := range [][32]byte{selection.EntryNodeID, selection.InteriorNodeID} {
		if err := owner.issueTextTokens(t.Context(), [][32]byte{receiver}, 2); err != nil {
			t.Fatal(err)
		}
	}
	ready := func() int {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		count := 0
		for _, stock := range owner.permission.stock {
			if stock.challenge.ReceiverNodeID == source.view.Profile.IssuerNodeID && stock.challenge.Class == 1 {
				count += len(stock.tokens)
			}
		}
		return count
	}
	// Ensure once while the prefix is live so the finalized class-1 reserve
	// exists, then spend it deterministically.
	if err := owner.ensureQualificationIssuerReserve(t.Context(), qualificationIssuerReserve); err != nil {
		t.Fatal(err)
	}
	// Spend the finalized class-1 reserve down to four
	// tokens: above the two-token prefix-open admission floor and below the
	// qualification refill boundary.
	for ready() > 4 {
		owner.mu.Lock()
		for slot := range owner.permission.stock {
			stock := &owner.permission.stock[slot]
			if stock.challenge.ReceiverNodeID == source.view.Profile.IssuerNodeID && stock.challenge.Class == 1 && len(stock.tokens) > 0 {
				clear(stock.tokens[0])
				stock.tokens = stock.tokens[1:]
				break
			}
		}
		owner.mu.Unlock()
	}
	// Retire the Source prefix on its joined terminal path, as the finite
	// post-work interval does inside a long retained setup.
	if err := prefix.Close(); err != nil {
		t.Fatal(err)
	}
	before := ready()
	if err := owner.ensureQualificationIssuerReserve(t.Context(), qualificationIssuerReserve); err != nil {
		t.Fatalf("retired Source prefix failed the issuer reserve: %v", err)
	}
	owner.mu.Lock()
	reopened := owner.prefix
	owner.mu.Unlock()
	if reopened == nil || reopened == prefix {
		t.Fatalf("issuer reserve did not reopen the retired Source prefix: %p", reopened)
	}
	// Repeated completed-stream boundaries converge on the reserve; one
	// boundary may only fund part of it while the hour allocation remains.
	if after := ready(); after <= before {
		t.Fatalf("qualification issuer reserve = %d after %d", after, before)
	}
}

func TestQualificationReaderOpeningDelayStaggersFourReaders(t *testing.T) {
	want := []time.Duration{0, 312500 * time.Microsecond, 625 * time.Millisecond, 937500 * time.Microsecond}
	for reader, expected := range want {
		if got := qualificationReaderOpeningDelay(reader); got != expected {
			t.Fatalf("Reader %d opening delay = %s, want %s", reader, got, expected)
		}
	}
}

func TestQualificationRefillsPublisherIssuerReserveBetweenStreams(t *testing.T) {
	endpoint, owner, source := startTextIssuanceNetwork(t, route.ClosedCarrierTCP)
	defer func() {
		if err := endpoint.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := owner.openTextPrefix(t.Context()); err != nil {
		t.Fatal(err)
	}
	selection := selectTextSource(t, owner)
	ready := func() int {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		count := 0
		for _, stock := range owner.permission.stock {
			if stock.challenge.ReceiverNodeID == source.view.Profile.IssuerNodeID && stock.challenge.Class == 1 {
				count += len(stock.tokens)
			}
		}
		return count
	}
	for attempts := 0; ready() >= qualificationIssuerReserve && attempts < 64; attempts++ {
		if err := owner.issueTextTokens(t.Context(), [][32]byte{selection.EntryNodeID}, 2); err != nil {
			t.Fatal(err)
		}
	}
	before := ready()
	if before >= qualificationIssuerReserve {
		t.Fatalf("could not reach qualification issuer refill boundary: %d", before)
	}
	if err := owner.ensureQualificationIssuerReserve(t.Context(), qualificationIssuerReserve); err != nil {
		t.Fatal(err)
	}
	if after := ready(); after < qualificationIssuerReserve || after <= before {
		t.Fatalf("qualification issuer reserve = %d after %d", after, before)
	}
}
