package join

import (
	"bytes"
	"encoding/json"
)

// No Network identity/pins, semantic root, credentials, protection profile or
// lifecycle bound changes with a resource-only update. A version/formatting
// change without different program or actual work/termination is not an update.
func checkReplacementPlan(oldRaw, nextRaw []byte, programChanged bool) error {
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
