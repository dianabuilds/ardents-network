package console

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"time"
)

// Run owns the input/output descriptors through cancellation. Closing them on cancellation
// joins the blocking local I/O; no abandoned scanner goroutine is created.
func Run(ctx context.Context, in io.ReadCloser, out io.Writer, step func(context.Context, []byte) (any, bool, error)) int {
	if ctx == nil || in == nil {
		return 2
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	joined := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = in.Close()
		if closer, ok := out.(io.Closer); ok {
			_ = closer.Close()
		}
		close(joined)
	})
	defer func() {
		if !stop() {
			<-joined
		}
	}()
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), 64<<10)
	encode := json.NewEncoder(out)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return 130
		}
		value, done, err := step(ctx, scanner.Bytes())
		if err != nil && value == nil {
			value = map[string]string{"outcome": "refused"}
		}
		if encode.Encode(value) != nil {
			return 2
		}
		if done {
			if err != nil {
				return 1
			}
			return 0
		}
	}
	if ctx.Err() != nil {
		return 130
	}
	if scanner.Err() != nil {
		return 2
	}
	return 0
}
