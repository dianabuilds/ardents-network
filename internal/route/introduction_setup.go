package route

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

const introductionSetupBodySize = 5 + 32*11 + 8 + 32

type introductionSetupResult struct {
	receipt      [32]byte
	proof        introductionSetup
	opaqueBytes  uint64
	opaqueDigest [32]byte
	err          error
}

func requestIntroductionSetup(ctx context.Context, input Actor) (introductionSetup, [32]byte, error) {
	var empty [32]byte
	raw, err := (&net.Dialer{Timeout: input.Deadline}).DialContext(ctx, "unix", input.IntroductionSetupSocket)
	if err != nil {
		return introductionSetup{}, empty, fmt.Errorf("dial Introduction setup path: %w", err)
	}
	defer raw.Close()
	if err := raw.SetDeadline(time.Now().Add(input.Deadline)); err != nil {
		return introductionSetup{}, empty, fmt.Errorf("bound Introduction setup path: %w", err)
	}
	outer := tls.Client(raw, clientTLS(input.ClientCertificate, input.IntroductionSetupPublic))
	if err := outer.HandshakeContext(ctx); err != nil {
		return introductionSetup{}, empty, fmt.Errorf("authenticate Introduction role: %w", err)
	}
	sealed := tls.Client(outer, clientTLS(tls.Certificate{}, input.IntroductionServicePublic))
	if err := sealed.HandshakeContext(ctx); err != nil {
		return introductionSetup{}, empty, fmt.Errorf("authenticate sealed setup service: %w", err)
	}
	body, err := introductionSetupBody(input)
	if err != nil {
		return introductionSetup{}, empty, err
	}
	if err := writeAll(sealed, body); err != nil {
		return introductionSetup{}, empty, fmt.Errorf("write sealed invitation: %w", err)
	}
	reply := make([]byte, 32)
	if _, err := io.ReadFull(sealed, reply); err != nil {
		return introductionSetup{}, empty, fmt.Errorf("read sealed invitation receipt: %w", err)
	}
	proof := decodeIntroductionSetup(body, reply)
	return proof, introductionSetupReceipt(body, reply), nil
}

func introductionSetupBody(input Actor) ([]byte, error) {
	body := make([]byte, introductionSetupBodySize)
	copy(body[:5], "ASIS\x02")
	fields := [][32]byte{input.ManifestDigest, input.Plan.NetworkID, input.Plan.Digest, input.Plan.ViewRoot,
		sha256.Sum256([]byte(input.Plan.Profile)), introductionCapabilities(), input.Plan.Positions[1].NodeID,
		input.Plan.Positions[2].NodeID, rendezvousReachability(input.Plan.Positions[2].Endpoint)}
	for index, field := range fields {
		copy(body[5+index*32:5+(index+1)*32], field[:])
	}
	if _, err := rand.Read(body[293:357]); err != nil {
		return nil, fmt.Errorf("draw sealed invitation handles: %w", err)
	}
	expires := time.Unix(input.Plan.SelectionAt, 0).Add(time.Hour)
	binary.BigEndian.PutUint64(body[357:365], uint64(expires.UnixNano()))
	transcript := introductionTranscript(body[:365])
	copy(body[365:397], transcript[:])
	return body, nil
}

func validIntroductionSetupBody(body []byte, input Actor, now time.Time) bool {
	if len(body) != introductionSetupBodySize || string(body[:5]) != "ASIS\x02" ||
		string(body[5:37]) != string(input.ManifestDigest[:]) || string(body[37:69]) != string(input.NetworkID[:]) ||
		string(body[69:101]) != string(input.EpochDigest[:]) || string(body[197:229]) != string(input.IntroductionSetupNode[:]) {
		return false
	}
	expires := int64(binary.BigEndian.Uint64(body[357:365]))
	transcript := introductionTranscript(body[:365])
	return nonzeroIntroductionField(body[101:133]) && nonzeroIntroductionField(body[133:165]) &&
		nonzeroIntroductionField(body[165:197]) && nonzeroIntroductionField(body[229:261]) &&
		nonzeroIntroductionField(body[261:293]) && nonzeroIntroductionField(body[293:325]) &&
		nonzeroIntroductionField(body[325:357]) && expires > now.UnixNano() && expires <= now.Add(90*time.Minute).UnixNano() &&
		string(body[365:397]) == string(transcript[:])
}

func nonzeroIntroductionField(value []byte) bool {
	for _, item := range value {
		if item != 0 {
			return true
		}
	}
	return false
}

func introductionCapabilities() [32]byte {
	return sha256.Sum256([]byte("ardents-h3-recovery-setup-capabilities-v1\x00tls13|single-use|no-application-data"))
}

func rendezvousReachability(endpoint string) [32]byte {
	return sha256.Sum256(append([]byte("ardents-h3-rendezvous-reachability-v1\x00"), endpoint...))
}

func introductionTranscript(body []byte) [32]byte {
	return sha256.Sum256(append([]byte("ardents-h3-sealed-introduction-transcript-v2\x00"), body...))
}

func introductionSetupReceipt(body, reply []byte) [32]byte {
	value := make([]byte, 0, 48+len(body)+len(reply))
	value = append(value, "ardents-h3-sealed-introduction-v2\x00"...)
	value = append(value, body...)
	value = append(value, reply...)
	return sha256.Sum256(value)
}

func decodeIntroductionSetup(body, reply []byte) introductionSetup {
	var result introductionSetup
	fields := []*[32]byte{&result.ManifestDigest, &result.NetworkID, &result.EpochDigest, &result.ViewRoot,
		&result.ProfileDigest, &result.CapabilitiesDigest, &result.IntroductionNode, &result.RendezvousNode,
		&result.RendezvousReachability, &result.JoinHandle, &result.EndpointHandshake}
	for index, field := range fields {
		copy(field[:], body[5+index*32:5+(index+1)*32])
	}
	result.ExpiresAtNanos = int64(binary.BigEndian.Uint64(body[357:365]))
	copy(result.TranscriptContext[:], body[365:397])
	copy(result.Reply[:], reply)
	return result
}

func cleanupUnixListener(listener net.Listener, path string) error {
	closeErr := listener.Close()
	if errors.Is(closeErr, net.ErrClosed) {
		closeErr = nil
	}
	removeErr := os.Remove(path)
	if errors.Is(removeErr, os.ErrNotExist) {
		removeErr = nil
	}
	return errors.Join(closeErr, removeErr)
}
