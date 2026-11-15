package introduction

import "testing"

// Detached clocks test only exit correspondence. They never attest native join
// or enter a successful live Stop; that proof belongs to the installed profile.
func TestIntroductionRetirementRequiresOriginalCleanExit(t *testing.T) {
	archive := startArchive{PID: 1024, StartedRealtime: 100, StartedMonotonic: 50}
	clean := introductionExecution{pid: 1024, startedRealtime: 100, startedMonotonic: 50, exitedRealtime: 120, exitedMonotonic: 70, code: 1}
	if !introductionCleanExit(clean, archive) {
		t.Fatal("matching clean exit refused")
	}
	for _, change := range []func(*introductionExecution){
		func(v *introductionExecution) { v.pid++ },
		func(v *introductionExecution) { v.startedRealtime++ },
		func(v *introductionExecution) { v.startedMonotonic++ },
		func(v *introductionExecution) { v.exitedRealtime = 99 },
		func(v *introductionExecution) { v.exitedMonotonic = 49 },
		func(v *introductionExecution) { v.code = 0 },
		func(v *introductionExecution) { v.code = 2 },
		func(v *introductionExecution) { v.status = 2 },
	} {
		other := clean
		change(&other)
		if introductionCleanExit(other, archive) {
			t.Fatal("foreign, signalled or unsuccessful exit accepted", other)
		}
	}
	if introductionCleanExit(introductionExecution{}, startArchive{}) {
		t.Fatal("empty native observation supplied a completed invocation")
	}
	if retirementNumber(1).retirement("/opt/node") != "" || retirementNumber(1).retirementJournal("/opt/node") != "" {
		t.Fatal("first-stop owner admitted a later invocation")
	}
}
