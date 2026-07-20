package recovery

import (
	"strings"

	"ardents/internal/diagnostics"
)

func RunStartupStep(
	diag *diagnostics.Recorder,
	kind, domain, resource string,
	recoverable bool,
	recoveryAction string,
	fail func(code, domain, summary, detail, impact, recovery string),
	fn func() error,
) bool {
	op := diag.BeginOperation(kind, domain, resource, recoverable, recoveryAction)
	if err := fn(); err != nil {
		diag.FailOperation(op.ID, err.Error())
		code := "node." + strings.ReplaceAll(kind, ".", "_") + ".failed"
		summary := "startup step failed"
		if kind == "node.startup.state_load" {
			code = "node.state.load_failed"
			summary = "state load failed"
		}
		fail(code, domain, summary, err.Error(), "node startup could not complete", "restart_required")
		return false
	}
	diag.CompleteOperation(op.ID, kind+" completed")
	return true
}
