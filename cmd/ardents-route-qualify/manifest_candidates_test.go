package main

import (
	"strings"
	"testing"

	qualification "github.com/dianabuilds/ardents-network/internal/qualification/route"
)

func TestResolveCandidatesPreservesNullIdentityExclusions(t *testing.T) {
	var input qualification.Case
	zero := strings.Repeat("00", 32)
	value := manifest{Candidates: []manifestCandidate{{NodeID: zero, PublicKey: zero}}}
	if err := resolveCandidates(&input, value); err != nil {
		t.Fatal(err)
	}
	if input.ExcludedIdentities != nil {
		t.Fatal("null identity exclusions became a non-nil empty slice")
	}
}
