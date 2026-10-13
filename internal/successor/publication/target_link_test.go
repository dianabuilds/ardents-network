package publication_test

import (
	"encoding/base64"
	"strings"
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

func TestParseTargetLinkIndependentV3Vector(t *testing.T) {
	const link = "ardents-target:v3:ATMAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAARAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	network, target, err := publication.ParseTargetLink(link)
	if err != nil || network != [32]byte{51} || target != [32]byte{68} {
		t.Fatal("independent Target Link vector differs", err)
	}
	raw := make([]byte, 65)
	raw[0], raw[1], raw[33] = 1, 51, 68
	malformed := []string{"", " " + link, link + " ", link + "=", link + "\n", strings.Replace(link, ":v3:", ":v2:", 1), strings.Replace(link, "target", "name", 1), link[:len(link)-1] + "B"}
	for _, field := range []int{0, 1, 33} {
		altered := append([]byte(nil), raw...)
		altered[field] = 0
		malformed = append(malformed, "ardents-target:v3:"+base64.RawURLEncoding.EncodeToString(altered))
	}
	for _, candidate := range malformed {
		if _, _, err := publication.ParseTargetLink(candidate); err == nil {
			t.Fatal("malformed, legacy or unbound Target Link accepted")
		}
	}
}
