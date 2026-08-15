//go:build ignore

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type campaignReport struct {
	Candidate               string              `json:"candidate"`
	Verdict                 string              `json:"verdict"`
	StartedAt               string              `json:"started_at"`
	StartupMilliseconds     int64               `json:"startup_milliseconds"`
	UsefulWorkMilliseconds  int64               `json:"useful_work_milliseconds"`
	RequestSHA256           string              `json:"request_sha256"`
	ResponseSHA256          string              `json:"response_sha256"`
	DNSPackets              int64               `json:"dns_packets"`
	DNSControlPackets       int64               `json:"dns_control_packets"`
	DNSAmbiguousPackets     int64               `json:"dns_ambiguous_packets"`
	DNSObserverCapabilities string              `json:"dns_observer_capabilities"`
	Client                  processObservation  `json:"client"`
	Server                  processObservation  `json:"server"`
	Resources               []resourceSample    `json:"resource_series"`
	ClientControlSHA256     string              `json:"client_control_sha256"`
	ServerControlSHA256     string              `json:"server_control_sha256"`
	ClientShutdownRung      string              `json:"client_shutdown_rung"`
	ServerShutdownRung      string              `json:"server_shutdown_rung"`
	CleanupMilliseconds     int64               `json:"cleanup_milliseconds"`
	StateEntries            int                 `json:"state_entries"`
	StateBytes              int64               `json:"state_bytes"`
	RequestedShutdownRung   string              `json:"requested_shutdown_rung"`
	Residual                residualObservation `json:"residual"`
	UsefulWorkVerified      bool                `json:"useful_work_verified"`
	RunManifestSHA256       string              `json:"run_manifest_sha256"`
}

func run(candidate, evidence, observerSync string, provenance runProvenance) (err error) {
	if candidate != "obfs4" && candidate != "webtunnel" {
		return errors.New("candidate must be obfs4 or webtunnel")
	}
	if evidence == "" || !filepath.IsAbs(evidence) {
		return errors.New("evidence path must be absolute")
	}
	if observerSync == "" || !filepath.IsAbs(observerSync) {
		return errors.New("observer sync path must be absolute")
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
	for _, stale := range []string{"summary.json", "summary.json.tmp", "secret"} {
		if err := os.RemoveAll(filepath.Join(evidence, stale)); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(observerSync, 0700); err != nil {
		return err
	}
	if err := waitDNSObserver(observerSync); err != nil {
		return err
	}
	if err := sendDNSObserverControl(); err != nil {
		return err
	}
	if err := closeDNSObserverControl(observerSync); err != nil {
		return err
	}
	observerDone := false
	defer func() {
		if !observerDone {
			_, stopErr := stopDNSObserver(observerSync)
			err = errors.Join(err, stopErr)
		}
	}()
	work, err := os.MkdirTemp("", "r036-"+candidate+"-")
	if err != nil {
		return err
	}
	if _, err := reserveAddress(); err != nil {
		return err
	}
	baseline, err := captureResidualBaseline()
	if err != nil {
		return err
	}
	echo, echoAddress, err := startEcho(workload)
	if err != nil {
		return err
	}
	owned := campaignOwned{work: work, echo: echo}
	cleaned := false
	defer func() {
		if !cleaned {
			err = errors.Join(err, emergencyCleanup(&owned))
		}
	}()
	report := campaignReport{Candidate: candidate, Verdict: "fail",
		StartedAt:             time.Now().UTC().Format(time.RFC3339Nano),
		RequestedShutdownRung: provenance.ShutdownRung}
	started := time.Now()
	startupDeadline := started.Add(5 * time.Second)
	if err := openCampaign(candidate, work, echoAddress, startupDeadline, &owned); err != nil {
		return err
	}
	server, client, carrier := owned.server, owned.client, owned.carrier
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
	if err := validatePIDNamespace(baseline, client.cmd.Process.Pid, server.cmd.Process.Pid); err != nil {
		return err
	}
	first, err := captureResourceSample(started, client, server, work)
	if err != nil {
		return err
	}
	report.Resources = append(report.Resources, first)
	workStarted := time.Now()
	report.RequestSHA256, report.ResponseSHA256, err = exerciseCarrier(carrier, workload)
	if err != nil {
		return err
	}
	report.UsefulWorkMilliseconds = time.Since(workStarted).Milliseconds()
	report.UsefulWorkVerified = true
	for range 2 {
		time.Sleep(25 * time.Millisecond)
		sample, err := captureResourceSample(started, client, server, work)
		if err != nil {
			return err
		}
		report.Resources = append(report.Resources, sample)
	}
	if err := cleanupCampaign(&owned, baseline, observerSync, evidence, provenance, &report); err != nil {
		return err
	}
	cleaned, observerDone = true, true
	return nil
}

func openCampaign(candidate, work, echoAddress string, deadline time.Time, owned *campaignOwned) error {
	method := candidate
	serverPath, clientPath := "/candidate/lyrebird", "/candidate/lyrebird"
	if candidate == "webtunnel" {
		serverPath, clientPath = "/candidate/webtunnel-server", "/candidate/webtunnel-client"
	}
	serverBind, err := reserveAddress()
	if err != nil {
		return err
	}
	serverState := filepath.Join(work, "server")
	clientState := filepath.Join(work, "client")
	if err := os.MkdirAll(serverState, 0700); err != nil {
		return err
	}
	serverEnv := map[string]string{
		"TOR_PT_MANAGED_TRANSPORT_VER": "1", "TOR_PT_SERVER_TRANSPORTS": method,
		"TOR_PT_SERVER_BINDADDR": method + "-" + serverBind, "TOR_PT_ORPORT": echoAddress,
		"TOR_PT_STATE_LOCATION": serverState, "TOR_PT_EXIT_ON_STDIN_CLOSE": "1",
	}
	if candidate == "webtunnel" {
		serverEnv["TOR_PT_SERVER_TRANSPORT_OPTIONS"] = "webtunnel:url=https://bridge.invalid" + webTunnelPath
	}
	server, serverReady, err := startChild(serverPath, serverEnv, method, "SMETHOD", deadline)
	owned.server = server
	if err != nil {
		return err
	}
	target := serverBind
	args := map[string]string{}
	var front *tlsFront
	if candidate == "obfs4" {
		args["cert"] = serverReady.args["cert"]
		args["iat-mode"] = "0"
		if args["cert"] == "" {
			return errors.New("obfs4 server omitted cert")
		}
	} else {
		front, target, args["cert"], err = startTLSFront("192.0.2.3", serverBind)
		owned.front = front
		if err != nil {
			return err
		}
		if err := front.checkOrdinary(target, deadline); err != nil {
			return err
		}
		args["url"] = "https://" + target + webTunnelPath
		args["servername"] = "bridge.invalid"
	}
	if err := os.MkdirAll(clientState, 0700); err != nil {
		return err
	}
	clientEnv := map[string]string{
		"TOR_PT_MANAGED_TRANSPORT_VER": "1", "TOR_PT_CLIENT_TRANSPORTS": method,
		"TOR_PT_STATE_LOCATION": clientState, "TOR_PT_EXIT_ON_STDIN_CLOSE": "1",
	}
	client, clientReady, err := startChild(clientPath, clientEnv, method, "CMETHOD", deadline)
	owned.client = client
	if err != nil {
		return err
	}
	carrier, err := openSOCKS(clientReady.address, target, args, deadline)
	if err != nil {
		return err
	}
	owned.carrier = carrier
	return nil
}
