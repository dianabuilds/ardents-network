//go:build linux

package launch

import (
	"bytes"
	"context"
	"testing"
)

func TestNativePlatformRefusesCanceledContextBeforeEffects(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var output bytes.Buffer
	if code := RunNative(ctx, []string{"stage-endpoint", "/absent"}, &output); code != 2 || output.Len() != 0 {
		t.Fatalf("canceled launch: code %d, output %q", code, output.String())
	}
}
