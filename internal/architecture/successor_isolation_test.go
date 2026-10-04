package architecture

import (
	"go/parser"
	"go/token"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"
)

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
		"cmd/ardents-next":                           {"network", "network/state", "admission/stock", "admission/receiving", "admission/allocation", "admission", "admission/quota", "admission/issuerprofile", "admission/issuance", "admission/issuer", "nodeidentity", "hosting"},
	}
	if zoneDependency {
		if source == "cmd/ardents-next/network_admission_fixture_linux_test.go" && dependency == modulePath+"/internal/successor/network/epoch" {
			return true
		}
		if owner == "cmd/ardents-next" && strings.HasSuffix(source, "_test.go") && (dependency == modulePath+"/internal/successor/admission/token" || dependency == modulePath+"/internal/successor/admission/spending") {
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
	if (owner == "internal/successor/admission/issuance" || owner == "internal/successor/admission/token") && dependency == "github.com/cloudflare/circl/blindsign/blindrsa" {
		return true
	}
	if (owner == "internal/successor/admission/spending" || owner == "internal/successor/network/duty" || owner == "internal/successor/network/state/durable") && dependency == "golang.org/x/sys/windows" {
		return true
	}
	if strings.HasSuffix(source, "_test.go") {
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
