package network

type BootstrapStatus struct {
	Joined bool
	State  string
	Reason string
}

type BootstrapDialReport struct {
	Peer    string
	Success bool
	Detail  string
}

func UpdateBootstrapFailures(err error, current int, failures *int, lastErr string) string {
	if err != nil {
		*failures = current + 1
		return err.Error()
	}
	return lastErr
}

func ClassifyBootstrapStatus(relayPeerCount, attempts, failures int, _ string) BootstrapStatus {
	if attempts == 0 {
		return BootstrapStatus{State: "idle", Reason: "no network bootstrap sources"}
	}
	if relayPeerCount > 0 {
		return BootstrapStatus{Joined: true, State: "ready"}
	}
	if failures == attempts {
		return BootstrapStatus{State: "degraded", Reason: "bootstrap peer dial failed"}
	}
	return BootstrapStatus{State: "degraded", Reason: "bootstrap relay readiness failed"}
}

func ClassifyLightBootstrapStatus(filterPeers, lightpushPeers, storePeers, attempts, failures int) BootstrapStatus {
	if attempts == 0 {
		return BootstrapStatus{State: "idle", Reason: "no network bootstrap sources"}
	}
	if filterPeers > 0 && lightpushPeers > 0 && storePeers > 0 {
		return BootstrapStatus{Joined: true, State: "ready"}
	}
	if failures == attempts {
		return BootstrapStatus{State: "degraded", Reason: "bootstrap peer dial failed"}
	}
	return BootstrapStatus{
		State: "degraded", Reason: "bootstrap peers do not provide required Filter, Lightpush, and Store protocols",
	}
}
