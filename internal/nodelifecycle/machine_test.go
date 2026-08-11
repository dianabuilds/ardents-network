package nodelifecycle

import "testing"

func TestStateMachineAcceptsOnlyDeclaredTransitions(t *testing.T) {
	t.Parallel()
	allowed := map[[2]lifecycleState]bool{
		{stateAbsent, statePrepared}: true, {stateAbsent, stateFailed}: true,
		{statePrepared, stateReady}: true, {statePrepared, stateFailed}: true,
		{stateReady, stateDraining}: true, {stateReady, stateFailed}: true,
		{stateDraining, stateWithdrawn}: true, {stateDraining, stateFailed}: true,
	}
	for from := stateAbsent; from <= stateFailed; from++ {
		for to := stateAbsent; to <= stateFailed; to++ {
			machine := stateMachine{current: from}
			err := machine.move(to)
			want := from == to || allowed[[2]lifecycleState{from, to}]
			if (err == nil) != want {
				t.Fatalf("transition %s -> %s error = %v", stateNames[from], stateNames[to], err)
			}
		}
	}
}
