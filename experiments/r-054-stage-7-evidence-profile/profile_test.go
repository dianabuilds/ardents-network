//go:build ignore

package profile

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCanonicalCampaignRoundTrips(t *testing.T) {
	raw, err := canonical(referenceCampaign())
	if err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 100; round++ {
		admitted, err := admitCampaign(raw)
		if err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
		next, err := canonical(admitted)
		if err != nil || !bytes.Equal(raw, next) {
			t.Fatalf("round %d changed bytes: %v", round, err)
		}
		raw = next
	}
}

func TestCampaignMutationsReject(t *testing.T) {
	valid, err := canonical(referenceCampaign())
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string][]byte{
		"unknown-field":   insertBeforeFinal(valid, `,"verdict":"pass"`),
		"missing-field":   bytes.Replace(valid, []byte(`"source_clean":true,`), nil, 1),
		"duplicate-field": insertBeforeFinal(valid, `,"schema":"1"`),
		"leading-space":   append([]byte(" "), valid...),
		"trailing-byte":   append(append([]byte(nil), valid...), '\n'),
		"wrong-schema":    bytes.Replace(valid, []byte(`"schema":"1"`), []byte(`"schema":"2"`), 1),
		"upper-digest":    bytes.Replace(valid, []byte(strings.Repeat("a", 64)), []byte(strings.Repeat("A", 64)), 1),
		"ordinal-gap":     bytes.Replace(valid, []byte(`"ordinal":1`), []byte(`"ordinal":3`), 1),
		"oversized":       bytes.Repeat([]byte{'x'}, maximumCampaignJSON+1),
	}
	for name, mutated := range mutations {
		t.Run(name, func(t *testing.T) {
			if _, err := admitCampaign(mutated); err == nil {
				t.Fatal("mutation was accepted")
			}
		})
	}
}

func TestRelativePathConfinement(t *testing.T) {
	accepted := []string{
		"evidence/index.json",
		"evidence/cells/000/attempts/000/terminal.json",
	}
	rejected := []string{
		"", "/absolute", "../escape", "evidence/../escape", `c:\escape`,
		`evidence\escape`, "evidence/%2e%2e/escape", "evidence/CON/file",
		"evidence/com9.log/file", "evidence/lpt7/file", "evidence/file.",
		"evidence//gap", "evidence/UPPER",
	}
	for _, value := range accepted {
		if !safeRelativePath(value) {
			t.Errorf("safe path rejected: %q", value)
		}
	}
	for _, value := range rejected {
		if safeRelativePath(value) {
			t.Errorf("unsafe path accepted: %q", value)
		}
	}
}

func TestVerdictPrecedenceIsDeterministic(t *testing.T) {
	passing := attemptFacts{true, true, true, true, true, true}
	behaviorFail := attemptFacts{true, true, true, false, true, true}
	cleanupFail := attemptFacts{true, true, true, true, true, false}
	observerInvalid := attemptFacts{true, false, true, false, true, false}
	secretInvalid := attemptFacts{true, true, false, true, true, true}

	cases := []struct {
		name  string
		facts []attemptFacts
		want  string
	}{
		{"empty", nil, "invalid"}, {"pass", []attemptFacts{passing}, "pass"},
		{"behavior", []attemptFacts{passing, behaviorFail}, "fail"},
		{"cleanup", []attemptFacts{cleanupFail}, "fail"},
		{"observer-before-behavior", []attemptFacts{behaviorFail, observerInvalid}, "invalid"},
		{"secret-before-pass", []attemptFacts{passing, secretInvalid}, "invalid"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := reduce(test.facts); got != test.want {
				t.Fatalf("got %s want %s", got, test.want)
			}
		})
	}
}

func TestMaximumEvidenceStreamsWithinLocalEnvelope(t *testing.T) {
	var before runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	started := time.Now()
	digest, err := streamDigest(io.LimitReader(zeroReader{}, int64(maximumEvidence)), maximumEvidence)
	elapsed := time.Since(started)
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	const zeroGiBSHA256 = "49bc20df15e412a64472421e13fe86ff1c5165e18b2afccf160d4dc19fe68a14"
	if digest != zeroGiBSHA256 {
		t.Fatalf("maximum stream digest %s", digest)
	}
	if elapsed > 60*time.Second {
		t.Fatalf("maximum stream took %s", elapsed)
	}
	allocated := after.TotalAlloc - before.TotalAlloc
	if allocated > 16<<20 {
		t.Fatalf("maximum stream allocated %d bytes", allocated)
	}
	t.Logf("streamed %d bytes in %s with %d allocated bytes", maximumEvidence, elapsed, allocated)

	if _, err := streamDigest(strings.NewReader("too long"), 1); err == nil {
		t.Fatal("length mismatch was accepted")
	}
	if _, err := streamDigest(strings.NewReader(""), maximumEvidence+1); err == nil {
		t.Fatal("oversized commitment was accepted before reading")
	}
}

type zeroReader struct{}

func (zeroReader) Read(buffer []byte) (int, error) {
	clear(buffer)
	return len(buffer), nil
}

func referenceCampaign() campaign {
	reference := func(ordinal int) digestReference {
		digest := sha256.Sum256([]byte(fmt.Sprintf("reference-%d", ordinal)))
		return digestReference{Ordinal: uint16(ordinal), SHA256: fmt.Sprintf("%x", digest)}
	}
	hosts := []digestReference{reference(0), reference(1)}
	cells := make([]digestReference, logicalCellCount)
	for index := range cells {
		cells[index] = reference(index)
	}
	return campaign{
		Schema: schemaVersion, Profile: profileID, RunID: strings.Repeat("a", 64),
		SourceCommit: strings.Repeat("b", 64), SourceClean: true,
		MaximumEvidence: maximumEvidence, Hosts: hosts, Cells: cells,
	}
}

func insertBeforeFinal(input []byte, addition string) []byte {
	result := append([]byte(nil), input[:len(input)-1]...)
	result = append(result, addition...)
	return append(result, '}')
}
