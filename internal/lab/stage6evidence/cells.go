package stage6evidence

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/naming"
)

func runCell(ordinal uint32, spec cellSpec, admissionSecret [32]byte, start int64) (traceRecord, string, error) {
	trace := traceRecord{Schema: "ardents-stage-6-trace-v1", Cell: spec.id, Ordinal: ordinal,
		Operation: spec.predicate, StartOffset: start, Values: []int64{}, Fields: []string{}}
	switch spec.id {
	case "A0":
		name, err := naming.Parse("alice.site")
		if err != nil {
			return trace, "", err
		}
		wire, err := naming.EncodeWire(name)
		if err != nil {
			return trace, "", err
		}
		link, err := naming.FormatServiceLink(name)
		if err != nil {
			return trace, "", err
		}
		trace.Input, trace.Output, trace.Auxiliary = []byte("alice.site"), wire, []byte(link)
	case "A1":
		invalid := []byte("Alice\nálîce\nxn--alice\na..site\n-a.site\na--b.site\nARDENTS://alice")
		for _, raw := range []string{"Alice", "álîce", "xn--alice", "a..site", "-a.site", "a--b.site"} {
			if _, err := naming.Parse(raw); err == nil {
				return trace, "", errors.New("invalid Service Name was accepted")
			}
		}
		if _, err := naming.ParseServiceLink("ARDENTS://alice"); err == nil {
			return trace, "", errors.New("invalid Service Link was accepted")
		}
		trace.Input = invalid
	case "A2", "A3", "A4", "A5", "B2", "B4", "C0", "C1", "D5":
		if err := runLifecycleCell(&trace); err != nil {
			return trace, "", err
		}
	case "B0", "B1", "B3", "B5":
		if err := runAuthorityCell(&trace); err != nil {
			return trace, "", err
		}
	case "C4", "C5", "C6":
		if err := runClaimCell(&trace); err != nil {
			return trace, "", err
		}
	case "C7":
		if err := runAdmissionCell(&trace, admissionSecret); err != nil {
			return trace, "", err
		}
	case "C2", "C3":
		if err := runConnectionCell(&trace); err != nil {
			return trace, "", err
		}
	case "D0", "D1", "D3", "D4":
		if err := runResolutionCell(&trace); err != nil {
			return trace, "", err
		}
	case "D2":
		if err := runControlRoleCell(&trace, admissionSecret); err != nil {
			return trace, "", err
		}
	case "D6":
		if err := runNamespaceForkCell(&trace); err != nil {
			return trace, "", err
		}
	default:
		return trace, "", errors.New("stage 6 cell has no semantic implementation")
	}
	return trace, spec.class, nil
}
