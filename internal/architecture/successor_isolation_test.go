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

func successorImportAllowed(source, dependency string) bool {
	inZone := strings.HasPrefix(source, "internal/successor/") || strings.HasPrefix(source, "cmd/ardents-next/")
	zoneDependency := dependency == modulePath+"/internal/successor" || strings.HasPrefix(dependency, modulePath+"/internal/successor/")
	if !inZone {
		return !zoneDependency
	}
	if zoneDependency {
		// These two independent domain owners have exact standard-library-only
		// import contracts. Directory grouping grants no cross-domain dependency.
		if strings.HasPrefix(source, "internal/successor/admission/") || strings.HasPrefix(source, "internal/successor/hosting/") {
			return false
		}
		return true
	}
	// OTel composition is confined to the command. Admission remains standard
	// library only; no arbitrary third-party bridge is permitted.
	if strings.HasPrefix(source, "cmd/ardents-next/") {
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
			for _, allowed := range []string{"go.opentelemetry.io/proto/otlp/collector/trace/v1", "go.opentelemetry.io/proto/otlp/collector/metrics/v1", "google.golang.org/protobuf/proto"} {
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
		{"standard", "internal/successor/admission/check.go", "context", true},
		{"zone", "cmd/ardents-next/main.go", modulePath + "/internal/successor/admission", true},
		{"hosting caller", "cmd/ardents-next/hosting.go", modulePath + "/internal/successor/hosting", true},
		{"independent domains", "internal/successor/hosting/budget.go", modulePath + "/internal/successor/admission", false},
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
