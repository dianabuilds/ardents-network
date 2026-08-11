package carrier

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	smokeRoleSchema   = "carrier-lab-smoke-role/v1"
	smokeResultSchema = "carrier-lab-smoke-result/v1"
	smokeEvidenceCap  = 64 * 1024
)

type smokeRoleConfig struct {
	SchemaVersion string `json:"schema_version"`
	RunID         string `json:"run_id"`
	Role          string `json:"role"`
	ListenAddress string `json:"listen_address"`
	PeerRole      string `json:"peer_role"`
	PeerAddress   string `json:"peer_address"`
}

type smokeRoleResult struct {
	SchemaVersion string   `json:"schema_version"`
	RunID         string   `json:"run_id"`
	Role          string   `json:"role"`
	Status        string   `json:"status"`
	ObservedPeers []string `json:"observed_peers"`
	Failure       string   `json:"failure,omitempty"`
}

// RunRole executes one fixed two-peer isolation role from data-only
// configuration and writes bounded role-local evidence.
func RunRole(configPath, evidenceDir string) error {
	config, err := readSmokeRoleConfig(configPath)
	if err != nil {
		return err
	}
	if err := requireCanonicalDirectory(evidenceDir); err != nil {
		return fmt.Errorf("evidence directory: %w", err)
	}
	result := smokeRoleResult{SchemaVersion: smokeResultSchema, RunID: config.RunID, Role: config.Role, Status: "failed"}
	finish := func(runErr error) error {
		if runErr == nil {
			result.Status = "passed"
		} else {
			result.Failure = runErr.Error()
		}
		slices.Sort(result.ObservedPeers)
		if evidenceErr := writeBoundedJSON(filepath.Join(evidenceDir, "result.json"), result); evidenceErr != nil {
			return errors.Join(runErr, evidenceErr)
		}
		return runErr
	}

	listener, err := net.Listen("tcp", config.ListenAddress)
	if err != nil {
		return finish(fmt.Errorf("listen: %w", err))
	}
	defer listener.Close()
	if err := writeBoundedJSON(filepath.Join(evidenceDir, "ready.json"), map[string]string{
		"schema_version": smokeResultSchema,
		"run_id":         config.RunID,
		"role":           config.Role,
		"status":         "ready",
	}); err != nil {
		return finish(err)
	}

	type peerObservation struct {
		peer string
		err  error
	}
	inbound := make(chan peerObservation, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			inbound <- peerObservation{err: acceptErr}
			return
		}
		defer connection.Close()
		peer, exchangeErr := exchangeSmokeIdentity(connection, config.RunID, config.Role, config.PeerRole)
		inbound <- peerObservation{peer: peer, err: exchangeErr}
	}()

	outbound, err := dialSmokePeer(config.PeerAddress, 10*time.Second)
	if err != nil {
		return finish(err)
	}
	peer, outboundErr := exchangeSmokeIdentity(outbound, config.RunID, config.Role, config.PeerRole)
	_ = outbound.Close()
	if outboundErr != nil {
		return finish(outboundErr)
	}
	result.ObservedPeers = append(result.ObservedPeers, peer)
	select {
	case observation := <-inbound:
		if observation.err != nil {
			return finish(observation.err)
		}
		result.ObservedPeers = append(result.ObservedPeers, observation.peer)
	case <-time.After(10 * time.Second):
		return finish(errors.New("timed out waiting for the allowed peer"))
	}
	return finish(nil)
}

func readSmokeRoleConfig(path string) (smokeRoleConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return smokeRoleConfig{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var config smokeRoleConfig
	if err := decoder.Decode(&config); err != nil {
		return smokeRoleConfig{}, err
	}
	if config.SchemaVersion != smokeRoleSchema || !runIDPattern.MatchString(config.RunID) {
		return smokeRoleConfig{}, errors.New("invalid smoke role schema or run identity")
	}
	if !validSmokePair(config.Role, config.PeerRole) || config.ListenAddress == "" || config.PeerAddress == "" {
		return smokeRoleConfig{}, errors.New("smoke role configuration does not name the fixed allowed peer")
	}
	return config, nil
}

func validSmokePair(role, peer string) bool {
	return role == "alpha" && peer == "beta" || role == "beta" && peer == "alpha"
}

func dialSmokePeer(address string, timeout time.Duration) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
		if err == nil {
			return connection, nil
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	return nil, fmt.Errorf("connect to allowed peer: %w", lastErr)
}

func exchangeSmokeIdentity(connection net.Conn, runID, role, expectedPeer string) (string, error) {
	if err := connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return "", err
	}
	if _, err := fmt.Fprintf(connection, "smoke/%s/%s\n", runID, role); err != nil {
		return "", err
	}
	line, err := bufio.NewReaderSize(connection, 512).ReadString('\n')
	if err != nil {
		return "", err
	}
	want := "smoke/" + runID + "/" + expectedPeer
	if strings.TrimSpace(line) != want {
		return "", errors.New("received identity from an unexpected peer")
	}
	return expectedPeer, nil
}

func writeBoundedJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if len(data) > smokeEvidenceCap {
		return errors.New("role evidence exceeds its fixed cap")
	}
	return writeAtomic(path, data, 0o600)
}
