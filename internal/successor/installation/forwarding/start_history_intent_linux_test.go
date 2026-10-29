package forwarding

import (
	"bytes"
	"testing"
)

func TestForwardingHistoryIntentBindsChoiceSeparatelyFromAttempt(t *testing.T) {
	b, raw, _ := selectedGenerationVector(t)
	bh, ih, th, ph := digest(raw), digest([]byte("original initial")), digest([]byte("targets")), digest([]byte("predecessor"))
	for _, number := range []attemptNumber{0, 7} {
		predecessor := ph
		if number == 0 {
			predecessor = ""
		}
		for _, choice := range []uint8{0, 1} {
			guard, err := encodeHistoryStartIntent(b, bh, ih, th, number, predecessor, choice)
			if err != nil {
				t.Fatal(err)
			}
			history, extended, err := readStartHistoryIntent(guard, b, bh, ih, number, predecessor)
			if err != nil || !extended || history != choice {
				t.Fatal("attempt number replaced explicit history choice", number, choice, err)
			}
			if checkLegacyStartIntent(guard, b, bh, ih, number, predecessor) == nil {
				t.Fatal("new choice lost in a legacy intent")
			}
			for _, bad := range [][]byte{bytes.TrimSuffix(guard, []byte{'\n'}), bytes.Replace(guard, []byte("\"history\":"), []byte("\"history\":2,\"unknown\":"), 1), bytes.Replace(guard, []byte("\"history\":"), []byte("\"history\":0,\"history\":"), 1)} {
				if checkStartIntent(bad, b, bh, ih, number, predecessor) == nil {
					t.Fatal("ambiguous explicit history admitted")
				}
			}
		}
		legacy, err := encodeStartIntent(b, bh, ih, th, number, predecessor)
		if err != nil {
			t.Fatal(err)
		}
		history, extended, err := readStartHistoryIntent(legacy, b, bh, ih, number, predecessor)
		expected := uint8(0)
		if number != 0 {
			expected = 1
		}
		if err != nil || extended || history != expected {
			t.Fatal("legacy completed meaning changed", err)
		}
	}
	if _, err := encodeHistoryStartIntent(b, bh, ih, th, 0, "", 2); err == nil {
		t.Fatal("unknown history byte encoded")
	}
	// These byte choices prove no actual prior runtime or safe recovery. Root
	// derives accepting choice from the separately retained complete chain.
}
