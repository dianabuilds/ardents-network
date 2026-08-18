package blockedentry

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func collectFinalPrelude(ctx context.Context, encoder *json.Encoder, decoder *json.Decoder,
	spec *finalSpec, secretRoot string, bound time.Duration,
) ([]finalCellObservation, error) {
	hostileCount := 0
	for _, group := range hostileMatrix() {
		hostileCount += len(group.Variants) * 5
	}
	limit := len(spec.CellOrder) - hostileCount
	if limit <= 0 || len(spec.Seeds) != len(spec.CellOrder) {
		return nil, errors.New("final non-hostile schedule is incomplete")
	}
	cells := make([]finalCellObservation, 0, len(spec.CellOrder))
	for index := range limit {
		plan := cellPlan{Schema: "ardents-h3-blocked-cell-plan-v1", CellID: spec.CellOrder[index],
			Seed: spec.Seeds[index]}
		if err := encoder.Encode(plan); err != nil {
			return nil, err
		}
		output, err := decodeCell(ctx, decoder, bound)
		if err != nil || output.Schema != "ardents-h3-blocked-cell-observation-v1" || output.EventID != "" ||
			output.CellID != plan.CellID || output.Seed != plan.Seed ||
			!cleanFinalObservation(output, !strings.Contains(plan.CellID, "/direct-")) {
			return nil, errors.Join(err, errors.New("final non-hostile cell evidence is missing or invalid"))
		}
		cell, captureErr := finalCellFromOutput(secretRoot, output)
		if captureErr != nil {
			return nil, captureErr
		}
		cells = append(cells, cell)
	}
	return cells, nil
}

func finalCellFromOutput(secretRoot string, output cellObservation) (finalCellObservation, error) {
	artifact, err := captureFinalObserverEvidence(secretRoot, output)
	if err != nil {
		return finalCellObservation{}, err
	}
	telemetry, err := captureFinalTelemetryEvidence(secretRoot, output)
	if err != nil {
		return finalCellObservation{}, err
	}
	return finalCellObservation{ID: output.CellID, Seed: output.Seed, Terminal: output.ObservedTerminal,
		StartedOffsetMillis: output.StartedOffsetMillis, TerminalOffsetMillis: output.TerminalOffsetMillis,
		CleanupOffsetMillis: output.CleanupOffsetMillis, ObserverEvidence: artifact, TelemetryEvidence: telemetry}, nil
}

func cleanFinalObservation(output cellObservation, productRequired bool) bool {
	if (productRequired && !output.ProductStarted) || output.TerminalOffsetMillis < output.StartedOffsetMillis ||
		output.CleanupOffsetMillis < output.TerminalOffsetMillis || len(output.Residuals) != len(residualKinds) ||
		output.CleanupOffsetMillis-output.TerminalOffsetMillis > 15_000 || len(output.Observers) != len(boundaries) {
		return false
	}
	for index, item := range output.Residuals {
		if item.Kind != residualKinds[index] || item.Count != 0 {
			return false
		}
	}
	for index, item := range output.Observers {
		if item.Boundary != boundaries[index] || !item.IPv4UDPControl || !item.IPv6UDPControl ||
			!item.IPv4TCPControl || !item.ObservationCompleted || item.ForbiddenPackets != 0 ||
			item.UnclassifiedPackets != 0 || item.Attribution != "exact" {
			return false
		}
	}
	return true
}
