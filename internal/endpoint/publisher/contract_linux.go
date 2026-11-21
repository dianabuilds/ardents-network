//go:build linux

package publisher

import (
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	executionruntime "github.com/dianabuilds/ardents-network/internal/execution/runtime"
)

// TextContract binds the fixed snapshot response to its original live operation.
func TextContract(operation *executionruntime.Operation) (executionruntime.StreamContract, error) {
	return operation.BindStreamContract(uint64(textdocument.MaximumBytes+13), 512)
}
