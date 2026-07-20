package transfer

import (
	"errors"
	"testing"

	appdata "ardents/internal/data"

	"github.com/stretchr/testify/require"
)

type failingTransferState struct {
	DataExchange
	completeErr error
	failErr     error
}

func (s failingTransferState) CompleteTransfer(string, string, int64, string) (appdata.TransferRecord, error) {
	return appdata.TransferRecord{}, s.completeErr
}

func (s failingTransferState) FailTransfer(string, string, string) (appdata.TransferRecord, error) {
	return appdata.TransferRecord{}, s.failErr
}

func TestCompleteTransferSurfacesPersistenceFailure(t *testing.T) {
	err := completeTransfer(failingTransferState{completeErr: errors.New("disk unavailable")}, "xfer-1", "peer-1", 42, "done")
	require.ErrorContains(t, err, "record completed transfer xfer-1")
	require.ErrorContains(t, err, "disk unavailable")
}

func TestFailTransferPreservesCauseAndPersistenceFailure(t *testing.T) {
	cause := errors.New("remote response rejected")
	err := failTransfer(failingTransferState{failErr: errors.New("disk unavailable")}, "xfer-2", "peer-2", cause)
	require.ErrorIs(t, err, cause)
	require.ErrorContains(t, err, "record failed transfer xfer-2")
	require.ErrorContains(t, err, "disk unavailable")
}
