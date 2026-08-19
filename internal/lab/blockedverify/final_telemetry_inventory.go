package blockedverify

import "strings"

type finalTelemetrySlot struct {
	root uint16
	role string
	kind string
}

func finalTelemetryLayout(cell string) []finalTelemetrySlot {
	var result []finalTelemetrySlot
	appendRole := func(root uint16, role string, tree bool) {
		kinds := []string{"resource.jsonl", "carrier.jsonl"}
		if tree {
			kinds = append(kinds, "tree.jsonl")
			if role == "bridge" {
				kinds = append(kinds, "runtime.jsonl")
			}
		}
		for _, kind := range kinds {
			result = append(result, finalTelemetrySlot{root, role, kind})
		}
	}
	capacity := 0
	if strings.HasPrefix(cell, "capacity/h3-s5-b1-v1-strong/") {
		capacity = 16
	} else if strings.HasPrefix(cell, "capacity/h3-s5-b1-v1/") {
		capacity = 4
	}
	if capacity > 0 {
		for root := range capacity {
			appendRole(uint16(root), "endpoint", true)
		}
		appendRole(0, "bridge", true)
		appendRole(0, "publisher", true)
		return result
	}
	if strings.HasPrefix(cell, "pressure/") {
		roots := 1
		if cell == "pressure/P4" {
			roots = 10
		}
		for root := range roots {
			result = append(result, finalTelemetrySlot{uint16(root), "bridge", "resource.jsonl"})
			if cell == "pressure/P0" || cell == "pressure/P1" || cell == "pressure/P4" {
				result = append(result, finalTelemetrySlot{uint16(root), "bridge", "pressure-input.json"})
			} else {
				result = append(result, finalTelemetrySlot{uint16(root), "pressure", "pressure-injection.jsonl"})
				result = append(result, finalTelemetrySlot{uint16(root), "bridge", "pressure-state.jsonl"})
			}
		}
		return append(result, finalTelemetrySlot{0, "bridge", "pressure.json"})
	}
	roots := 1
	for root := range roots {
		for _, role := range []string{"endpoint", "bridge", "publisher"} {
			appendRole(uint16(root), role, strings.HasPrefix(cell, "sustained/"))
		}
	}
	if strings.HasPrefix(cell, "recovery/") {
		result = append(result, finalTelemetrySlot{0, "bridge", "recovery.json"})
	}
	return result
}
