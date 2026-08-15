//go:build ignore

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

type campaignReport struct {
	Candidate              string             `json:"candidate"`
	Verdict                string             `json:"verdict"`
	StartedAt              string             `json:"started_at"`
	StartupMilliseconds    int64              `json:"startup_milliseconds"`
	UsefulWorkMilliseconds int64              `json:"useful_work_milliseconds"`
	RequestSHA256          string             `json:"request_sha256"`
	ResponseSHA256         string             `json:"response_sha256"`
	DNSQueries             int64              `json:"dns_queries"`
	Client                 processObservation `json:"client"`
	Server                 processObservation `json:"server"`
	ClientControlSHA256    string             `json:"client_control_sha256"`
	ServerControlSHA256    string             `json:"server_control_sha256"`
	ClientShutdownRung     string             `json:"client_shutdown_rung"`
	ServerShutdownRung     string             `json:"server_shutdown_rung"`
	CleanupMilliseconds    int64              `json:"cleanup_milliseconds"`
	StateEntries           int                `json:"state_entries"`
	StateBytes             int64              `json:"state_bytes"`
	UsefulWorkVerified     bool               `json:"useful_work_verified"`
	RunManifestSHA256      string             `json:"run_manifest_sha256"`
}

func run(candidate, evidence string, provenance runProvenance) (err error) {
	if candidate != "obfs4" && candidate != "webtunnel" {
		return errors.New("candidate must be obfs4 or webtunnel")
	}
	if evidence == "" || !filepath.IsAbs(evidence) {
		return errors.New("evidence path must be absolute")
	}
	workload, err := makeWorkload(provenance.Seed)
	if err != nil {
		return err
	}
	if err := verifyProvenance(candidate, provenance); err != nil {
		return err
	}
	if err := os.MkdirAll(evidence, 0700); err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "r036-"+candidate+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	trap, err := startDNSTrap()
	if err != nil {
		return fmt.Errorf("DNS trap: %w", err)
	}
	defer trap.close()
	echo, echoAddress, err := startEcho(workload)
	if err != nil {
		return err
	}
	defer echo.close()
	report := campaignReport{Candidate: candidate, Verdict: "fail", StartedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	started := time.Now()
	server, client, carrier, front, err := openCampaign(candidate, work, echoAddress)
	if err != nil {
		return err
	}
	defer func() {
		if carrier != nil {
			_ = carrier.Close()
		}
	}()
	if front != nil {
		defer front.close()
	}
	report.StartupMilliseconds = time.Since(started).Milliseconds()
	report.Client, err = observeProcess(client.cmd.Process.Pid)
	if err != nil {
		return err
	}
	report.Server, err = observeProcess(server.cmd.Process.Pid)
	if err != nil {
		return err
	}
	if err := validateObservation(report.Client); err != nil {
		return fmt.Errorf("client process: %w", err)
	}
	if err := validateObservation(report.Server); err != nil {
		return fmt.Errorf("server process: %w", err)
	}
	workStarted := time.Now()
	report.RequestSHA256, report.ResponseSHA256, err = exerciseCarrier(carrier, workload)
	if err != nil {
		return err
	}
	report.UsefulWorkMilliseconds = time.Since(workStarted).Milliseconds()
	report.UsefulWorkVerified = true
	_ = carrier.Close()
	cleanupStarted := time.Now()
	report.ClientShutdownRung, _, err = client.stop()
	if err != nil {
		return err
	}
	report.ServerShutdownRung, _, err = server.stop()
	if err != nil {
		return err
	}
	report.CleanupMilliseconds = time.Since(cleanupStarted).Milliseconds()
	report.ClientControlSHA256 = client.transcriptHash()
	report.ServerControlSHA256 = server.transcriptHash()
	report.StateEntries, report.StateBytes, err = scanState(work)
	if err != nil {
		return err
	}
	report.DNSQueries = trap.close()
	if report.DNSQueries != 0 {
		return fmt.Errorf("observed %d DNS queries", report.DNSQueries)
	}
	if report.CleanupMilliseconds > 6000 {
		return errors.New("cleanup exceeded 6 seconds")
	}
	report.Verdict = "incomplete"
	return writeReport(evidence, report, client, server, provenance)
}

func openCampaign(candidate, work, echoAddress string) (*child, *child, net.Conn, *tlsFront, error) {
	method := candidate
	serverPath, clientPath := "/candidate/lyrebird", "/candidate/lyrebird"
	if candidate == "webtunnel" {
		serverPath, clientPath = "/candidate/webtunnel-server", "/candidate/webtunnel-client"
	}
	serverBind, err := reserveAddress()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	serverState := filepath.Join(work, "server")
	clientState := filepath.Join(work, "client")
	if err := os.MkdirAll(serverState, 0700); err != nil {
		return nil, nil, nil, nil, err
	}
	serverEnv := map[string]string{
		"TOR_PT_MANAGED_TRANSPORT_VER": "1", "TOR_PT_SERVER_TRANSPORTS": method,
		"TOR_PT_SERVER_BINDADDR": method + "-" + serverBind, "TOR_PT_ORPORT": echoAddress,
		"TOR_PT_STATE_LOCATION": serverState, "TOR_PT_EXIT_ON_STDIN_CLOSE": "1",
	}
	if candidate == "webtunnel" {
		serverEnv["TOR_PT_SERVER_TRANSPORT_OPTIONS"] = "webtunnel:url=https://bridge.invalid" + webTunnelPath
	}
	server, serverReady, err := startChild(serverPath, serverEnv, method, "SMETHOD")
	if err != nil {
		return server, nil, nil, nil, err
	}
	target := serverBind
	args := map[string]string{}
	var front *tlsFront
	if candidate == "obfs4" {
		args["cert"] = serverReady.args["cert"]
		args["iat-mode"] = "0"
		if args["cert"] == "" {
			_ = server.forceStop()
			return server, nil, nil, nil, errors.New("obfs4 server omitted cert")
		}
	} else {
		front, target, args["cert"], err = startTLSFront("192.0.2.3", serverBind)
		if err != nil {
			_ = server.forceStop()
			return server, nil, nil, nil, err
		}
		if err := front.checkOrdinary(target); err != nil {
			front.close()
			_ = server.forceStop()
			return server, nil, nil, nil, err
		}
		args["url"] = "https://" + target + webTunnelPath
		args["servername"] = "bridge.invalid"
	}
	if err := os.MkdirAll(clientState, 0700); err != nil {
		return server, nil, nil, front, err
	}
	clientEnv := map[string]string{
		"TOR_PT_MANAGED_TRANSPORT_VER": "1", "TOR_PT_CLIENT_TRANSPORTS": method,
		"TOR_PT_STATE_LOCATION": clientState, "TOR_PT_EXIT_ON_STDIN_CLOSE": "1",
	}
	client, clientReady, err := startChild(clientPath, clientEnv, method, "CMETHOD")
	if err != nil {
		_ = server.forceStop()
		return server, client, nil, front, err
	}
	carrier, err := openSOCKS(clientReady.address, target, args)
	if err != nil {
		_ = client.forceStop()
		_ = server.forceStop()
		return server, client, nil, front, err
	}
	return server, client, carrier, front, nil
}

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
