package durablefile

import "testing"

func TestSHA256HexCanonicalDigest(t *testing.T) {
	// Known SHA-256 of the empty input and of "abc".
	if got := SHA256Hex(nil); got != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("empty digest = %s", got)
	}
	if got := SHA256Hex([]byte("abc")); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("abc digest = %s", got)
	}
	if !GenerationName.MatchString(SHA256Hex([]byte("abc"))) {
		t.Fatal("digest is not a canonical generation name")
	}
}

func TestGenerationNameGrammar(t *testing.T) {
	valid := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if !GenerationName.MatchString(valid) {
		t.Fatal("canonical digest refused")
	}
	for _, invalid := range []string{"", "ABC", valid[:63], valid + "0", "g" + valid[1:], valid + "\n"} {
		if GenerationName.MatchString(invalid) {
			t.Fatalf("non-canonical name accepted: %q", invalid)
		}
	}
}
