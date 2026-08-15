//go:build ignore

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

type runProvenance struct {
	Candidate     string `json:"candidate"`
	Seed          string `json:"seed"`
	ClientSHA256  string `json:"client_sha256"`
	ServerSHA256  string `json:"server_sha256"`
	HarnessSHA256 string `json:"harness_sha256"`
	Image         string `json:"image"`
	ShutdownRung  string `json:"shutdown_rung"`
	Observer      string `json:"observer"`
}

func verifyProvenance(candidate string, provenance runProvenance) error {
	if provenance.Candidate != candidate {
		return errors.New("candidate provenance mismatch")
	}
	clientPath, serverPath := "/candidate/lyrebird", "/candidate/lyrebird"
	if candidate == "webtunnel" {
		clientPath, serverPath = "/candidate/webtunnel-client", "/candidate/webtunnel-server"
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	checks := []struct{ path, expected string }{
		{clientPath, provenance.ClientSHA256},
		{serverPath, provenance.ServerSHA256},
		{executable, provenance.HarnessSHA256},
	}
	for _, check := range checks {
		actual, err := fileSHA256(check.path)
		if err != nil {
			return err
		}
		if actual != check.expected || len(actual) != 64 {
			return fmt.Errorf("binary hash mismatch for %s", check.path)
		}
	}
	if provenance.Image == "" {
		return errors.New("runtime image identity is required")
	}
	if provenance.ShutdownRung != "stdin" && provenance.ShutdownRung != "sigterm" && provenance.ShutdownRung != "sigkill" {
		return errors.New("shutdown rung must be stdin, sigterm, or sigkill")
	}
	if provenance.Observer != "af-packet-port53-v1" {
		return errors.New("observer identity mismatch")
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
