package endpoint

import (
	"runtime"
	"time"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
)

const metadataRoot = "https://release.invalid/metadata/"

// InitialInputs projects frozen Enrollment bytes for initial Release evaluation.
func InitialInputs(bundle enrollment.Bundle, ref time.Time) (release.Inputs, bool) {
	f, ok := bundle.Facts()
	if !ok {
		return release.Inputs{}, false
	}
	return projectInputs(bundle, f, ref)
}

// CandidateInputs projects a self-consistent inventory for retained Release.
// This projection does not grant initial trust or authorization.
func CandidateInputs(candidate enrollment.Candidate, ref time.Time) (release.Inputs, bool) {
	f, ok := candidate.Facts()
	if !ok {
		return release.Inputs{}, false
	}
	return projectInputs(candidate, enrollment.Facts(f), ref)
}

func projectInputs(source inventory, facts enrollment.Facts, ref time.Time) (release.Inputs, bool) {
	root, rootOK := source.File(facts.TrustedRoot)
	program, programOK := source.File(facts.Artifact)
	if !rootOK || !programOK {
		return release.Inputs{}, false
	}
	files := make(map[string][]byte)
	for _, name := range source.MetadataNames() {
		files[metadataRoot+name], _ = source.File(name)
	}
	return release.Inputs{RootBytes: root, Artifact: program, Files: files, TargetPath: facts.TargetPath,
		Local: release.LocalEnvironment{Platform: facts.Platform, Architecture: runtime.GOARCH,
			Environment: facts.Environment, Network: facts.Network, RefTime: ref.UTC()}}, true
}
