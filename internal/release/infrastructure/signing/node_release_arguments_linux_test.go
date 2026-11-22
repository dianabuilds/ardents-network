package signing

import (
	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"testing"
)

// The shared public-option grammar keeps Go flag's own option predicate: a
// bare "-" is a terminator, not an option named "" (audit E1-3). Every current
// caller pairs the grammar with flags.NArg()==0, so a bare "-" still refuses
// through flag's terminator handling before any output is created.
func TestUniqueOptionsGrammarPinsFlagCompatiblePredicate(t *testing.T) {
	for _, args := range [][]string{
		{"-", "-"},
		{"-", "--duty", "a"},
		{"positional", "-", "positional"},
	} {
		if !artifactio.UniqueOptions(args, "duty") {
			t.Fatal("bare terminator treated as a repeatable option", args)
		}
	}
	if !artifactio.UniqueOptions([]string{"--duty", "a", "--duty=b"}, "duty") {
		t.Fatal("declared repeatable option refused")
	}
	if artifactio.UniqueOptions([]string{"--duty", "a", "--duty=b"}) {
		t.Fatal("undeclared repetition permits implicit last-wins")
	}
	if artifactio.UniqueOptions([]string{"--network", "a", "--network=b"}, "duty") {
		t.Fatal("non-repeatable option repeated under another spelling")
	}
	if !artifactio.UniqueOptions([]string{"--domain", "a", "--domain=b", "--record-plan", "a", "--record-plan=b", "--record", "a", "--record=b"}, "domain", "record-plan", "record") {
		t.Fatal("Epoch repeatable set refused")
	}
	if artifactio.UniqueOptions([]string{"--assignment-seed", "a", "--assignment-seed=b"}, "domain", "record-plan", "record") {
		t.Fatal("option outside the repeatable set repeated")
	}
}
