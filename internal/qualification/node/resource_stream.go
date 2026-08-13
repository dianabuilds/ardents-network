package node

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

type nodeResourceStreamLine struct {
	raw []byte
	err error
}

type nodeFaultTransition struct {
	generation uint64
	at         time.Time
	faults     map[string]bool
}

func (observer *nodeObserver) runSamples() {
	defer observer.work.Done()
	if err := observer.consumeNodeResourceStream(); err != nil && observer.ctx.Err() == nil {
		observer.recordEvidenceError(err)
	}
}

func (observer *nodeObserver) consumeNodeResourceStream() error {
	setupContext, setupCancel := context.WithTimeout(observer.ctx, 30*time.Second)
	candidates, err := observer.nodeResourceCandidates(setupContext)
	setupCancel()
	if err != nil {
		return invalidNodeCampaign(fmt.Errorf("discover initial node resource candidates: %w", err))
	}
	payload, err := encodeNodeResourceStreamUpdate(1, candidates)
	if err != nil {
		return invalidNodeCampaign(err)
	}
	ctx, cancel := context.WithCancel(observer.ctx)
	defer cancel()
	arguments := []string{"exec", "-i", observer.collectorID,
		"/usr/local/bin/ardents-qualify", "sample-node-stream", string(payload)}
	command := exec.CommandContext(ctx, "docker", arguments...)
	command.Env = observer.dockerEnvironment()
	stdin, err := command.StdinPipe()
	if err != nil {
		return invalidNodeCampaign(fmt.Errorf("open node resource stream input: %w", err))
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return invalidNodeCampaign(fmt.Errorf("open node resource stream: %w", err))
	}
	stderr := byteio.NewBuffer(32 << 10)
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		return invalidNodeCampaign(fmt.Errorf("start node resource stream: %w", err))
	}
	lines := make(chan nodeResourceStreamLine, 1)
	go scanNodeResourceStream(ctx, stdout, lines)
	watchdog := time.NewTimer(3 * time.Second)
	defer watchdog.Stop()
	generation := uint64(1)
	faultsByGeneration := map[uint64]map[string]bool{generation: observer.faultSnapshot()}
	var faultTransitions []nodeFaultTransition
	lastObservedGeneration := uint64(0)
	for {
		select {
		case <-observer.ctx.Done():
			_ = stdin.Close()
			cancel()
			_ = command.Wait()
			return observer.ctx.Err()
		case reset := <-observer.resourceReset:
			resets := []nodeResourceReset{reset}
			resets = observer.drainNodeResourceResets(resets)
			generation++
			faultsByGeneration[generation] = resets[len(resets)-1].faults
			faultTransitions = appendNodeFaultTransitions(faultTransitions, generation, resets)
			if err := observer.updateNodeResourceStream(stdin, generation); err != nil {
				cancel()
				_ = command.Wait()
				return err
			}
			resetNodeResourceWatchdog(watchdog)
		case <-watchdog.C:
			cancel()
			_ = command.Wait()
			return invalidNodeCampaign(errors.New("node resource stream missed its one-second deadline"))
		case line := <-lines:
			if line.err != nil {
				waitErr := command.Wait()
				return invalidNodeCampaign(fmt.Errorf("node resource stream ended: %w: %s", errors.Join(line.err, waitErr), stderr.Bytes()))
			}
			resets := observer.drainNodeResourceResets(nil)
			if len(resets) > 0 {
				generation++
				faultsByGeneration[generation] = resets[len(resets)-1].faults
				faultTransitions = appendNodeFaultTransitions(faultTransitions, generation, resets)
				if err := observer.updateNodeResourceStream(stdin, generation); err != nil {
					cancel()
					_ = command.Wait()
					return err
				}
			}
			streamOutput, samples, decodeErr := decodeNodeResourceStreamOutput(line.raw)
			if decodeErr != nil {
				cancel()
				_ = command.Wait()
				return decodeErr
			}
			if len(samples) == 0 {
				return invalidNodeCampaign(errors.New("node resource stream returned an empty required sample"))
			}
			baseFaults, found := faultsByGeneration[streamOutput.Generation]
			if !found || streamOutput.Generation < lastObservedGeneration || streamOutput.Generation > generation {
				return invalidNodeCampaign(errors.New("node resource stream generation is invalid"))
			}
			lastObservedGeneration = streamOutput.Generation
			scheduled := samples[0].At.Add(-time.Duration(samples[0].TickDelayNanos))
			faults := nodeResourceFaultsAt(baseFaults, streamOutput.Generation, samples[0].At, faultTransitions)
			if !observer.writeNodeSample(nodeSampleResult{at: scheduled, resources: samples, faults: faults}) {
				cancel()
				_ = command.Wait()
				return errors.New("node resource stream evidence write failed")
			}
			resetNodeResourceWatchdog(watchdog)
		}
	}
}

func appendNodeFaultTransitions(result []nodeFaultTransition, generation uint64,
	resets []nodeResourceReset,
) []nodeFaultTransition {
	for _, reset := range resets {
		result = append(result, nodeFaultTransition{generation: generation, at: reset.at, faults: reset.faults})
	}
	return result
}

func nodeResourceFaultsAt(base map[string]bool, generation uint64, at time.Time,
	transitions []nodeFaultTransition,
) map[string]bool {
	result := copyNodeFaults(base)
	for _, transition := range transitions {
		if transition.generation <= generation || transition.at.After(at) {
			continue
		}
		for name, active := range transition.faults {
			if active {
				result[name] = true
			}
		}
	}
	return result
}

func (observer *nodeObserver) updateNodeResourceStream(input io.Writer, generation uint64) error {
	ctx, cancel := context.WithTimeout(observer.ctx, 30*time.Second)
	candidates, err := observer.nodeResourceCandidates(ctx)
	cancel()
	if err != nil {
		return invalidNodeCampaign(fmt.Errorf("refresh node resource candidates: %w", err))
	}
	payload, err := encodeNodeResourceStreamUpdate(generation, candidates)
	if err != nil {
		return invalidNodeCampaign(err)
	}
	written, err := input.Write(append(payload, '\n'))
	if err != nil {
		return invalidNodeCampaign(fmt.Errorf("update node resource stream: %w", err))
	}
	if written != len(payload)+1 {
		return invalidNodeCampaign(io.ErrShortWrite)
	}
	return nil
}

func (observer *nodeObserver) drainNodeResourceResets(resets []nodeResourceReset) []nodeResourceReset {
	for {
		select {
		case reset := <-observer.resourceReset:
			resets = append(resets, reset)
		default:
			return resets
		}
	}
}

func resetNodeResourceWatchdog(watchdog *time.Timer) {
	if !watchdog.Stop() {
		select {
		case <-watchdog.C:
		default:
		}
	}
	watchdog.Reset(1500 * time.Millisecond)
}

func scanNodeResourceStream(ctx context.Context, input interface{ Read([]byte) (int, error) }, output chan<- nodeResourceStreamLine) {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64<<10), 384<<10)
	for scanner.Scan() {
		raw := append([]byte(nil), scanner.Bytes()...)
		select {
		case output <- nodeResourceStreamLine{raw: raw}:
		case <-ctx.Done():
			return
		}
	}
	select {
	case output <- nodeResourceStreamLine{err: errors.Join(scanner.Err(), errors.New("resource stream closed"))}:
	case <-ctx.Done():
	}
}
