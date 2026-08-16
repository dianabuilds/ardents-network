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

type cellPlan struct {
	Schema           string `json:"schema"`
	EventID          string `json:"event_id"`
	Group            string `json:"group"`
	Variant          string `json:"variant"`
	Episode          int    `json:"episode"`
	ExpectedTerminal string `json:"expected_terminal"`
}

type cellObservation struct {
	Schema               string     `json:"schema"`
	EventID              string     `json:"event_id"`
	ObservedTerminal     string     `json:"observed_terminal"`
	ProductStarted       bool       `json:"product_started"`
	FaultInjected        bool       `json:"fault_injected"`
	FaultOwner           string     `json:"fault_owner"`
	Attribution          string     `json:"attribution"`
	AttributionEvidence  string     `json:"attribution_evidence"`
	Diagnostic           string     `json:"diagnostic"`
	StartedOffsetMillis  uint64     `json:"started_offset_millis"`
	TerminalOffsetMillis uint64     `json:"terminal_offset_millis"`
	CleanupOffsetMillis  uint64     `json:"cleanup_offset_millis"`
	AdapterCleanupMillis uint64     `json:"adapter_cleanup_millis"`
	Observers            []observer `json:"observers"`
	Residuals            []residual `json:"residuals"`
}

func collectEvents(config Config, canaries canaryCorpus) ([]event, []observer, cleanupInventory, error) {
	if err := os.Mkdir(attributionRoot(config.EvidenceRoot), 0o700); err != nil {
		return nil, nil, cleanupInventory{}, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := campaignCommand(ctx, config)
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, nil, cleanupInventory{}, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, nil, cleanupInventory{}, err
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return nil, nil, cleanupInventory{}, err
	}
	if err := command.Start(); err != nil {
		return nil, nil, cleanupInventory{}, err
	}
	stderrResult := make(chan []byte, 1)
	go readBounded(stderr, stderrResult)
	encoder := json.NewEncoder(stdin)
	decoder := json.NewDecoder(io.LimitReader(stdout, maximumCampaignOutput+1))
	decoder.DisallowUnknownFields()
	observers, cleanup := pristineObservers(), pristineCleanup()
	commitments := canaryCommitments(canaries)
	var events []event
	for _, group := range hostileMatrix() {
		for _, variant := range group.Variants {
			for episode := range 5 {
				plan := cellPlan{Schema: "ardents-h3-blocked-cell-plan-v1",
					EventID: eventID(group.ID, variant, episode), Group: group.ID, Variant: variant,
					Episode: episode, ExpectedTerminal: expectedTerminal(group.ID, variant)}
				if err := encoder.Encode(plan); err != nil {
					return nil, nil, cleanupInventory{}, stopCampaign(command, stdin, stderrResult, err)
				}
				output, err := decodeCell(ctx, decoder)
				if err != nil || output.Schema != "ardents-h3-blocked-cell-observation-v1" || output.EventID != plan.EventID {
					return nil, nil, cleanupInventory{}, stopCampaign(command, stdin, stderrResult,
						errors.Join(err, errors.New("hostile cell evidence is missing or reordered")))
				}
				owner := fixtureOwner(config.Mode, plan.EventID)
				attributionHash, err := writeAttribution(config.EvidenceRoot, plan.EventID, owner)
				if err != nil {
					return nil, nil, cleanupInventory{}, stopCampaign(command, stdin, stderrResult, err)
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
				if stopAfterCell(output, gatePassed, trustworthy, owner, canaries) {
					return events, observers, cleanup, finishCampaign(command, stdin, decoder, stderrResult)
				}
			}
		}
	}
	return events, observers, cleanup, finishCampaign(command, stdin, decoder, stderrResult)
}

func forbiddenOwnersMatch(observers []observer, owner string) bool {
	for _, observed := range observers {
		if observed.ForbiddenPackets > 0 && observed.ForbiddenOwner != owner {
			return false
		}
	}
	return true
}

func campaignCommand(ctx context.Context, config Config) *exec.Cmd {
	command := exec.CommandContext(ctx, config.RunnerPath)
	command.Env = []string{"ARDENTS_BLOCKED_MODE=" + config.Mode, "ARDENTS_BLOCKED_CLIENT=" + config.ClientPath,
		"ARDENTS_BLOCKED_SERVER=" + config.ServerPath, "ARDENTS_BLOCKED_CANARY_FILE=" + canaryPath(config.EvidenceRoot),
		"ARDENTS_BLOCKED_CELL_HELPER=1", "SYSTEMROOT=" + os.Getenv("SYSTEMROOT")}
	return command
}

func decodeCell(ctx context.Context, decoder *json.Decoder) (cellObservation, error) {
	type decoded struct {
		value cellObservation
		err   error
	}
	result := make(chan decoded, 1)
	go func() {
		var value cellObservation
		err := decoder.Decode(&value)
		result <- decoded{value: value, err: err}
	}()
	select {
	case item := <-result:
		return item.value, item.err
	case <-time.After(31 * time.Second):
		return cellObservation{}, errors.New("hostile cell exceeded its execution and cleanup bound")
	case <-ctx.Done():
		return cellObservation{}, ctx.Err()
	}
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

func finishCampaign(command *exec.Cmd, stdin io.WriteCloser, decoder *json.Decoder, stderr <-chan []byte) error {
	if err := stdin.Close(); err != nil {
		return stopCampaign(command, stdin, stderr, err)
	}
	closed, closeErr, waitErr := waitForCampaignEnd(command, decoder, 31*time.Second)
	diagnostic := <-stderr
	if closeErr != nil || closed.Schema != "ardents-h3-blocked-campaign-closed-v1" ||
		closed.EventID != "" || waitErr != nil || len(diagnostic) > maximumCampaignOutput {
		return errors.New("hostile campaign runner failed or emitted trailing evidence")
	}
	return nil
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
