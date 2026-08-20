package stage6verify

import (
	"bytes"
	"strings"
)

func verifyTrace(trace traceRecord, expected expectedCell, secret [32]byte) bool {
	if trace.Schema != "ardents-stage-6-trace-v1" || trace.Cell != expected.id ||
		trace.Operation != expected.predicate || trace.StartOffset < 0 || trace.EndOffset < trace.StartOffset {
		return false
	}
	switch trace.Cell {
	case "A0":
		wire := []byte{0, 1, 5, 'a', 'l', 'i', 'c', 'e', 4, 's', 'i', 't', 'e'}
		return string(trace.Input) == "alice.site" && bytes.Equal(trace.Output, wire) &&
			string(trace.Auxiliary) == "ardents://alice.site" && len(trace.Values) == 0 && len(trace.Fields) == 0
	case "A1":
		values := strings.Split(string(trace.Input), "\n")
		return len(values) == 7 && values[0] == "Alice" && values[1] == "álîce" &&
			values[2] == "xn--alice" && values[3] == "a..site" && values[4] == "-a.site" &&
			values[5] == "a--b.site" && values[6] == "ARDENTS://alice" && len(trace.Output) == 0 &&
			len(trace.Auxiliary) == 0 && len(trace.Values) == 0 && len(trace.Fields) == 0
	case "A2", "A3", "A4", "A5", "B2", "B4", "C0", "C1", "D5":
		return verifyLifecycleTrace(trace)
	case "B0", "B1", "B3", "B5":
		return verifyAuthorityTrace(trace)
	case "C4", "C5", "C6":
		return verifyClaimTrace(trace)
	case "C7":
		return verifyAdmissionTrace(trace, secret)
	case "C2", "C3":
		return verifyConnectionTrace(trace)
	case "D0", "D1", "D3", "D4":
		return verifyResolutionTrace(trace)
	case "D2":
		return verifyControlRoleTrace(trace, secret)
	case "D6":
		return verifyNamespaceForkTrace(trace)
	default:
		return false
	}
}

func validHex(raw string, size int) bool {
	if len(raw) != size*2 {
		return false
	}
	for _, value := range raw {
		if !(value >= '0' && value <= '9' || value >= 'a' && value <= 'f') {
			return false
		}
	}
	return true
}
