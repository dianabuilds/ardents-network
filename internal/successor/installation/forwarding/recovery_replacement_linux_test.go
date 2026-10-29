package forwarding

import (
	"bytes"
	"testing"
)

// Invented identities isolate the byte classifier. They supply no cut proof,
// Release authority, native custody or installed recovery acceptance.
func TestForwardingRecoveryReplacementKeepsPrefixAndOnlyCompletesStoppedPublication(t *testing.T) {
	rows, selected, previous, hashes, before, after := replacementContextVector(t)
	whole := encodeReplacementVector(t, rows)
	for _, count := range []int{61, 62} {
		prefix := encodeReplacementVector(t, rows[:count])
		unchanged := bytes.Clone(prefix)
		cut := transitionCutResult{Boundary: transitionMessage{Operation: "replacement", Journal: rows[0].Path, Device: rows[0].Device, Inode: rows[0].Inode, PrefixSHA256: digest(prefix), Records: uint32(count), Generation: selected.binding.Generation, BindingSHA256: digest(selected.raw), Binding: selected.raw, OriginalContext: before, NativeContext: after, Manager: "physical-test-only"}, Origin: transitionOrigin{ProgramSHA256: selected.binding.ProgramSHA256}}
		original := &recoveryOriginal{prefix: &transitionPrefix{journal: &initialFile{body: prefix}}, cut: cut, result: &initialFile{body: []byte("detached cut fixture\n")}}
		candidate := &recoveryCandidate{selection: selected}
		raw, err := stoppedReplacementCompletion(original, candidate, previous, hashes, before, after)
		if err != nil {
			t.Fatal(count, err)
		}
		envelope, logical, err := decodeRecoveryCompletion(raw, prefix, original.result.body, cut)
		if err != nil || !bytes.Equal(logical, whole) || len(envelope.Records) != 62-count || !bytes.Equal(prefix, unchanged) {
			t.Fatal("prefix changed or stopped continuation differs", count, err)
		}
		for _, fault := range []string{"program", "binding", "before", "after", "manager", "ready-inode"} {
			bad := cut
			changed := append([]initialRecord(nil), rows[:count]...)
			switch fault {
			case "program":
				bad.Origin.ProgramSHA256 = digest([]byte("foreign executable"))
			case "binding":
				bad.Boundary.Binding = previous.raw
			case "before":
				bad.Boundary.OriginalContext = after
			case "after":
				bad.Boundary.NativeContext = before
			case "manager":
				bad.Boundary.Manager = "foreign-manager"
			case "ready-inode":
				changed[60].Inode++
			}
			if _, err := stoppedReplacementTranscript(encodeReplacementVector(t, changed), bad, selected, previous, hashes, before, after); err == nil {
				t.Fatal("foreign original proof accepted", count, fault)
			}
		}
	}
	for count := 0; count < 61; count++ {
		if _, err := stoppedReplacementTranscript(encodeReplacementVector(t, rows[:count]), transitionCutResult{}, selected, previous, hashes, before, after); err == nil {
			t.Fatal("earlier unsafe prefix accepted", count)
		}
	}
}
