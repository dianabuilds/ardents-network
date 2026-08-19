package blockedentry

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const maximumCampaignOutput = 16 << 20

func collectEvents(config Config, canaries canaryCorpus, finalSpecValue *finalSpec) (
	[]event, []observer, cleanupInventory, *finalSummary, error,
) {
	if err := os.Mkdir(attributionRoot(config.EvidenceRoot), 0o700); err != nil {
		return nil, nil, cleanupInventory{}, nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := campaignCommand(ctx, config)
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, nil, cleanupInventory{}, nil, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, nil, cleanupInventory{}, nil, err
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return nil, nil, cleanupInventory{}, nil, err
	}
	if err := command.Start(); err != nil {
		return nil, nil, cleanupInventory{}, nil, err
	}
	stderrResult := make(chan []byte, 1)
	go readBounded(stderr, stderrResult)
	encoder := json.NewEncoder(stdin)
	decoder := json.NewDecoder(io.LimitReader(stdout, maximumCampaignOutput+1))
	decoder.DisallowUnknownFields()
	observers, cleanup := pristineObservers(), pristineCleanup()
	commitments := canaryCommitments(canaries)
	cellBound := 31 * time.Second
	if config.Mode == "final-campaign" {
		cellBound = 3 * time.Hour
	}
	var events []event
	var cells []finalCellObservation
	if finalSpecValue != nil {
		cells, err = collectFinalPrelude(ctx, encoder, decoder, finalSpecValue,
			config.EvidenceRoot+".partial/secret", cellBound)
		if err != nil {
			return nil, nil, cleanupInventory{}, nil, stopCampaign(command, stdin, stderrResult, err)
		}
	}
	for _, group := range hostileMatrix() {
		for _, variant := range group.Variants {
			if finalSpecValue != nil && finalEvidenceMutationVariant(group.ID, variant) {
				continue
			}
			for episode := range 5 {
				plan := cellPlan{Schema: "ardents-h3-blocked-cell-plan-v1",
					EventID: eventID(group.ID, variant, episode), Group: group.ID, Variant: variant,
					Episode: episode, ExpectedTerminal: expectedTerminal(group.ID, variant)}
				if finalSpecValue != nil {
					index := finalCellIndex("hostile/" + plan.EventID)
					if index < 0 || index >= len(finalSpecValue.Seeds) {
						return nil, nil, cleanupInventory{}, nil, stopCampaign(command, stdin, stderrResult,
							errors.New("hostile cell is absent from the frozen final schedule"))
					}
					plan.CellID, plan.Seed = finalSpecValue.CellOrder[index], finalSpecValue.Seeds[index]
				}
				if err := encoder.Encode(plan); err != nil {
					return nil, nil, cleanupInventory{}, nil, stopCampaign(command, stdin, stderrResult, err)
				}
				output, err := decodeCell(ctx, decoder, cellBound)
				if err != nil || output.Schema != "ardents-h3-blocked-cell-observation-v1" || output.EventID != plan.EventID ||
					output.CellID != plan.CellID || output.Seed != plan.Seed {
					return nil, nil, cleanupInventory{}, nil, stopCampaign(command, stdin, stderrResult,
						errors.Join(err, errors.New("hostile cell evidence is missing or reordered")))
				}
				owner := fixtureOwner(config.Mode, plan.EventID)
				attributionHash, err := writeAttribution(config.EvidenceRoot, plan.EventID, owner)
				if err != nil {
					return nil, nil, cleanupInventory{}, nil, stopCampaign(command, stdin, stderrResult, err)
				}
				gatePassed := output.ObservedTerminal == plan.ExpectedTerminal
				trustworthy := output.ProductStarted && output.FaultInjected && output.Attribution == "exact" &&
					forbiddenOwnersMatch(output.Observers, owner)
				events = append(events, event{ID: plan.EventID, Group: plan.Group, Variant: plan.Variant,
					Episode: plan.Episode, ExpectedTerminal: plan.ExpectedTerminal,
					ObservedTerminal: output.ObservedTerminal, GatePassed: gatePassed,
					EvidenceTrustworthy: trustworthy, FaultOwner: owner,
					AttributionEvidence: attributionHash,
					Diagnostic:          output.Diagnostic, CanarySetHash: commitments[plan.Variant],
					StartedOffsetMillis: output.StartedOffsetMillis, TerminalOffsetMillis: output.TerminalOffsetMillis,
					CleanupOffsetMillis: output.CleanupOffsetMillis, AdapterCleanupMillis: output.AdapterCleanupMillis})
				mergeObservers(observers, output.Observers, owner, trustworthy)
				mergeResiduals(&cleanup, output.Residuals, owner, attributionHash)
				if finalSpecValue != nil {
					cell, captureErr := finalCellFromOutput(config.EvidenceRoot+".partial/secret", output)
					if captureErr != nil {
						return nil, nil, cleanupInventory{}, nil, stopCampaign(command, stdin, stderrResult, captureErr)
					}
					cells = append(cells, cell)
				}
				if stopAfterCell(output, gatePassed, trustworthy, owner, canaries) {
					summary, finishErr := finishCollectedCampaign(command, stdin, decoder, stderrResult,
						cells, finalSpecValue != nil)
					return events, observers, cleanup, summary, finishErr
				}
			}
		}
	}
	summary, finishErr := finishCollectedCampaign(command, stdin, decoder, stderrResult,
		cells, finalSpecValue != nil)
	return events, observers, cleanup, summary, finishErr
}

