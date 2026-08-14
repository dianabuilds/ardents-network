package route

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

func startIntroductionService(ctx context.Context, input Actor) (func() error, <-chan introductionSetupResult, error) {
	listener, err := net.Listen("unix", input.IntroductionSetupSocket)
	if err != nil {
		return nil, nil, fmt.Errorf("listen for sealed setup service: %w", err)
	}
	if err := os.Chmod(input.IntroductionSetupSocket, 0o600); err != nil {
		return nil, nil, errors.Join(err, listener.Close(), os.Remove(input.IntroductionSetupSocket))
	}
	completed := make(chan introductionSetupResult, 1)
	go serveIntroductionService(ctx, listener, input, completed)
	go func() { <-ctx.Done(); _ = listener.Close() }()
	return func() error { return cleanupUnixListener(listener, input.IntroductionSetupSocket) }, completed, nil
}

func serveIntroductionService(ctx context.Context, listener net.Listener, input Actor,
	completed chan<- introductionSetupResult) {
	raw, err := listener.Accept()
	if err != nil {
		completed <- introductionSetupResult{err: fmt.Errorf("accept sealed setup service path: %w", contextError(ctx, err))}
		return
	}
	defer raw.Close()
	stop := context.AfterFunc(ctx, func() { _ = raw.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		if err := raw.SetDeadline(deadline); err != nil {
			completed <- introductionSetupResult{err: fmt.Errorf("bound sealed setup service path: %w", err)}
			return
		}
	}
	outer := tls.Server(raw, serverTLS(input.Certificate, input.IntroductionSetupPeer))
	if err := outer.HandshakeContext(ctx); err != nil {
		completed <- introductionSetupResult{err: fmt.Errorf("authenticate Introduction relay: %w", err)}
		return
	}
	sealed := tls.Server(outer, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{input.ServiceCertificate}, SessionTicketsDisabled: true})
	if err := sealed.HandshakeContext(ctx); err != nil {
		completed <- introductionSetupResult{err: fmt.Errorf("accept opaque sealed invitation: %w", err)}
		return
	}
	body := make([]byte, introductionSetupBodySize)
	if _, err := io.ReadFull(sealed, body); err != nil {
		completed <- introductionSetupResult{err: fmt.Errorf("read sealed invitation: %w", err)}
		return
	}
	if !validIntroductionSetupBody(body, input, time.Now()) {
		completed <- introductionSetupResult{err: errors.New("sealed invitation binding is invalid")}
		return
	}
	reply := make([]byte, 32)
	if _, err := rand.Read(reply); err != nil {
		completed <- introductionSetupResult{err: fmt.Errorf("draw sealed invitation receipt: %w", err)}
		return
	}
	if err := writeAll(sealed, reply); err != nil {
		completed <- introductionSetupResult{err: fmt.Errorf("write sealed invitation receipt: %w", err)}
		return
	}
	completed <- introductionSetupResult{receipt: introductionSetupReceipt(body, reply),
		proof: decodeIntroductionSetup(body, reply)}
}
