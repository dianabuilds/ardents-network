//go:build ignore

package main

import (
	"errors"
	"net"
	"os"
	"time"
)

type campaignOwned struct {
	work    string
	echo    *fixture
	front   *tlsFront
	client  *child
	server  *child
	carrier net.Conn
}

func cleanupCampaign(owned *campaignOwned, baseline residualBaseline, observerSync,
	evidence string, provenance runProvenance, report *campaignReport,
) error {
	started := time.Now()
	if owned.carrier != nil {
		if err := owned.carrier.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			return err
		}
	}
	clientPID, serverPID := owned.client.cmd.Process.Pid, owned.server.cmd.Process.Pid
	var err error
	report.ClientShutdownRung, _, err = owned.client.stop(provenance.ShutdownRung)
	if err != nil {
		return err
	}
	report.ServerShutdownRung, _, err = owned.server.stop(provenance.ShutdownRung)
	if err != nil {
		return err
	}
	if report.ClientShutdownRung != provenance.ShutdownRung || report.ServerShutdownRung != provenance.ShutdownRung {
		return errors.New("candidate did not stop at the requested rung")
	}
	clientAgain, _, clientErr := owned.client.stop(provenance.ShutdownRung)
	serverAgain, _, serverErr := owned.server.stop(provenance.ShutdownRung)
	if clientErr != nil || serverErr != nil || clientAgain != report.ClientShutdownRung ||
		serverAgain != report.ServerShutdownRung {
		return errors.New("candidate shutdown was not idempotent")
	}
	if owned.front != nil {
		owned.front.close()
	}
	if err := owned.echo.close(); err != nil {
		return err
	}
	report.ClientControlSHA256 = owned.client.transcriptHash()
	report.ServerControlSHA256 = owned.server.transcriptHash()
	report.StateEntries, report.StateBytes, err = scanState(owned.work)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(owned.work); err != nil {
		return err
	}
	dns, err := stopDNSObserver(observerSync)
	if err != nil {
		return err
	}
	report.DNSPackets = dns.Packets
	report.DNSControlPackets = dns.ControlPackets
	report.DNSAmbiguousPackets = dns.AmbiguousPackets
	report.DNSObserverCapabilities = dns.Capabilities
	if err := validateObserver(dns); err != nil {
		return err
	}
	report.Residual, err = verifyResidual(baseline, clientPID, serverPID, owned.work)
	if err != nil {
		return err
	}
	report.RunManifestSHA256, err = writeEvidence(evidence, owned.client, owned.server, provenance)
	if err != nil {
		return err
	}
	report.Verdict = "pass"
	return publishSummary(evidence, report, started)
}

func emergencyCleanup(owned *campaignOwned) error {
	var result error
	if owned.carrier != nil {
		if err := owned.carrier.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			result = errors.Join(result, err)
		}
	}
	if owned.client != nil {
		result = errors.Join(result, owned.client.forceStop())
	}
	if owned.server != nil {
		result = errors.Join(result, owned.server.forceStop())
	}
	if owned.front != nil {
		owned.front.close()
	}
	if owned.echo != nil {
		result = errors.Join(result, owned.echo.close())
	}
	if owned.work != "" {
		result = errors.Join(result, os.RemoveAll(owned.work))
	}
	return result
}
