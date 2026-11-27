package architecture

import (
	"go/build"
	"os"
	"path"
	"strings"
	"testing"
)

// The shared channel has no native mechanism. Check actual Go file selection,
// including tests: a passing suite must not silently omit an inherited OS file.
func TestSuccessorRouteChannelRunsSameSourcesOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/domain/route/channel")
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		for _, target := range []string{"windows", "linux"} {
			profile := build.Default
			profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
			selected, err := profile.MatchFile(directory, entry.Name())
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable channel source %s excluded on %s", entry.Name(), target)
			}
		}
	}
}

func TestSuccessorRouteRoleRulesRunOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/domain/route/role")
	for _, filename := range []string{"authority.go", "authority_test.go", "presentation.go", "presentation_test.go", "doc.go"} {
		for _, target := range []string{"windows", "linux"} {
			profile := build.Default
			profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
			selected, err := profile.MatchFile(directory, filename)
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable role rule %s excluded on %s", filename, target)
			}
		}
	}
}

// Prefix consumes retained facts and actual portable adapters. Durable roots
// belong to selection/Admission, not to its physical generation or borrowers.
func TestSuccessorRoutePrefixRunsSameSourcesOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/domain/route/prefix")
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		for _, target := range []string{"windows", "linux"} {
			profile := build.Default
			profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
			selected, err := profile.MatchFile(directory, name)
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable Prefix source %s excluded on %s", name, target)
			}
		}
	}
}

// Receiving pairing/relay opens no durable root and owns no native mechanism.
func TestSuccessorRouteReceivingJoinRunsOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/domain/route/join/pairing")
	for _, filename := range []string{"pair.go", "relay.go", "pair_test.go", "relay_test.go", "relay_eof_test.go", "doc.go"} {
		for _, target := range []string{"windows", "linux"} {
			profile := build.Default
			profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
			selected, err := profile.MatchFile(directory, filename)
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable receiving JOIN source %s excluded on %s", filename, target)
			}
		}
	}
}

// Holder exchange and inner/lower retirement use ordered I/O and exact
// borrowed generations. Native Context selection is a separate owner.
func TestSuccessorRouteHolderJoinRunsOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/domain/route/join")
	for _, filename := range []string{"acquisition.go", "acquisition_test.go", "client.go", "stream.go", "stream_test.go"} {
		for _, target := range []string{"windows", "linux"} {
			profile := build.Default
			profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
			selected, err := profile.MatchFile(directory, filename)
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable holder JOIN source %s excluded on %s", filename, target)
			}
		}
	}
}

// Directed-pair pool owns no native root or receiving Grant. Its actual
// physical retirement tests must execute on both platforms.
func TestSuccessorRouteReceiverPoolRunsOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/domain/route/receiver")
	for _, filename := range []string{"carrier_pool.go", "carrier_pool_test.go", "physical_fixture_test.go"} {
		for _, target := range []string{"windows", "linux"} {
			profile := build.Default
			profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
			selected, err := profile.MatchFile(directory, filename)
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable pool source %s excluded on %s", filename, target)
			}
		}
	}
}

// Holder registration consumes a retained Prefix, not a native storage owner.
func TestSuccessorRouteRegistrationRunsOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/domain/route/introduction")
	for _, filename := range []string{"registration.go", "registration_test.go"} {
		for _, target := range []string{"windows", "linux"} {
			profile := build.Default
			profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
			selected, err := profile.MatchFile(directory, filename)
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable registration source %s excluded on %s", filename, target)
			}
		}
	}
}

// Durable selection owns native root leases, but retained authority checks,
// store behavior and recipient choices must execute, including their tests, on
// both platforms. The physical lease-corruption and foreign-uid controls stay
// native (installation_cleanup_linux_test.go, closed_set_open_cleanup_linux_test.go,
// root_hardening_linux_test.go, root_lease_identity_unix_test.go).
func TestSuccessorRouteSelectionRulesRunOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/domain/route/selection")
	for _, filename := range []string{"leg.go", "leg_test.go", "entry_set.go", "entry_set_test.go", "introduction_recipient.go", "rendezvous.go", "rendezvous_test.go", "closed_sets_test.go", "closed_adjacent_domains_test.go", "generation_store_test.go", "interior_store_test.go", "interior_refusal_test.go", "installation_test.go", "installation_history_test.go", "selection_owner_test.go"} {
		for _, target := range []string{"windows", "linux"} {
			profile := build.Default
			profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
			selected, err := profile.MatchFile(directory, filename)
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable selection rule %s excluded on %s", filename, target)
			}
		}
	}
}

func TestSuccessorRouteIntroductionSnapshotRunsOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/domain/route/introduction")
	for _, filename := range []string{"slot_snapshot.go", "slot_snapshot_test.go", "slot_history.go", "slot_history_test.go", "registry.go", "receiving.go", "receiving_test.go"} {
		for _, target := range []string{"windows", "linux"} {
			profile := build.Default
			profile.GOOS, profile.GOARCH, profile.CgoEnabled = target, "amd64", false
			selected, err := profile.MatchFile(directory, filename)
			if err != nil {
				t.Fatal(err)
			}
			if !selected {
				t.Errorf("portable Introduction snapshot %s excluded on %s", filename, target)
			}
		}
	}
}
