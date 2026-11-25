//go:build !linux

package runtime

import (
	"context"
	"errors"
)

// The byte-stream launch entries refuse on platforms without the selected native
// worker. The portable ExchangeByteStream composition still compiles and joins a
// refused invocation, but no qualified byte-stream worker is admitted here. This
// closes the refusal-stub gap flagged by the Cohort 2b endpoint boundary review
// (PR #860); the genuine Linux composition lives in byte_stream_linux.go.
func (*Owner) LaunchByteStreamPublisher(context.Context, [32]byte) (*Invocation, error) {
	return nil, errors.New("ordinary byte-stream Publisher native launch is unavailable")
}

func (*Owner) LaunchByteStreamReader(context.Context, [32]byte) (*Invocation, error) {
	return nil, errors.New("ordinary byte-stream Reader native launch is unavailable")
}