func finalCellIndex(identity string) int {
	for index, candidate := range finalCellOrder() {
		if candidate == identity {
			return index
		}
	}
	return -1
}

func forbiddenOwnersMatch(observers []observer, owner string) bool {
	for _, observed := range observers {
		if observed.ForbiddenPackets > 0 && observed.ForbiddenOwner != owner {
			return false
		}
	}
	return true
}

func stopAfterCell(output cellObservation, gatePassed, trustworthy bool, owner string, canaries canaryCorpus) bool {
	if !trustworthy || !gatePassed && owner == "candidate" || owner == "candidate" &&
		diagnosticContainsCanary(output.Diagnostic, canaries) {
		return true
	}
	if len(output.Residuals) != len(residualKinds) || len(output.Observers) != len(boundaries) {
		return true
	}
	for index, item := range output.Residuals {
		if item.Kind != residualKinds[index] || item.Count > 0 ||
			item.Owner != "none" && item.Owner != "candidate" && item.Owner != "harness" {
			return true
		}
	}
	for index, observed := range output.Observers {
		if observed.Boundary != boundaries[index] || !observed.IPv4UDPControl || !observed.IPv6UDPControl ||
			!observed.IPv4TCPControl || observed.Attribution != "exact" || !observed.ObservationCompleted ||
			observed.ForbiddenPackets != 0 || observed.UnclassifiedPackets != 0 {
			return true
		}
	}
	return false
}

func diagnosticContainsCanary(diagnostic string, canaries canaryCorpus) bool {
	for _, set := range canaries.Sets {
		for _, value := range []string{set.Invite, set.Address, set.Path, set.Certificate} {
			if value != "" && strings.Contains(diagnostic, value) {
				return true
			}
		}
	}
	return false
}

func finishCampaign(command *exec.Cmd, stdin io.WriteCloser, decoder *json.Decoder, stderr <-chan []byte) (
	*finalSummary, error,
) {
	if err := stdin.Close(); err != nil {
		return nil, stopCampaign(command, stdin, stderr, err)
	}
	closed, closeErr, waitErr := waitForCampaignEnd(command, decoder, 31*time.Second)
	diagnostic := <-stderr
	if closeErr != nil || closed.Schema != "ardents-h3-blocked-campaign-closed-v1" ||
		closed.EventID != "" || waitErr != nil || len(diagnostic) > maximumCampaignOutput {
		return nil, errors.New("hostile campaign runner failed or emitted trailing evidence")
	}
	return closed.FinalSummary, nil
}

func waitForCampaignEnd(command *exec.Cmd, decoder *json.Decoder, bound time.Duration) (
	cellObservation, error, error,
) {
	type result struct {
		closed  cellObservation
		readErr error
		waitErr error
	}
	finished := make(chan result, 1)
	go func() {
		var closed cellObservation
		closeErr := decoder.Decode(&closed)
		var trailing json.RawMessage
		trailingErr := decoder.Decode(&trailing)
		if closeErr == nil && trailingErr != io.EOF {
			closeErr = errors.Join(closeErr, errors.New("hostile campaign emitted trailing evidence"), trailingErr)
		}
		finished <- result{closed, closeErr, command.Wait()}
	}()
	select {
	case observed := <-finished:
		return observed.closed, observed.readErr, observed.waitErr
	case <-time.After(bound):
		if command.Process != nil {
			_ = command.Process.Kill()
		}
		observed := <-finished
		return observed.closed, errors.Join(observed.readErr,
			errors.New("hostile campaign closure or process exit exceeded its bound")), observed.waitErr
	}
}

func stopCampaign(command *exec.Cmd, stdin io.Closer, stderr <-chan []byte, cause error) error {
	_ = stdin.Close()
	if command.Process != nil {
		_ = command.Process.Kill()
	}
	_ = command.Wait()
	<-stderr
	return cause
}

func readBounded(reader io.Reader, result chan<- []byte) {
	raw, _ := io.ReadAll(io.LimitReader(reader, maximumCampaignOutput+1))
	result <- raw
}

func canaryPath(evidenceRoot string) string {
	return evidenceRoot + ".partial/secret/canaries.json"
}
