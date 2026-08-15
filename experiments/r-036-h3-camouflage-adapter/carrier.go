//go:build ignore

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

func exerciseCarrier(carrier net.Conn, work workload) (string, string, error) {
	payload := append(append([]byte{}, work.clientCanary...), work.request...)
	_ = carrier.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := carrier.Write(payload); err != nil {
		return "", "", err
	}
	response := make([]byte, len(work.serverCanary)+len(work.response))
	if _, err := io.ReadFull(carrier, response); err != nil {
		return "", "", err
	}
	expected := append(append([]byte{}, work.serverCanary...), work.response...)
	if !equalBytes(response, expected) {
		return "", "", errors.New("useful-work response mismatch")
	}
	requestDigest := sha256.Sum256(work.request)
	responseDigest := sha256.Sum256(work.response)
	return hex.EncodeToString(requestDigest[:]), hex.EncodeToString(responseDigest[:]), nil
}

func validateObservation(obs processObservation) error {
	if obs.Capabilities != "0000000000000000" {
		return fmt.Errorf("effective capabilities %s", obs.Capabilities)
	}
	if obs.Descendants != 0 {
		return fmt.Errorf("%d descendants", obs.Descendants)
	}
	return nil
}
