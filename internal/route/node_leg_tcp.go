package route

import (
	"context"
	"crypto/tls"
	"net"
)

func openTCPNodeCarrier(ctx context.Context, input NodeLegRequest) (nodeCarrierResult, error) {
	raw, err := (&net.Dialer{}).DialContext(ctx, "tcp", input.Endpoint)
	if err != nil {
		return nodeCarrierResult{}, err
	}
	secured := tls.Client(raw, nativeNodeTLS(input.Certificate, input.ExpectedPeerKey))
	if err := secured.SetDeadline(input.Deadline); err != nil {
		_ = raw.Close()
		return nodeCarrierResult{}, err
	}
	if err := secured.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		return nodeCarrierResult{}, err
	}
	return nodeCarrierResult{lane: secured, state: secured.ConnectionState(), abort: raw.Close}, nil
}

func nativeNodeTLS(certificate tls.Certificate, expected [32]byte) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		InsecureSkipVerify: true, SessionTicketsDisabled: true, NextProtos: []string{Profile}, VerifyConnection: exactPeer(expected)}
}
