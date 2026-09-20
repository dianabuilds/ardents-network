//go:build linux

// Package streamqualification's bridge moves bytes only on Service streams
// already authenticated and owned by Endpoint.
package streamqualification

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
)

// BoundStream pairs a real Service stream with its fixed workload position.
// IDs 1..127 are active; the remaining odd IDs through 511 carry canaries.
// Each Reader owns 16 active and 48 retained positions, Publisher owns all 256.
type BoundStream struct {
	ID     uint32
	Stream connection.Stream
}

type StreamMeasurement struct {
	LastTxElapsed time.Duration
	LastRxElapsed time.Duration
	ID            uint32
	Tx            uint64
	Rx            uint64
	MaximumTxGap  time.Duration
	MaximumRxGap  time.Duration
	LastTx        time.Time
	LastRx        time.Time
}

// Report is local evidence, never a worker authority or an admission receipt.
type Report struct {
	MeasuredDuration time.Duration
	StartedElapsed   time.Duration
	StoppedElapsed   time.Duration
	Started          time.Time
	Finished         time.Time
	Stopped          time.Time
	Streams          []StreamMeasurement
	Failure          string
}

type bridgeLane struct {
	BoundStream
	outgoing chan workerFrame
	credit   chan uint32
	pending  atomic.Uint32
	eof      bool
}

