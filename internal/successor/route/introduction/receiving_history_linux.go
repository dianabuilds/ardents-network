//go:build linux

package introduction

import (
	"errors"
	"os"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
)

// OpenReceivingRegistry prepares the original Route capacity/history owner
// before listener effects. A retained root is strictly reopened; only a vacant
// root can consume the genuine Receiving owner's opaque one-use birth fact.
// Composition owns its one Close after Receiver and original users join.
func OpenReceivingRegistry(root string, binding Binding, receivingOwner *receiving.Owner) (*Registry, error) {
	if receivingOwner == nil {
		return nil, errors.New("introduction receiving owner absent")
	}
	names, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var history *History
	if len(names) == 0 {
		fact, err := receivingOwner.TakeFreshRoot()
		if err != nil {
			return nil, err
		}
		history, err = InitializeHistory(root, binding, fact)
		if err != nil {
			return nil, err
		}
	} else {
		history, err = OpenHistory(root, binding)
		if err != nil {
			return nil, err
		}
	}
	registry, err := NewRegistry(history)
	if err != nil {
		return nil, errors.Join(err, history.Close())
	}
	if err := registry.Check(binding); err != nil {
		return nil, errors.Join(err, registry.Close())
	}
	return registry, nil
}
