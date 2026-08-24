package publication

import "testing"

func TestIntroductionInstructionV1BindsCurrentPublication(t *testing.T) {
	current := Current{Credential: Credential{Target: [32]byte{1}, Generation: 2}, Digest: [32]byte{3}}
	input := IntroductionInstruction{Target: current.Credential.Target, Generation: current.Credential.Generation,
		PublicationDigest: current.Digest, AttachmentID: [32]byte{4}}
	raw, err := EncodeIntroductionInstruction(input)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeIntroductionInstruction(raw)
	if err != nil || decoded != input {
		t.Fatalf("DecodeIntroductionInstruction = %+v, %v", decoded, err)
	}
	if err := current.ValidateIntroductionInstruction(decoded); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*IntroductionInstruction){
		"target":     func(value *IntroductionInstruction) { value.Target[0]++ },
		"generation": func(value *IntroductionInstruction) { value.Generation++ },
		"digest":     func(value *IntroductionInstruction) { value.PublicationDigest[0]++ },
	} {
		t.Run(name, func(t *testing.T) {
			changed := decoded
			mutate(&changed)
			if err := current.ValidateIntroductionInstruction(changed); err == nil {
				t.Fatal("publication substitution was accepted")
			}
		})
	}
}

func TestIntroductionInstructionV1RejectsNonCanonicalBytes(t *testing.T) {
	input := IntroductionInstruction{Target: [32]byte{1}, Generation: 2, PublicationDigest: [32]byte{3}, AttachmentID: [32]byte{4}}
	raw, err := EncodeIntroductionInstruction(input)
	if err != nil {
		t.Fatal(err)
	}
	for index, value := range [][]byte{nil, raw[:len(raw)-1], append(raw, 0)} {
		if _, err := DecodeIntroductionInstruction(value); err == nil {
			t.Fatalf("mutation %d was accepted", index)
		}
	}
}
