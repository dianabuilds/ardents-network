package architecture

import (
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"
)

// Enrollment grammar and all portable behavior controls must be selected on
// both platforms. Only concrete file ownership/open adapters have native tags.
func TestSuccessorEnrollmentRunsPortableRulesOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/successor/enrollment")
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		switch name {
		case "file_unix.go", "file_unix_test.go", "file_windows.go", "file_unsupported.go":
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
				t.Errorf("portable Enrollment source %s excluded on %s", name, target)
			}
		}
	}
}

// The shared channel has no native mechanism. Check actual Go file selection,
// including tests: a passing suite must not silently omit an inherited OS file.
func TestSuccessorRouteChannelRunsSameSourcesOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/successor/route/channel")
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
	directory := path.Join(repositoryRoot(t), "internal/successor/route/role")
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
	directory := path.Join(repositoryRoot(t), "internal/successor/route/prefix")
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
	directory := path.Join(repositoryRoot(t), "internal/successor/route/join")
	for _, filename := range []string{"pair.go", "relay.go", "pair_test.go", "relay_test.go", "doc.go"} {
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
	directory := path.Join(repositoryRoot(t), "internal/successor/route/join")
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
	directory := path.Join(repositoryRoot(t), "internal/successor/route/receiver")
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
	directory := path.Join(repositoryRoot(t), "internal/successor/route/introduction")
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

// Durable selection owns native root leases, but retained authority checks and
// recipient choices must execute, including their tests, on both platforms.
func TestSuccessorRouteSelectionRulesRunOnWindowsAndLinux(t *testing.T) {
	directory := path.Join(repositoryRoot(t), "internal/successor/route/selection")
	for _, filename := range []string{"leg.go", "leg_test.go", "entry_set.go", "entry_set_test.go", "introduction_recipient.go", "rendezvous.go", "rendezvous_test.go"} {
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
	directory := path.Join(repositoryRoot(t), "internal/successor/route/introduction")
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

func TestSuccessorImportIsolation(t *testing.T) {
	root := repositoryRoot(t)
	walk(t, root, func(filename string, entry os.DirEntry) {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			return
		}
		relative := relativePath(t, root, filename)
		if !strings.HasPrefix(relative, "internal/") && !strings.HasPrefix(relative, "cmd/") && !strings.HasPrefix(relative, "tests/") {
			return
		}
		file, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range file.Imports {
			dependency, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if !successorImportAllowed(relative, dependency) {
				t.Errorf("successor isolation: %s imports forbidden dependency %s", relative, dependency)
			}
		}
	})
}

// This check is independently runnable with the new-domain regression. It
// checks actual implementation imports without executing legacy consumers.
func TestNetworkIntegrationImportIsolation(t *testing.T) {
	root := repositoryRoot(t)
	walk(t, root, func(filename string, entry os.DirEntry) {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			return
		}
		relative := relativePath(t, root, filename)
		if !strings.HasPrefix(relative, "internal/successor/") && !strings.HasPrefix(relative, "cmd/ardents-next/") {
			return
		}
		file, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range file.Imports {
			dependency, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if !successorImportAllowed(relative, dependency) {
				t.Errorf("new-domain isolation: %s imports forbidden dependency %s", relative, dependency)
			}
		}
	})
}

func successorImportAllowed(source, dependency string) bool {
	inZone := strings.HasPrefix(source, "internal/successor/") || strings.HasPrefix(source, "cmd/ardents-next/")
	zoneDependency := dependency == modulePath+"/internal/successor" || strings.HasPrefix(dependency, modulePath+"/internal/successor/")
	// Network's physical/application adapters consume domain policy through
	// exact files. Directory nesting does not grant policy or Admission access.
	if strings.HasPrefix(source, "internal/successor/network/") {
		sourceAdapter := source
		if sourceAdapter == "internal/successor/network/closedprofile/token_spki.go" && dependency == modulePath+"/internal/successor/admission/issuerprofile" {
			return true
		}
		if sourceAdapter == "internal/successor/network/duty/participation_policy.go" && dependency == modulePath+"/internal/successor/network" {
			return true
		}
		if dependency == modulePath+"/internal/successor/network" {
			switch sourceAdapter {
			case "internal/successor/network/state/accepted_observation.go":
				return inZone
			case "internal/successor/network/state/closed_profile_role_join_test.go", "internal/successor/network/state/closed_runtime_view_test.go":
				return inZone
			case "internal/successor/network/state/acquisition_history.go":
				return true
			case "internal/successor/network/epoch/candidate_evaluation.go", "internal/successor/network/epoch/assignment.go":
				return true
			case "internal/successor/network/state/epoch_history.go", "internal/successor/network/state/membership.go", "internal/successor/network/state/closed_member.go", "internal/successor/network/state/closed_runtime_view.go", "internal/successor/network/state/runtime_observation.go", "internal/successor/network/state/clock_observation.go", "internal/successor/network/state/closed_profile_view.go", "internal/successor/network/state/closed_duty_binding.go", "internal/successor/network/state/closed_profile_accept.go":
				return true
			}
		}
	}
	if !inZone {
		return !zoneDependency
	}
	// Match exact Go packages. A parent folder grants no child/sibling imports.
	owner := path.Dir(source)
	permitted := map[string][]string{
		"internal/successor/network":                 {},
		"internal/successor/network/closedprofile":   {},
		"internal/successor/network/duty":            {},
		"internal/successor/network/epoch":           {},
		"internal/successor/network/source":          {"network/epoch"},
		"internal/successor/network/state":           {"network/closedprofile", "network/duty", "network/epoch", "network/source", "network/state/durable"},
		"internal/successor/network/state/durable":   {},
		"internal/successor/admission":               {"admission/issuerprofile"},
		"internal/successor/admission/issuerprofile": {},
		"internal/successor/admission/allocation":    {"admission"},
		"internal/successor/admission/attempts":      {},
		"internal/successor/admission/spending":      {},
		"internal/successor/admission/token":         {"admission", "admission/issuerprofile"},
		"internal/successor/admission/receiving":     {"admission", "admission/token", "admission/spending"},
		"internal/successor/admission/stock":         {"admission", "admission/allocation", "admission/attempts", "admission/token"},
		"internal/successor/admission/quota":         {"admission", "admission/issuerprofile"},
		"internal/successor/admission/issuance":      {"admission", "admission/quota", "admission/issuerprofile", "nodeidentity"},
		"internal/successor/admission/issuer":        {"admission", "admission/quota", "admission/issuance", "nodeidentity"},
		"internal/successor/nodeidentity":            {"admission/issuerprofile"},
		"internal/successor/hosting":                 {},
		"internal/successor/publication":             {},
		"internal/successor/reachability":            {"publication"},
		"internal/successor/enrollment":              {},
		"internal/successor/release":                 {},
		"internal/successor/execution":               {},
		"internal/successor/execution/runtime":       {"execution", "execution/worker"},
		"internal/successor/execution/worker":        {"installation/systemd", "installation/unit"},
		"internal/successor/installation":            {"enrollment", "release", "installation/cgroup", "installation/systemd", "installation/journal", "installation/process", "installation/generation", "installation/completion", "installation/fixedfile", "installation/unit", "installation/request"},
		"internal/successor/installation/cgroup":     {},
		"internal/successor/installation/request":    {},
		"internal/successor/installation/systemd":    {},
		"internal/successor/installation/journal":    {},
		"internal/successor/installation/process":    {},
		"internal/successor/installation/generation": {},
		"internal/successor/installation/completion": {},
		"internal/successor/installation/fixedfile":  {},
		"internal/successor/installation/unit":       {"installation/systemd"},
		"internal/successor/route":                   {},
		"internal/successor/route/ardp":              {},
		"internal/successor/route/issuer":            {"route/ardp", "admission"},
		"internal/successor/route/transport":         {},
		"internal/successor/route/channel":           {"route/ardp", "route/transport"},
		"internal/successor/route/bootstrap":         {"route/ardp", "route/channel"},
		"internal/successor/route/transport/tls":     {"route/transport"},
		"internal/successor/route/transport/quic":    {"route/transport"},
		"internal/successor/route/introduction":      {"admission/spending", "route/ardp", "route/role", "route/prefix", "network", "admission"},
		"internal/successor/route/selection":         {"route", "network"},
		"internal/successor/route/role":              {"route", "route/ardp", "route/channel", "route/transport", "network", "admission"},
		"internal/successor/route/join":              {"route/prefix", "route", "route/selection", "route/ardp", "route/channel", "route/role", "route/transport", "network", "admission"},
		"internal/successor/route/prefix":            {"route/issuer", "route", "route/selection", "route/ardp", "route/bootstrap", "route/channel", "route/role", "route/transport", "route/transport/tls", "route/transport/quic", "network", "admission"},
		"internal/successor/route/receiver":          {"route/issuer", "route/join", "route", "route/role", "route/transport", "route/transport/tls", "route/transport/quic", "route/channel", "route/bootstrap", "route/ardp", "route/introduction", "network", "admission", "admission/receiving"},
		"cmd/ardents-next":                           {"route/introduction", "route/prefix", "network", "network/state", "admission/stock", "admission/receiving", "admission/allocation", "admission", "admission/quota", "admission/issuerprofile", "admission/issuance", "admission/issuer", "nodeidentity", "hosting", "route", "route/role", "route/selection", "route/join", "route/receiver", "route/channel", "route/ardp"},
	}
	if zoneDependency {
		if source == "cmd/ardents-next/installed_endpoint_linux.go" && (dependency == modulePath+"/internal/successor/installation" || dependency == modulePath+"/internal/successor/execution" || dependency == modulePath+"/internal/successor/execution/runtime") {
			return true
		}
		if source == "cmd/ardents-next/installed_source_linux.go" && (dependency == modulePath+"/internal/successor/execution" || dependency == modulePath+"/internal/successor/network/source") {
			return true
		}
		if (source == "cmd/ardents-next/execution_holder.go" || source == "cmd/ardents-next/admission_holder.go") && dependency == modulePath+"/internal/successor/execution/runtime" {
			return true
		}
		if source == "cmd/ardents-next/execution_holder.go" && dependency == modulePath+"/internal/successor/execution" {
			return true
		}
		if (source == "cmd/ardents-next/installation.go" || source == "cmd/ardents-next/installation_test.go") && (dependency == modulePath+"/internal/successor/installation" || dependency == modulePath+"/internal/successor/enrollment" || dependency == modulePath+"/internal/successor/release") {
			return true
		}
		if source == "cmd/ardents-next/release_process_test.go" && (dependency == modulePath+"/internal/successor/enrollment" || dependency == modulePath+"/internal/successor/release") {
			return true
		}
		if dependency == modulePath+"/internal/successor/enrollment" && (source == "cmd/ardents-next/enrollment.go" || source == "cmd/ardents-next/release.go") {
			return true
		}
		if dependency == modulePath+"/internal/successor/release" && source == "cmd/ardents-next/release.go" {
			return true
		}
		if source == "cmd/ardents-next/route_bootstrap_linux_test.go" && (dependency == modulePath+"/internal/successor/route/issuer" || dependency == modulePath+"/internal/successor/route/bootstrap") {
			return true
		}
		if source == "cmd/ardents-next/route_fixture_linux_test.go" && (dependency == modulePath+"/internal/successor/route/transport/tls" || dependency == modulePath+"/internal/successor/route/transport/quic") {
			return true
		}
		if dependency == modulePath+"/internal/successor/route/transport" {
			if source == "cmd/ardents-next/execution_installed_linux_test.go" {
				return true
			}
			if source == "cmd/ardents-next/route_descriptor_linux_test.go" {
				return true
			}
			if source == "cmd/ardents-next/route_bootstrap_linux_test.go" {
				return true
			}
			switch source {
			case "cmd/ardents-next/route_refill_linux_test.go", "cmd/ardents-next/route_fixture_linux_test.go", "cmd/ardents-next/route_prefix_linux_test.go", "cmd/ardents-next/route_prefix_caller_linux_test.go", "cmd/ardents-next/route_process_linux_test.go", "cmd/ardents-next/route_refusal_linux_test.go", "cmd/ardents-next/route_registration_linux_test.go", "cmd/ardents-next/route_join_fixture_linux_test.go", "cmd/ardents-next/route_join_linux_test.go", "cmd/ardents-next/route_join_process_linux_test.go", "cmd/ardents-next/route_close_failure_linux_test.go", "cmd/ardents-next/route_join_lifecycle_linux_test.go":
				return true
			}
		}
		if source == "cmd/ardents-next/network_admission_fixture_linux_test.go" && dependency == modulePath+"/internal/successor/network/epoch" {
			return true
		}
		if owner == "cmd/ardents-next" && strings.HasSuffix(source, "_test.go") && (dependency == modulePath+"/internal/successor/admission/token" || dependency == modulePath+"/internal/successor/admission/spending") {
			return true
		}
		if source == "cmd/ardents-next/route_issuer_linux.go" && (dependency == modulePath+"/internal/successor/admission/token" || dependency == modulePath+"/internal/successor/route/bootstrap") {
			return true
		}
		if dependency == modulePath+"/internal/successor/reachability" && (source == "internal/successor/route/prefix/descriptor_exchange.go" || source == "internal/successor/route/receiver/receiver_linux.go" || source == "internal/successor/route/receiver/descriptor_linux.go" || source == "cmd/ardents-next/route_linux.go" || source == "cmd/ardents-next/route.go" || source == "cmd/ardents-next/admission_holder.go" || source == "cmd/ardents-next/route_descriptor_linux_test.go") {
			return true
		}
		if (source == "internal/successor/admission/stock/issuance_fixture_test.go" || source == "internal/successor/admission/stock/lifecycle_test.go" || source == "internal/successor/admission/stock/receiving_cycle_test.go") && (dependency == modulePath+"/internal/successor/admission/issuer" || dependency == modulePath+"/internal/successor/admission/issuance" || dependency == modulePath+"/internal/successor/admission/quota") {
			return true
		}
		if (source == "internal/successor/admission/stock/cycle_test.go" || source == "internal/successor/admission/stock/receiving_cycle_test.go") && (dependency == modulePath+"/internal/successor/admission/receiving" || dependency == modulePath+"/internal/successor/admission/spending") {
			return true
		}
		// A registered package's external behavior tests may import that package.
		// This grants neither another package nor nested tests its imports.
		if _, registered := permitted[owner]; registered && strings.HasSuffix(source, "_test.go") && dependency == modulePath+"/"+owner {
			return true
		}
		for _, allowed := range permitted[owner] {
			if dependency == modulePath+"/internal/successor/"+allowed {
				return true
			}
		}
		return false
	}
	if source == "internal/successor/execution/runtime/initialization_linux.go" && dependency == modulePath+"/internal/application/textdocument" {
		return true
	}
	if (owner == "internal/successor/admission/issuance" || owner == "internal/successor/admission/token") && dependency == "github.com/cloudflare/circl/blindsign/blindrsa" {
		return true
	}
	if owner == "internal/successor/route/transport/quic" && dependency == "github.com/quic-go/quic-go" {
		return true
	}
	if source == "internal/successor/installation/systemd/endpoint_reference_linux.go" && dependency == "github.com/godbus/dbus/v5" {
		return true
	}
	if (owner == "internal/successor/admission/spending" || owner == "internal/successor/network/duty" || owner == "internal/successor/network/state/durable") && dependency == "golang.org/x/sys/windows" {
		return true
	}
	if source == "internal/successor/reachability/store_platform_windows.go" && dependency == "golang.org/x/sys/windows" {
		return true
	}
	if source == "internal/successor/release/history_native_windows.go" && dependency == "golang.org/x/sys/windows" {
		return true
	}
	if owner == "internal/successor/release" && dependency == "github.com/theupdateframework/go-tuf/v2/metadata" {
		switch source {
		case "internal/successor/release/metadata_authentication.go", "internal/successor/release/target_identity.go", "internal/successor/release/safety_policy.go", "internal/successor/release/floor_encoding.go":
			return true
		}
	}
	if source == "internal/successor/release/metadata_authentication.go" && dependency == "github.com/theupdateframework/go-tuf/v2/metadata/trustedmetadata" {
		return true
	}
	if strings.HasSuffix(source, "_test.go") {
		if source == "internal/successor/release/generation_authorization_test.go" && dependency == "github.com/theupdateframework/go-tuf/v2/metadata" {
			return true
		}
		if source == "internal/successor/release/rotation_test.go" && (dependency == "github.com/sigstore/sigstore/pkg/signature" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata") {
			return true
		}
		if source == "cmd/ardents-next/release_process_test.go" && (dependency == "github.com/sigstore/sigstore/pkg/signature" || dependency == "github.com/theupdateframework/go-tuf/v2/metadata") {
			return true
		}
		if (owner == "internal/successor/network/epoch" || owner == "internal/successor/network/state" || owner == "cmd/ardents-next") && dependency == modulePath+"/tests/epochfixture/network" {
			return true
		}
		if owner == "internal/successor/network/epoch" && dependency == modulePath+"/tests/epochfixture/assignment" {
			return true
		}
	}

	// OTel composition is confined to the exact command package.
	// No arbitrary third-party bridge is permitted.
	if owner == "cmd/ardents-next" {
		for _, allowed := range []string{
			"go.opentelemetry.io/otel/attribute",
			"go.opentelemetry.io/otel/metric",
			"go.opentelemetry.io/otel/sdk/metric",
			"go.opentelemetry.io/otel/sdk/metric/exemplar",
			"go.opentelemetry.io/otel/sdk/metric/metricdata",
			"go.opentelemetry.io/otel/sdk/resource",
			"go.opentelemetry.io/otel/sdk/trace",
			"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp",
			"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp",
		} {
			if dependency == allowed {
				return true
			}
		}
		if strings.HasSuffix(source, "_test.go") {
			for _, allowed := range []string{"github.com/cloudflare/circl/blindsign/blindrsa", "go.opentelemetry.io/proto/otlp/collector/trace/v1", "go.opentelemetry.io/proto/otlp/collector/metrics/v1", "google.golang.org/protobuf/proto"} {
				if dependency == allowed {
					return true
				}
			}
		}
	}
	// Go reserves first import-path elements without a dot for standard-library
	// paths. Unknown paths still fail compilation; relative paths are forbidden.
	first := strings.Split(dependency, "/")[0]
	return dependency != "C" && dependency != "unsafe" && dependency != "" && !strings.Contains(first, ".") && path.Clean(dependency) == dependency && !strings.HasPrefix(dependency, "/")
}

func TestSuccessorIsolationPolicy(t *testing.T) {
	for _, test := range []struct {
		name, source, dependency string
		allowed                  bool
	}{
		{"Execution launch borrows only Text INIT grammar", "internal/successor/execution/runtime/initialization_linux.go", modulePath + "/internal/application/textdocument", true},
		{"Execution rules cannot borrow Text runtime", "internal/successor/execution/job.go", modulePath + "/internal/application/textdocument", false},
		{"common Execution launch has no native Text exception", "internal/successor/execution/runtime/launch.go", modulePath + "/internal/application/textdocument", false},
		{"other Execution composition has no Text exception", "internal/successor/execution/runtime/other_linux.go", modulePath + "/internal/application/textdocument", false},
		{"Execution cannot borrow old Endpoint authority", "internal/successor/execution/runtime/initialization_linux.go", modulePath + "/internal/endpoint", false},
		{"Execution cannot borrow old Broker", "internal/successor/execution/authority.go", modulePath + "/internal/application/broker", false},
		{"old Endpoint cannot consume new Execution", "internal/endpoint/worker_grant_linux.go", modulePath + "/internal/successor/execution", false},
		{"native original manager reference owns bus client", "internal/successor/installation/systemd/endpoint_reference_linux.go", "github.com/godbus/dbus/v5", true},
		{"manager transport excludes bus client", "internal/successor/installation/systemd/reference_transport_linux.go", "github.com/godbus/dbus/v5", false},
		{"Installation decisions exclude bus client", "internal/successor/installation/predecessor_linux.go", "github.com/godbus/dbus/v5", false},
		{"unregistered manager file excludes bus client", "internal/successor/installation/systemd/other_linux.go", "github.com/godbus/dbus/v5", false},
		{"Reachability verifies public delegation", "internal/successor/reachability/descriptor.go", modulePath + "/internal/successor/publication", true},
		{"Prefix completes genuine private lookup", "internal/successor/route/prefix/descriptor_exchange.go", modulePath + "/internal/successor/reachability", true},
		{"Receiver uses genuine Descriptor Store", "internal/successor/route/receiver/descriptor_linux.go", modulePath + "/internal/successor/reachability", true},
		{"Carrier cannot acquire Descriptor authority", "internal/successor/route/transport/tls/node.go", modulePath + "/internal/successor/reachability", false},
		{"Reachability cannot own physical Prefix", "internal/successor/reachability/lookup_history.go", modulePath + "/internal/successor/route/prefix", false},
		{"Store native Windows lease and sync", "internal/successor/reachability/store_platform_windows.go", "golang.org/x/sys/windows", true},
		{"Descriptor rules cannot use native Windows mechanisms", "internal/successor/reachability/descriptor.go", "golang.org/x/sys/windows", false},
		{"Store rules cannot use native Windows mechanisms", "internal/successor/reachability/store.go", "golang.org/x/sys/windows", false},
		{"public proof cannot acquire Route transport", "internal/successor/publication/public_proof.go", modulePath + "/internal/successor/route/ardp", false},
		{"public proof cannot acquire Store", "internal/successor/publication/public_proof.go", modulePath + "/internal/successor/reachability", false},
		{"Reachability cannot delegate to old verifier", "internal/successor/reachability/descriptor.go", modulePath + "/internal/service/reachability", false},
		{"public proof cannot delegate to old Publication", "internal/successor/publication/public_proof.go", modulePath + "/internal/service/publication", false},
		{"physical implementation uses shared contract", "internal/successor/route/transport/tls/node.go", modulePath + "/internal/successor/route/transport", true},
		{"Prefix uses shared contract", "internal/successor/route/prefix/prefix.go", modulePath + "/internal/successor/route/transport", true},
		{"Prefix uses portable framing", "internal/successor/route/prefix/prefix.go", modulePath + "/internal/successor/route/channel", true},
		{"channel uses grammar", "internal/successor/route/channel/session.go", modulePath + "/internal/successor/route/ardp", true},
		{"bootstrap consumes framing capacity", "internal/successor/route/bootstrap/budget.go", modulePath + "/internal/successor/route/channel", true},
		{"bootstrap describes canonical restriction", "internal/successor/route/bootstrap/budget.go", modulePath + "/internal/successor/route/ardp", true},
		{"receiver reserves bootstrap capacity", "internal/successor/route/receiver/receiver_linux.go", modulePath + "/internal/successor/route/bootstrap", true},
		{"channel cannot acquire bootstrap policy", "internal/successor/route/channel/session.go", modulePath + "/internal/successor/route/bootstrap", false},
		{"bootstrap cannot spend Admission", "internal/successor/route/bootstrap/budget.go", modulePath + "/internal/successor/admission/receiving", false},
		{"bootstrap cannot import receiving composition", "internal/successor/route/bootstrap/budget.go", modulePath + "/internal/successor/route/receiver", false},
		{"old receiver cannot import new bootstrap", "internal/node/forwarding/link.go", modulePath + "/internal/successor/route/bootstrap", false},
		{"channel uses ordered transport", "internal/successor/route/channel/session.go", modulePath + "/internal/successor/route/transport", true},
		{"channel cannot import TLS adapter", "internal/successor/route/channel/session.go", modulePath + "/internal/successor/route/transport/tls", false},
		{"channel cannot import QUIC adapter", "internal/successor/route/channel/session_test.go", modulePath + "/internal/successor/route/transport/quic", false},
		{"holder JOIN cannot import Receiver", "internal/successor/route/join/acquisition.go", modulePath + "/internal/successor/route/receiver", false},
		{"Receiver cannot import holder Prefix", "internal/successor/route/receiver/receiver_linux.go", modulePath + "/internal/successor/route/prefix", false},
		{"actual receiver dispatches JOIN", "internal/successor/route/receiver/receiver_linux.go", modulePath + "/internal/successor/route/join", true},
		{"holder JOIN consumes Prefix", "internal/successor/route/join/acquisition.go", modulePath + "/internal/successor/route/prefix", true},
		{"former operation cannot return", "cmd/ardents-next/route_linux.go", modulePath + "/internal/successor/route/operation", false},
		{"Prefix cannot import JOIN consumer", "internal/successor/route/prefix/borrowing.go", modulePath + "/internal/successor/route/join", false},
		{"Prefix cannot import Introduction consumer", "internal/successor/route/prefix/introduction_channel.go", modulePath + "/internal/successor/route/introduction", false},
		{"Prefix cannot import receiving composition", "internal/successor/route/prefix/prefix.go", modulePath + "/internal/successor/route/receiver", false},
		{"Prefix calls actual role owner", "internal/successor/route/prefix/prefix.go", modulePath + "/internal/successor/route/role", true},
		{"holder consumes actual Prefix", "cmd/ardents-next/route_linux.go", modulePath + "/internal/successor/route/prefix", true},
		{"channel cannot import receiving composition", "internal/successor/route/channel/session.go", modulePath + "/internal/successor/route/receiver", false},
		{"channel cannot import Network", "internal/successor/route/channel/session_test.go", modulePath + "/internal/successor/network", false},
		{"channel cannot import role authority", "internal/successor/route/channel/session.go", modulePath + "/internal/successor/route/role", false},
		{"role cannot import receiving composition", "internal/successor/route/role/authority.go", modulePath + "/internal/successor/route/receiver", false},
		{"role cannot spend Admission", "internal/successor/route/role/authority.go", modulePath + "/internal/successor/admission/receiving", false},
		{"role consumes Network observations", "internal/successor/route/role/authority.go", modulePath + "/internal/successor/network", true},
		{"Introduction exchange uses role binding", "internal/successor/route/introduction/receiving.go", modulePath + "/internal/successor/route/role", true},
		{"Introduction cannot own receiving Grant", "internal/successor/route/introduction/receiving.go", modulePath + "/internal/successor/admission/receiving", false},
		{"Introduction cannot call listener composition", "internal/successor/route/introduction/receiving.go", modulePath + "/internal/successor/route/receiver", false},
		{"channel cannot import Admission", "internal/successor/route/channel/session.go", modulePath + "/internal/successor/admission", false},
		{"channel cannot import Hosting", "internal/successor/route/channel/session.go", modulePath + "/internal/successor/hosting", false},
		{"transport cannot import framing", "internal/successor/route/transport/stream.go", modulePath + "/internal/successor/route/channel", false},
		{"transport cannot import concrete implementation", "internal/successor/route/transport/stream.go", modulePath + "/internal/successor/route/carrier", false},
		{"transport cannot import receiving composition", "internal/successor/route/transport/stream.go", modulePath + "/internal/successor/route/receiver", false},
		{"transport cannot import QUIC library", "internal/successor/route/transport/stream.go", "github.com/quic-go/quic-go", false},
		{"TLS implementation consumes shared authentication", "internal/successor/route/transport/tls/client.go", modulePath + "/internal/successor/route/transport", true},
		{"TLS cannot import QUIC implementation", "internal/successor/route/transport/tls/client.go", modulePath + "/internal/successor/route/transport/quic", false},
		{"TLS cannot import former carrier", "internal/successor/route/transport/tls/server.go", modulePath + "/internal/successor/route/carrier", false},
		{"transport cannot import its TLS adapter", "internal/successor/route/transport/role_authentication.go", modulePath + "/internal/successor/route/transport/tls", false},
		{"QUIC implementation consumes shared authentication", "internal/successor/route/transport/quic/role.go", modulePath + "/internal/successor/route/transport", true},
		{"former selector cannot import QUIC library", "internal/successor/route/carrier/closed_node_carrier.go", "github.com/quic-go/quic-go", false},
		{"QUIC owns its native library", "internal/successor/route/transport/quic/node.go", "github.com/quic-go/quic-go", true},
		{"QUIC cannot import TLS implementation", "internal/successor/route/transport/quic/role.go", modulePath + "/internal/successor/route/transport/tls", false},
		{"transport exporter cannot import QUIC adapter", "internal/successor/route/transport/role_exporter.go", modulePath + "/internal/successor/route/transport/quic", false},
		{"production command excludes lower contract", "cmd/ardents-next/route_linux.go", modulePath + "/internal/successor/route/transport", false},
		{"bootstrap scenario uses shared contract", "cmd/ardents-next/route_bootstrap_linux_test.go", modulePath + "/internal/successor/route/transport", true},
		{"installed Execution scenario uses shared Carrier identity", "cmd/ardents-next/execution_installed_linux_test.go", modulePath + "/internal/successor/route/transport", true},
		{"installed Execution scenario excludes direct TLS construction", "cmd/ardents-next/execution_installed_linux_test.go", modulePath + "/internal/successor/route/transport/tls", false},
		{"installed Execution scenario excludes direct QUIC construction", "cmd/ardents-next/execution_installed_linux_test.go", modulePath + "/internal/successor/route/transport/quic", false},
		{"bootstrap scenario excludes TLS construction", "cmd/ardents-next/route_bootstrap_linux_test.go", modulePath + "/internal/successor/route/transport/tls", false},
		{"bootstrap scenario excludes QUIC construction", "cmd/ardents-next/route_bootstrap_linux_test.go", modulePath + "/internal/successor/route/transport/quic", false},
		{"unregistered scenario excludes lower contract", "cmd/ardents-next/route_other_linux_test.go", modulePath + "/internal/successor/route/transport", false},
		{"production bootstrap excludes TLS construction", "cmd/ardents-next/route_bootstrap_linux.go", modulePath + "/internal/successor/route/transport/tls", false},
		{"former selector cannot return in caller test", "cmd/ardents-next/route_prefix_caller_linux_test.go", modulePath + "/internal/successor/route/carrier", false},
		{"no production Carrier allowance", "cmd/ardents-next/route_prefix_caller_linux.go", modulePath + "/internal/successor/route/carrier", false},
		{"caller test cannot reach old Carrier", "cmd/ardents-next/route_prefix_caller_linux_test.go", modulePath + "/internal/route/carrier", false},
		{"JOIN cannot retain former selector", "cmd/ardents-next/route_join_linux_test.go", modulePath + "/internal/successor/route/carrier", false},
		{"JOIN production excludes Carrier adapter", "cmd/ardents-next/route_join_linux.go", modulePath + "/internal/successor/route/carrier", false},
		{"JOIN test excludes old Carrier", "cmd/ardents-next/route_join_linux_test.go", modulePath + "/internal/route/carrier", false},
		{"external public-contract test", "internal/successor/admission/issuerprofile/provenance_test.go", modulePath + "/internal/successor/admission/issuerprofile", true},
		{"no production self import", "internal/successor/admission/issuerprofile/profile.go", modulePath + "/internal/successor/admission/issuerprofile", false},
		{"no unregistered test self import", "internal/successor/future/file_test.go", modulePath + "/internal/successor/future", false},
		{"public profile is leaf", "internal/successor/admission/issuerprofile/profile.go", modulePath + "/internal/successor/admission", false},
		{"profile cannot borrow identity", "internal/successor/admission/issuerprofile/profile.go", modulePath + "/internal/successor/nodeidentity", false},
		{"identity cannot borrow ledger", "internal/successor/nodeidentity/store.go", modulePath + "/internal/successor/admission", false},
		{"ledger public grammar", "internal/successor/admission/binding.go", modulePath + "/internal/successor/admission/issuerprofile", true},
		{"issuance public grammar", "internal/successor/admission/issuance/profile.go", modulePath + "/internal/successor/admission/issuerprofile", true},
		{"no inherited nested imports", "internal/successor/admission/issuance/nested/file.go", modulePath + "/internal/successor/admission", false},
		{"no inherited nested third party", "internal/successor/admission/issuance/nested/file.go", "github.com/cloudflare/circl/blindsign/blindrsa", false},
		{"unregistered caller", "internal/successor/future/file.go", modulePath + "/internal/successor/admission", false},
		{"unregistered dependency", "cmd/ardents-next/main.go", modulePath + "/internal/successor/future", false},
		{"standard", "internal/successor/admission/check.go", "context", true},
		{"zone", "cmd/ardents-next/main.go", modulePath + "/internal/successor/admission", true},
		{"hosting caller", "cmd/ardents-next/hosting.go", modulePath + "/internal/successor/hosting", true},
		{"independent domains", "internal/successor/hosting/budget.go", modulePath + "/internal/successor/admission", false},
		{"network input adapter", "internal/successor/network/state/membership.go", modulePath + "/internal/successor/network", true},
		{"network adapter is exact", "internal/successor/network/state/other.go", modulePath + "/internal/successor/network", false},
		{"application observation", "internal/successor/network/state/runtime_observation.go", modulePath + "/internal/successor/network", true},
		{"legacy consumer has no new State", "internal/endpoint/source_state.go", modulePath + "/internal/successor/network/state", false},
		{"State consumer has no domain access", "internal/endpoint/source_state.go", modulePath + "/internal/successor/network", false},
		{"State consumer has no physical root access", "internal/endpoint/source_state.go", modulePath + "/internal/successor/network/state/durable", false},
		{"unregistered State consumer", "internal/service/publication/publication.go", modulePath + "/internal/successor/network/state", false},
		{"State excludes debit", "internal/successor/network/state/open.go", modulePath + "/internal/successor/admission/quota", false},
		{"State excludes old backend", "internal/successor/network/state/open.go", modulePath + "/internal/network/state", false},
		{"legacy admission consumer isolated", "internal/node/authority/admission_observation.go", modulePath + "/internal/successor/admission/receiving", false},
		{"legacy token consumer isolated", "internal/node/authority/token.go", modulePath + "/internal/successor/admission/receiving", false},
		{"observation excludes spending root", "internal/node/authority/admission_observation.go", modulePath + "/internal/successor/admission/spending", false},
		{"observation bridge is exact", "internal/node/authority/receiver.go", modulePath + "/internal/successor/admission/receiving", false},
		{"acquisition policy adapter", "internal/successor/network/state/acquisition_history.go", modulePath + "/internal/successor/network", true},
		{"acquisition adapter is exact", "internal/successor/network/state/attempts.go", modulePath + "/internal/successor/network", false},
		{"participation policy adapter", "internal/successor/network/duty/participation_policy.go", modulePath + "/internal/successor/network", true},
		{"participation adapter is exact", "internal/successor/network/duty/store.go", modulePath + "/internal/successor/network", false},
		{"candidate policy adapter", "internal/successor/network/epoch/candidate_evaluation.go", modulePath + "/internal/successor/network", true},
		{"assignment adapter", "internal/successor/network/epoch/assignment.go", modulePath + "/internal/successor/network", true},
		{"candidate adapter is exact", "internal/successor/network/epoch/record.go", modulePath + "/internal/successor/network", false},
		{"public token grammar adapter", "internal/successor/network/closedprofile/token_spki.go", modulePath + "/internal/successor/admission/issuerprofile", true},
		{"public token adapter is exact", "internal/successor/network/closedprofile/profile.go", modulePath + "/internal/successor/admission/issuerprofile", false},
		{"public token adapter excludes signing", "internal/successor/network/closedprofile/token_spki.go", modulePath + "/internal/successor/admission/issuance", false},
		{"network excludes old state", "internal/successor/network/membership.go", modulePath + "/internal/network/state", false},
		{"network excludes execution", "internal/successor/network/membership.go", modulePath + "/internal/node", false},
		{"network excludes debit", "internal/successor/network/membership.go", modulePath + "/internal/successor/admission", false},
		{"network excludes reservations", "internal/successor/network/membership.go", modulePath + "/internal/successor/hosting", false},
		{"issuance confirmed debit", "internal/successor/admission/issuance/store.go", modulePath + "/internal/successor/admission", true},
		{"operation owners", "internal/successor/admission/issuer/operation.go", modulePath + "/internal/successor/admission/issuance", true},
		{"operation excludes hosting", "internal/successor/admission/issuer/operation.go", modulePath + "/internal/successor/hosting", false},
		{"issuance excludes hosting", "internal/successor/admission/issuance/store.go", modulePath + "/internal/successor/hosting", false},
		{"identity purpose", "internal/successor/nodeidentity/store.go", modulePath + "/internal/successor/admission/issuerprofile", true},
		{"identity cannot borrow issuance", "internal/successor/nodeidentity/store.go", modulePath + "/internal/successor/admission/issuance", false},
		{"admission cannot borrow identity", "internal/successor/admission/issuer_profile.go", modulePath + "/internal/successor/nodeidentity", false},
		{"admission excludes composition", "internal/successor/admission/ledger.go", modulePath + "/internal/successor/admission/issuer", false},
		{"admission cannot borrow keys", "internal/successor/admission/batch.go", modulePath + "/internal/successor/admission/issuance", false},
		{"admission cannot borrow budget", "internal/successor/admission/check.go", modulePath + "/internal/successor/hosting", false},
		{"legacy", "internal/successor/admission/check.go", modulePath + "/internal/admission", false},
		{"legacy test fixture", "internal/successor/admission/check_test.go", modulePath + "/tests/fixtures", false},
		{"third party bridge", "internal/successor/admission/check.go", "example.org/bridge", false},
		{"reverse", "internal/endpoint/runtime.go", modulePath + "/internal/successor/admission", false},
		{"prefix collision", "internal/successor/admission/check.go", modulePath + "/internal/successorold", false},
		{"unrelated", "internal/endpoint/runtime.go", "context", true},
		{"relative", "internal/successor/admission/check.go", "../admission", false},
		{"unsafe", "internal/successor/admission/check.go", "unsafe", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := successorImportAllowed(test.source, test.dependency); got != test.allowed {
				t.Fatalf("allowed = %v, want %v", got, test.allowed)
			}
		})
	}
}
