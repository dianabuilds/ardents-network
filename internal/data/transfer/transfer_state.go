package transfer

import (
	"errors"
	"fmt"
)

func completeTransfer(data DataExchange, id, peer string, totalBytes int64, reason string) error {
	if _, err := data.CompleteTransfer(id, peer, totalBytes, reason); err != nil {
		return fmt.Errorf("record completed transfer %s: %w", id, err)
	}
	return nil
}

func failTransfer(data DataExchange, id, peer string, cause error) error {
	if cause == nil {
		cause = fmt.Errorf("transfer failed without a cause")
	}
	if _, err := data.FailTransfer(id, peer, cause.Error()); err != nil {
		return errors.Join(cause, fmt.Errorf("record failed transfer %s: %w", id, err))
	}
	return cause
}

func updateTransferProgress(data DataExchange, id string, progressBytes, totalBytes int64, reason string) error {
	if _, err := data.UpdateTransferProgress(id, progressBytes, totalBytes, reason); err != nil {
		return fmt.Errorf("record transfer progress %s: %w", id, err)
	}
	return nil
}
