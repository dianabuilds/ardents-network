package channel

import "fmt"

// physicalWriteFailure is created only after a session selects and starts its
// original physical frame. Queue refusal and ordinary EOF cannot mint it.
type physicalWriteFailure struct {
	owner *Session
	kind  uint8
	cause error
}

func (e *physicalWriteFailure) Error() string {
	return fmt.Sprintf("route frame %d physical write: %v", e.kind, e.cause)
}
func (e *physicalWriteFailure) Unwrap() error { return e.cause }

// physicalCloseFailure is minted only by the exact session's owned Conn.Close.
// An equal native error elsewhere is not evidence of this physical operation.
type physicalCloseFailure struct {
	owner *Session
	cause error
}

func (e *physicalCloseFailure) Error() string {
	return fmt.Sprintf("route physical close: %v", e.cause)
}
func (e *physicalCloseFailure) Unwrap() error { return e.cause }
