package forwarding

import (
	"bytes"
	"encoding/json"
)

func checkForwardPlan(oldRaw, nextRaw []byte, programChanged bool) error {
	previous, err := DecodePlan(oldRaw)
	if err != nil {
		return err
	}
	next, err := DecodePlan(nextRaw)
	if err != nil {
		return err
	}
	resourcesChanged := previous.Work != next.Work || previous.Termination != next.Termination
	next.Work, next.Termination = previous.Work, previous.Termination
	old, _ := json.Marshal(previous)
	new, _ := json.Marshal(next)
	if !bytes.Equal(old, new) || !programChanged && !resourcesChanged {
		return ErrBinding
	}
	return nil
}
