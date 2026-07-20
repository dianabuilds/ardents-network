package recovery

import (
	"ardents/internal/diagnostics"
)

func LoadDiagnosticsForStartup(
	diag *diagnostics.Recorder,
	fail func(code, domain, summary, detail, impact, recovery string),
) bool {
	if err := diag.Load(); err != nil {
		if ledgerErr, ok := diagnostics.IsCorruptLedger(err); ok {
			return handleCorruptDiagnosticsLedger(diag, fail, ledgerErr)
		}
		failDiagnosticsLedgerLoad(fail, err.Error())
		return false
	}
	retainLoadedDiagnosticsHealth(diag)
	return true
}

func handleCorruptDiagnosticsLedger(
	diag *diagnostics.Recorder,
	fail func(code, domain, summary, detail, impact, recovery string),
	ledgerErr *diagnostics.CorruptLedgerError,
) bool {
	diag.SetSubsystem("diagnostics", diagnostics.HealthDegraded, &diagnostics.Reason{
		Code:     "diagnostics.ledger.corrupt",
		Domain:   "diagnostics",
		Summary:  "diagnostics ledger needed recovery",
		Detail:   ledgerErr.Error(),
		Impact:   "pending operation history may be incomplete",
		Recovery: "automatic",
	})
	if ledgerErr.Fatal {
		failDiagnosticsLedgerLoad(fail, ledgerErr.Error())
		return false
	}
	return true
}

func failDiagnosticsLedgerLoad(
	fail func(code, domain, summary, detail, impact, recovery string),
	detail string,
) {
	fail(
		"diagnostics.ledger.load_failed",
		"diagnostics",
		"diagnostics ledger load failed",
		detail,
		"node recovery state is unavailable",
		"restart_required",
	)
}

func retainLoadedDiagnosticsHealth(diag *diagnostics.Recorder) {
	loaded := diag.Health()
	if loaded.PrimaryReason != nil || len(loaded.Subsystems) != 0 || loaded.State != diagnostics.HealthReady {
		diag.RetainCurrentHealth()
	}
}
