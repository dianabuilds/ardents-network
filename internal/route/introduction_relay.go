package route

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

func startIntroductionRelay(ctx context.Context, input Actor) (func() error, <-chan introductionSetupResult, error) {
	listener, err := net.Listen("unix", input.IntroductionSetupSocket)
	if err != nil {
		return nil, nil, fmt.Errorf("listen for sealed Introduction input: %w", err)
	}
	if err := os.Chmod(input.IntroductionSetupSocket, 0o600); err != nil {
		return nil, nil, fmt.Errorf("protect sealed Introduction input: %w", errors.Join(err,
			listener.Close(), os.Remove(input.IntroductionSetupSocket)))
	}
	completed := make(chan introductionSetupResult, 1)
	go serveIntroductionRelay(ctx, listener, input, completed)
	go func() { <-ctx.Done(); _ = listener.Close() }()
	return func() error { return cleanupUnixListener(listener, input.IntroductionSetupSocket) }, completed, nil
}

func serveIntroductionRelay(ctx context.Context, listener net.Listener, input Actor,
	completed chan<- introductionSetupResult) {
	raw, err := listener.Accept()
	if err != nil {
		completed <- introductionSetupResult{err: fmt.Errorf("accept sealed Introduction input: %w", contextError(ctx, err))}
		return
	}
	defer raw.Close()
	stop := context.AfterFunc(ctx, func() { _ = raw.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		if err := raw.SetDeadline(deadline); err != nil {
			completed <- introductionSetupResult{err: fmt.Errorf("bound sealed Introduction input: %w", err)}
			return
		}
	}
	upstream := tls.Server(raw, serverTLS(input.Certificate, input.IntroductionSetupPeer))
	if err := upstream.HandshakeContext(ctx); err != nil {
		completed <- introductionSetupResult{err: fmt.Errorf("authenticate sealed Introduction input: %w", err)}
		return
	}
	downstreamRaw, err := (&net.Dialer{Timeout: input.Deadline}).DialContext(ctx, "unix", input.IntroductionForwardSocket)
	if err != nil {
		completed <- introductionSetupResult{err: fmt.Errorf("dial sealed Introduction service path: %w", err)}
		return
	}
	defer downstreamRaw.Close()
	if err := downstreamRaw.SetDeadline(time.Now().Add(input.Deadline)); err != nil {
		completed <- introductionSetupResult{err: fmt.Errorf("bound sealed Introduction service path: %w", err)}
		return
	}
	downstream := tls.Client(downstreamRaw, clientTLS(input.Certificate, input.IntroductionForwardPublic))
	if err := downstream.HandshakeContext(ctx); err != nil {
		completed <- introductionSetupResult{err: fmt.Errorf("authenticate sealed Introduction service path: %w", err)}
		return
	}
	forward, reverse, relayErr := relayOpaque(upstream, downstream)
	if relayErr != nil && !benignStreamError(relayErr) {
		completed <- introductionSetupResult{err: fmt.Errorf("relay opaque sealed invitation: %w", relayErr)}
		return
	}
	combined := sha256.Sum256(append(forward.digest[:], reverse.digest[:]...))
	completed <- introductionSetupResult{opaqueBytes: forward.count + reverse.count, opaqueDigest: combined}
}
