// The custody terminal reader is implemented for Linux and Windows only
// (terminal_line_linux.go/terminal_line_windows.go); macOS refuses before the
// prompt. This control also drives a Unix pseudo-terminal (pty.UnixPty.Slave),
// which the Windows console pty does not satisfy, so the real prompt-failure
// path is exercised on Linux only.
package command

import (
	"context"
	"errors"
	"testing"
	"time"

	pty "github.com/aymanbagabas/go-pty"
)

type failingPromptWriter struct {
	err error
}

func (writer failingPromptWriter) Write([]byte) (int, error) {
	return 0, writer.err
}

func TestTerminalInputStopsWhenPromptFails(t *testing.T) {
	terminal, err := pty.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := terminal.Close(); err != nil {
			t.Error(err)
		}
	})
	input := terminalSecretInput{
		terminal: terminal.(pty.UnixPty).Slave(),
		prompts:  failingPromptWriter{err: errors.New("prompt unavailable")},
	}
	cases := []struct {
		name string
		run  func(context.Context) error
	}{
		{"password", func(ctx context.Context) error {
			_, err := input.ReadSecret(ctx, "vault-unlock")
			return err
		}},
		{"confirmation", func(ctx context.Context) error {
			_, err := input.Confirm(ctx, "vault-purge")
			return err
		}},
		{"service commitment", func(ctx context.Context) error {
			_, err := input.ReadServiceRequestCommitment(ctx)
			return err
		}},
		{"admission commitment", func(ctx context.Context) error {
			_, err := input.ReadAdmissionRequestCommitment(ctx)
			return err
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			if err := test.run(ctx); !errors.Is(err, input.prompts.(failingPromptWriter).err) {
				t.Fatalf("prompt failure was not retained: %v", err)
			}
		})
	}
}
