//go:build linux

package publisher

import (
	"github.com/dianabuilds/ardents-network/internal/application/document"
	executionruntime "github.com/dianabuilds/ardents-network/internal/execution/runtime"
)

// DocumentContract binds the fixed snapshot response to its original live operation.
func DocumentContract(operation *executionruntime.Operation) (executionruntime.StreamContract, error) {
	return operation.BindStreamContract(uint64(document.MaximumBytes+13), 512)
}
