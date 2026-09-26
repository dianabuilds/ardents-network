package architecture

import (
	"strings"
	"testing"
)

// ADR-0099 retired the superseded portable.Run foreground pump — the
// accepted command composition drives Open/Wait/FailureEvent directly —
// and moved the replacement crash-boundary seam into its test file, where
// its only consumer lives.
func TestSupersededPortableRunPumpIsAbsent(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	runtime := string(readProjectFile(t, root, "internal/endpoint/portable/runtime.go"))
	for _, forbidden := range []string{"func Run(", "func emit("} {
		if strings.Contains(runtime, forbidden) {
			t.Errorf("portable runtime.go still contains superseded pump member %q", forbidden)
		}
	}
	for _, retained := range []string{"func Open(", "func (runtime *Runtime) Wait(", "func FailureEvent("} {
		if !strings.Contains(runtime, retained) {
			t.Errorf("portable runtime.go lost retained declaration %q", retained)
		}
	}
	command := string(readProjectFile(t, root, "cmd/ardents/endpoint.go"))
	if !strings.Contains(command, "portable.Open(") || !strings.Contains(command, "running.Wait(ctx)") {
		t.Error("command composition lost the live portable Open/Wait wiring")
	}
}

func TestReplacementCrashSeamLivesWithItsTestConsumer(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	operation := string(readProjectFile(t, root, "internal/endpoint/replacement/operation.go"))
	if strings.Contains(operation, "replaceWithInterruption") {
		t.Error("operation.go still carries the test-only crash seam wrapper")
	}
	for _, retained := range []string{"func Replace(ctx context.Context, operation Operation) (Result, error)", "func Rollback(", "func Recover(", "func interrupted(control *operationControl, checkpoint string) bool"} {
		if !strings.Contains(operation, retained) {
			t.Errorf("operation.go lost retained declaration %q", retained)
		}
	}
	test := string(readProjectFile(t, root, "internal/endpoint/replacement/operation_test.go"))
	if !strings.Contains(test, "func replaceWithInterruption(") ||
		!strings.Contains(test, "func TestReplaceInterruptionLeavesOnlyExplicitRecoveryPaths(") {
		t.Error("operation_test.go lost the crash-boundary seam or its Recover oracle")
	}
}