// RunConnections owns and joins every supplied stream and the attachment.
// Per-stream queues are bounded by the existing 64 KiB worker credit window.
func RunConnections(ctx context.Context, attachment io.ReadWriteCloser, init Init, supplied []BoundStream, observe func(context.Context, Report) error) (report Report, outcome error) {
	owned := false
	defer func() {
		if owned {
			return
		}
		if attachment != nil {
			outcome = errors.Join(outcome, attachment.Close())
		}
		for _, bound := range supplied {
			if bound.Stream != nil {
				outcome = errors.Join(outcome, bound.Stream.Close())
			}
		}
	}()
	schedule, err := init.Profile.Definition(init.Role)
	if err != nil {
		outcome = err
		return
	}
	if ctx == nil || attachment == nil || len(supplied) != int(schedule.OpenConnections) {
		outcome = errors.New("qualification stream set incomplete")
		return
	}
	owned = true
	bounded, cancel := context.WithCancel(ctx)
	initialized := make(chan struct{})
	lanes := make(map[uint32]*bridgeLane, len(supplied))
	var workers sync.WaitGroup
	var writerMu, reportMu sync.Mutex
	failures := make(chan error, len(supplied)*2+1)
	write := func(frame workerFrame) error {
		writerMu.Lock()
		defer writerMu.Unlock()
		return writeWorkerFrame(attachment, frame)
	}
	fail := func(err error) {
		if err != nil {
			select {
			case failures <- err:
			default:
			}
			cancel()
		}
	}
	interrupted := make(chan struct{})
	stop := context.AfterFunc(bounded, func() {
		defer close(interrupted)
		_ = attachment.Close()
		for _, bound := range supplied {
			if bound.Stream != nil {
				_ = bound.Stream.Close()
			}
		}
	})
	defer func() {
		cancel()
		if !stop() {
			<-interrupted
		}
		outcome = errors.Join(outcome, attachment.Close())
		for _, bound := range supplied {
			if bound.Stream != nil {
				outcome = errors.Join(outcome, bound.Stream.Close())
			}
		}
		workers.Wait()
		close(failures)
		for failure := range failures {
			outcome = errors.Join(outcome, failure)
		}
		report.Finished = time.Now()
		if outcome != nil {
			report.Failure = outcome.Error()
		}
	}()
	report.Started = time.Now()
	for index, bound := range supplied {
		if bound.Stream == nil || bound.ID == 0 || bound.ID%2 != 1 || bound.ID > 511 || lanes[bound.ID] != nil || index > 0 && supplied[index-1].ID >= bound.ID {
			outcome = errors.New("qualification stream set invalid")
			return
		}
		lane := &bridgeLane{BoundStream: bound, outgoing: make(chan workerFrame, 65), credit: make(chan uint32, 4)}
		lanes[bound.ID] = lane
		report.Streams = append(report.Streams, StreamMeasurement{ID: bound.ID, LastTx: report.Started, LastRx: report.Started})
	}
	for index, bound := range supplied {
		lane := lanes[bound.ID]
		workers.Add(2)
		go func() {
			defer workers.Done()
			select {
			case <-initialized:
			case <-bounded.Done():
				return
			}
			buffer := make([]byte, frameLimit)
			for {
				n, readErr := lane.Stream.Read(buffer)
				if n > 0 {
					now := time.Now()
					reportMu.Lock()
					measurement := &report.Streams[index]
					measurement.Rx += uint64(n)
					measurement.MaximumRxGap = max(measurement.MaximumRxGap, now.Sub(measurement.LastRx))
					measurement.LastRx = now
					measurement.LastRxElapsed = now.Sub(report.Started)
					reportMu.Unlock()
					if err := write(workerFrame{kind: frameBytes, id: lane.ID, body: buffer[:n]}); err != nil {
						fail(err)
						return
					}
					remaining := uint32(n)
					for remaining > 0 {
						select {
						case credit := <-lane.credit:
							if credit > remaining {
								fail(errors.New("qualification receive credit exceeds delivery"))
								return
							}
							remaining -= credit
						case <-bounded.Done():
							return
						}
					}
				}
				if readErr != nil {
					if errors.Is(readErr, io.EOF) {
						fail(write(workerFrame{kind: frameEOF, id: lane.ID}))
					} else {
						fail(readErr)
					}
					return
				}
				if n == 0 {
					fail(io.ErrNoProgress)
					return
				}
			}
		}()
		go func() {
			defer workers.Done()
			for {
				select {
				case frame := <-lane.outgoing:
					if frame.kind == frameEOF {
						fail(lane.Stream.CloseInput())
						return
					}
					n := len(frame.body)
					if err := writeWorkerBytes(lane.Stream, frame.body); err != nil {
						fail(err)
						return
					}
					lane.pending.Add(^uint32(n - 1))
					now := time.Now()
					reportMu.Lock()
					measurement := &report.Streams[index]
					measurement.Tx += uint64(n)
					measurement.MaximumTxGap = max(measurement.MaximumTxGap, now.Sub(measurement.LastTx))
					measurement.LastTx = now
					measurement.LastTxElapsed = now.Sub(report.Started)
					reportMu.Unlock()
					var credit [4]byte
					binary.BigEndian.PutUint32(credit[:], uint32(n))
					if err := write(workerFrame{kind: frameCredit, id: lane.ID, body: credit[:]}); err != nil {
						fail(err)
						return
					}
				case <-bounded.Done():
					return
				}
			}
		}()
	}
	workers.Add(1)
	go func() {
		defer workers.Done()
		for _, bound := range supplied {
			if err := write(workerFrame{kind: frameOpen, id: bound.ID}); err != nil {
				fail(err)
				return
			}
		}
		reportMu.Lock()
		report.Started = time.Now()
		for index := range report.Streams {
			report.Streams[index].LastTx = report.Started
			report.Streams[index].LastRx = report.Started
		}
		reportMu.Unlock()
		var initial [4]byte
		binary.BigEndian.PutUint32(initial[:], frameCreditWindow)
		for _, bound := range supplied {
			if err := write(workerFrame{kind: frameCredit, id: bound.ID, body: initial[:]}); err != nil {
				fail(err)
				return
			}
		}
		close(initialized)
	}()
	if observe != nil {
		workers.Add(1)
		go func() {
			defer workers.Done()
			select {
			case <-initialized:
			case <-bounded.Done():
				return
			}
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-bounded.Done():
					return
				case <-ticker.C:
					reportMu.Lock()
					snapshot := report
					snapshot.Streams = append([]StreamMeasurement(nil), report.Streams...)
					reportMu.Unlock()
					if err := observe(bounded, snapshot); err != nil {
						fail(err)
						return
					}
				}
			}
		}()
	}
	closed := make(map[uint32]bool, len(lanes))
	for len(closed) < len(lanes) {
		frame, err := readWorkerFrame(attachment)
		if err != nil {
			outcome = errors.Join(err, bounded.Err())
			return
		}
		lane := lanes[frame.id]
		if lane == nil || closed[frame.id] {
			outcome = errors.New("qualification worker returned unknown or closed stream")
			return
		}
		switch frame.kind {
		case frameBytes, frameEOF:
			if lane.eof {
				outcome = errors.New("qualification worker sent data after EOF")
				return
			}
			if frame.kind == frameEOF {
				lane.eof = true
			} else if lane.pending.Add(uint32(len(frame.body))) > frameCreditWindow {
				outcome = errors.New("qualification worker exceeded stream credit")
				return
			}
			select {
			case lane.outgoing <- frame:
			case <-bounded.Done():
				outcome = bounded.Err()
				return
			default:
				outcome = errors.New("qualification worker exceeded stream queue")
				return
			}
		case frameCredit:
			credit := binary.BigEndian.Uint32(frame.body)
			if credit == 0 || credit > frameCreditWindow {
				outcome = errors.New("qualification worker returned invalid credit")
				return
			}
			select {
			case lane.credit <- credit:
			default:
				outcome = errors.New("qualification worker exceeded credit queue")
				return
			}
		case frameClose:
			if frame.body[0] != 0 {
				outcome = fmt.Errorf("qualification stream %d failed", frame.id)
				return
			}

			closed[frame.id] = true
		default:
			outcome = errors.New("qualification worker returned invalid operation")
			return
		}
	}
	reportMu.Lock()
	report.Stopped = time.Now()
	report.MeasuredDuration = report.Stopped.Sub(report.Started)
	reportMu.Unlock()
	// Drain every worker result before waiting on Service cleanup, so a slow
	// terminal on one stream cannot stop delivery of EOF to another stream.
	for _, bound := range supplied {
		select {
		case terminal, ok := <-bound.Stream.Done():
			if !ok || terminal.Class != connection.CleanClose {
				outcome = errors.Join(outcome, fmt.Errorf("qualification stream %d terminal failed", bound.ID))
			}
		case <-bounded.Done():
			outcome = errors.Join(outcome, bounded.Err())
			return
		}
	}
	outcome = errors.Join(outcome, ctx.Err())
	return
}
