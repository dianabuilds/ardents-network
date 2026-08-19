package blockedentry

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"time"
)

type cellPlan struct {
	Schema           string `json:"schema"`
	EventID          string `json:"event_id"`
	Group            string `json:"group"`
	Variant          string `json:"variant"`
	Episode          int    `json:"episode"`
	ExpectedTerminal string `json:"expected_terminal"`
	CellID           string `json:"cell_id,omitempty"`
	Seed             string `json:"seed,omitempty"`
}

type cellObservation struct {
	Schema               string             `json:"schema"`
	EventID              string             `json:"event_id"`
	CellID               string             `json:"cell_id,omitempty"`
	Seed                 string             `json:"seed,omitempty"`
	ObservedTerminal     string             `json:"observed_terminal"`
	ProductStarted       bool               `json:"product_started"`
	FaultInjected        bool               `json:"fault_injected"`
	FaultOwner           string             `json:"fault_owner"`
	Attribution          string             `json:"attribution"`
	AttributionEvidence  string             `json:"attribution_evidence"`
	Diagnostic           string             `json:"diagnostic"`
	StartedOffsetMillis  uint64             `json:"started_offset_millis"`
	TerminalOffsetMillis uint64             `json:"terminal_offset_millis"`
	CleanupOffsetMillis  uint64             `json:"cleanup_offset_millis"`
	AdapterCleanupMillis uint64             `json:"adapter_cleanup_millis"`
	Observers            []observer         `json:"observers"`
	ObserverEvidence     artifactCommitment `json:"observer_evidence,omitempty"`
	TelemetryEvidence    artifactCommitment `json:"telemetry_evidence,omitempty"`
	Residuals            []residual         `json:"residuals"`
	FinalSummary         *finalSummary      `json:"final_summary,omitempty"`
}

func campaignCommand(ctx context.Context, config Config) *exec.Cmd {
	command := exec.CommandContext(ctx, config.RunnerPath)
	command.Env = []string{"ARDENTS_BLOCKED_MODE=" + config.Mode, "ARDENTS_BLOCKED_CLIENT=" + config.ClientPath,
		"ARDENTS_BLOCKED_SERVER=" + config.ServerPath, "ARDENTS_BLOCKED_CANARY_FILE=" + canaryPath(config.EvidenceRoot),
		"ARDENTS_BLOCKED_CAMPAIGN_SPEC=" + config.CampaignSpecPath,
		"ARDENTS_BLOCKED_SECRET_ROOT=" + config.EvidenceRoot + ".partial/secret",
		"ARDENTS_BLOCKED_CELL_HELPER=1", "SYSTEMROOT=" + os.Getenv("SYSTEMROOT")}
	if config.CampaignSpecPath != "" {
		dockerConfig := config.EvidenceRoot + ".partial/secret/runtime/docker-config"
		command.Env = append(command.Env, "ARDENTS_BLOCKED_WORKSPACE_ROOT="+config.WorkspaceRoot,
			"ARDENTS_BLOCKED_COMPOSE_FILE="+config.RuntimeComposePath,
			"ARDENTS_BLOCKED_PRODUCT_IMAGE="+config.ProductImageID,
			"ARDENTS_BLOCKED_TOOL_IMAGE="+config.ToolImageID,
			"PATH=/usr/bin:/bin", "DOCKER_HOST=unix:///var/run/docker.sock",
			"DOCKER_CONFIG="+dockerConfig)
	}
	return command
}

func decodeCell(ctx context.Context, decoder *json.Decoder, bound time.Duration) (cellObservation, error) {
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
	case <-time.After(bound):
		return cellObservation{}, errors.New("hostile cell exceeded its execution and cleanup bound")
	case <-ctx.Done():
		return cellObservation{}, ctx.Err()
	}
}
