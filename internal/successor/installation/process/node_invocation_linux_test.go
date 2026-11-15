package process

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestNodeProcessCopiesRetainFailureAndCloseAdmission(t *testing.T) {
	for _, role := range []string{"join", "descriptor", "introduction", "forwarding"} {
		t.Run(role, func(t *testing.T) {
			cause := errors.New("original observation failed")
			original := &nodeInvocation{pin: &Invocation{}, failure: cause}
			var observe func(context.Context) error
			var closeOriginal, closeCopy func() error
			switch role {
			case "join":
				p := JoinInvocation{original: original}
				copy := p
				observe, closeOriginal, closeCopy = p.Observe, p.Close, copy.Close
			case "descriptor":
				p := DescriptorInvocation{original: original}
				copy := p
				observe, closeOriginal, closeCopy = p.Observe, p.Close, copy.Close
			case "introduction":
				p := IntroductionInvocation{original: original}
				copy := p
				observe, closeOriginal, closeCopy = p.Observe, p.Close, copy.Close
			case "forwarding":
				p := ForwardingInvocation{original: original}
				copy := p
				observe, closeOriginal, closeCopy = p.Observe, p.Close, copy.Close
			}
			if err := observe(context.Background()); !errors.Is(err, cause) || !errors.Is(err, os.ErrClosed) {
				t.Fatalf("failed original admitted observation: %v", err)
			}
			first := closeOriginal()
			if !errors.Is(first, cause) || closeCopy() != first {
				t.Fatalf("copies lost original terminal failure: %v", first)
			}
			if err := observe(context.Background()); !errors.Is(err, cause) || !errors.Is(err, os.ErrClosed) {
				t.Fatalf("closed original revived: %v", err)
			}
		})
	}
}
