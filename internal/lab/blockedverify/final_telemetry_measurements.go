package blockedverify

import (
	"reflect"
	"strconv"
)

type finalRecoveryMeasurement struct {
	Schema                 string `json:"schema"`
	Episode                uint16 `json:"episode"`
	ServiceClass           string `json:"service_class"`
	RecoveryClass          string `json:"recovery_class"`
	Attempt                string `json:"attempt"`
	ContactStarts          uint16 `json:"contact_starts"`
	LaterOrdinals          uint16 `json:"later_ordinals"`
	Cleanup                bool   `json:"cleanup"`
	PublishedDelayMillis   uint32 `json:"published_delay_millis"`
	ApplicationDelayMillis uint32 `json:"application_delay_millis"`
	Residuals              uint16 `json:"residuals"`
}

func validFinalPressureMeasurement(raw []byte) bool {
	var values []finalPressureCell
	return decodeFinalTelemetryLines(raw, &values) && len(values) == 1 &&
		values[0].Schema == "ardents-h3-final-pressure-v1" && values[0].ID != ""
}

func validFinalRecoveryMeasurement(raw []byte) bool {
	var values []finalRecoveryMeasurement
	return decodeFinalTelemetryLines(raw, &values) && len(values) == 1 &&
		values[0].Schema == "ardents-h3-final-recovery-episode-v1" && values[0].Episode < 5
}

func reproduceFinalPressureRecovery(root string, cells map[string]finalCellObservation,
	pressure []finalPressureCell, recovery finalRecovery,
) bool {
	derivedPressure := make([]finalPressureCell, 0, 5)
	for index := range 5 {
		cell, ok := cells["pressure/P"+strconv.Itoa(index)]
		if !ok {
			return false
		}
		raw, reason := loadFinalRawTelemetry(root, cell)
		value, ok := finalPressureFromTelemetry(raw.Files, "P"+strconv.Itoa(index), cell.Seed)
		if reason != "" || !ok {
			return false
		}
		derivedPressure = append(derivedPressure, value)
	}
	derivedRecovery := finalRecovery{AttemptIdentityStable: true, DeadlineStable: true}
	for episode := range 5 {
		cell, ok := cells["recovery/"+strconv.Itoa(episode)]
		if !ok {
			return false
		}
		raw, reason := loadFinalRawTelemetry(root, cell)
		value, ok := finalRecoveryFromTelemetry(raw.Files)
		if reason != "" || !ok || value.Episode != uint16(episode) {
			return false
		}
		derivedRecovery.Attempts++
		connectionLoss := value.ServiceClass == "abrupt connection loss" &&
			value.RecoveryClass == "bridge-deadline-exceeded"
		attemptStable := len(value.Attempt) == 64 && value.ContactStarts == 1 && value.LaterOrdinals == 0
		deadlineStable := value.Cleanup && value.PublishedDelayMillis <= 15_000 &&
			value.ApplicationDelayMillis <= 15_000
		if connectionLoss {
			derivedRecovery.ConnectionLoss++
		}
		derivedRecovery.LaterStarts += value.LaterOrdinals
		derivedRecovery.Residuals += value.Residuals
		derivedRecovery.AttemptIdentityStable = derivedRecovery.AttemptIdentityStable && attemptStable
		derivedRecovery.DeadlineStable = derivedRecovery.DeadlineStable && deadlineStable
	}
	return reflect.DeepEqual(derivedPressure, pressure) && derivedRecovery == recovery
}

func finalRecoveryFromTelemetry(files []finalRawTelemetry) (finalRecoveryMeasurement, bool) {
	for _, file := range files {
		if file.Kind != "recovery.json" {
			continue
		}
		var values []finalRecoveryMeasurement
		if !validFinalRecoveryMeasurement(file.Data) || !decodeFinalTelemetryLines(file.Data, &values) {
			return finalRecoveryMeasurement{}, false
		}
		return values[0], true
	}
	return finalRecoveryMeasurement{}, false
}
