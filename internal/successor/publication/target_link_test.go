package publication_test

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
)

func TestTargetLinkPreservesIndependentV3Vector(t *testing.T) {
	actual, err := publication.TargetLink([32]byte{51}, [32]byte{68})
	const expected = "ardents-target:v3:ATMAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAARAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	if err != nil || actual != expected {
		t.Fatal("canonical Target Link differs", actual, err)
	}
	for _, input := range [][2][32]byte{{}, {{51}, {}}, {{}, {68}}} {
		if _, err := publication.TargetLink(input[0], input[1]); err == nil {
			t.Fatal("unbound Target Link accepted")
		}
	}
}
