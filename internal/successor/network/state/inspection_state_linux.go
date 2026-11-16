package state

import (
	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state/durable"
)

func readRetainedInspection(cfg config, storage *durable.Inspection) (*epoch.Decision, distributionState, error) {
	return readRetainedHistory(cfg, storage)
}
