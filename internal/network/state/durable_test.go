package state

import "github.com/dianabuilds/ardents-network/internal/network/state/durable"

func openTestDurableRoot(path string) (*durable.Root, error) {
	return durable.Open(path, storageLimits())
}
