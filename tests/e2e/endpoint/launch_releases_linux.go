//go:build linux && endpoint_launch_native

package endpoint

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	releasefixture "github.com/dianabuilds/ardents-network/tests/releasefixture"
)

// launchReleases identifies the signed distributions used to stage Endpoint generations.
type launchReleases struct {
	Initial, Pin, Program string
	Candidate, Successor  string
}

// buildLaunchReleases signs distinct metadata for provisioning, first staging and replacement.
func buildLaunchReleases(t *testing.T, binary, base string, successor bool, predecessor *EnrollmentV5Artifacts) launchReleases {
	t.Helper()
	// Genuine Release authority + enrolled protected candidate (version 1). The
	// provision command authenticates the initial pair and establishes floors.
	rootAuthority := releasefixture.NewReleaseAuthority(t, time.Date(2031, 1, 2, 3, 4, 5, 0, time.UTC))
	resources := NativeResources()
	// The worker executable must be a genuine compiled text worker, not fixture
	// placeholder bytes: socket activation spawns it under confinement.
	resources["ardents-text-linux-amd64"] = BuildNativeWorker(t, "")
	var legacyCustody [][]byte
	if predecessor != nil {
		resources["ardents-linux-amd64"] = predecessor.Program
		legacyCustody = append(legacyCustody, predecessor.Custody)
	}
	bundle, pin, _ := releasefixture.SignedRelease(t, binary, "linux-amd64", true, true, 1, resources, rootAuthority, 1, 1, legacyCustody...)
	bundle, initialProgram := CopyNativeDistribution(t, bundle, filepath.Join(base, "initial-distribution"))
	// Provision establishes Release floors at metadata version 1.
	// Advance the signed metadata while retaining the exact staged release and descriptor.
	candidateBundle, _, _ := releasefixture.SignedRelease(t, binary, "linux-amd64", true, true, 1, resources, rootAuthority, 2, 1, legacyCustody...)
	candidate, _ := CopyNativeDistribution(t, candidateBundle, filepath.Join(base, "candidate-distribution"))
	releases := launchReleases{Initial: bundle, Pin: pin, Program: initialProgram, Candidate: candidate}
	if successor {
		current, err := os.ReadFile(binary)
		if err != nil {
			t.Fatal(err)
		}
		if predecessor == nil {
			current = append(current, []byte("signed successor fixture")...)
		}
		resources["ardents-linux-amd64"] = current
		updated, _, _ := releasefixture.SignedRelease(t, binary, "linux-amd64", true, true, 2, resources, rootAuthority, 3, 2)
		releases.Successor, _ = CopyNativeDistribution(t, updated, filepath.Join(base, "successor-distribution"))
	}
	return releases
}
