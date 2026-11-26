package issuance

import (
	"encoding/json"

	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
)

// InventoryPlan contains the local roots and binding for inventory delivery.
type InventoryPlan struct {
	Root          string
	Binding       Binding
	InventoryFile string
}

// DecodeInventoryPlan validates the exact inventory declaration before effects.
func DecodeInventoryPlan(raw []byte) (InventoryPlan, error) {
	var plan InventoryPlan
	fields, err := resultFields(raw, "root", "binding", "inventory_file")
	if err != nil {
		return plan, err
	}
	if json.Unmarshal(fields["root"], &plan.Root) != nil || !pathgrammar.Strict(plan.Root) ||
		json.Unmarshal(fields["inventory_file"], &plan.InventoryFile) != nil || !pathgrammar.Strict(plan.InventoryFile) ||
		!OutputOutsideRoots(plan.InventoryFile, plan.Root) {
		return plan, ErrInvalid
	}
	plan.Binding, err = DecodeKeyBinding(fields["binding"])
	return plan, err
}
