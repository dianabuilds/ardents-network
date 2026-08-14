package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"strconv"
	"strings"
)

func verifyCarrierEvidence(cell Cell, imageID string) Result {
	if cell.InitialCarrier == cell.ReplacementCarrier {
		return fail("replacement reused the failed carrier")
	}
	for _, carrier := range []string{cell.InitialCarrier, cell.ReplacementCarrier} {
		raw, err := hex.DecodeString(carrier)
		if err != nil || len(raw) != sha256.Size {
			return invalid("Carrier socket commitment is malformed")
		}
	}
	if cell.FaultedCarrier != cell.InitialCarrier || cell.ClosedCarrier != cell.InitialCarrier ||
		cell.InitialCarrierInode == 0 || cell.ReplacementCarrierInode == 0 ||
		cell.InitialCarrierInterface == "" || cell.InitialCarrierInterface != cell.ReplacementCarrierInterface ||
		cell.InitialCarrierInterfaceIndex <= 0 || cell.ReplacementCarrierInterfaceIndex <= 0 ||
		cell.InitialCarrierInterfaceIndex == cell.ReplacementCarrierInterfaceIndex ||
		!carrierEndpoint(cell.InitialCarrierLocal, "172.31.21.13", false) ||
		!carrierEndpoint(cell.InitialCarrierRemote, "172.31.21.14", true) ||
		!carrierEndpoint(cell.ReplacementCarrierLocal, "172.31.21.13", false) ||
		!carrierEndpoint(cell.ReplacementCarrierRemote, "172.31.21.14", true) {
		return invalid("native Carrier tuple, inode, or fault receipt is incomplete")
	}
	if cell.FaultService != "rendezvous-responder-carrier" {
		return invalid("fault did not identify the native same-leg Carrier socket")
	}
	if !strings.HasSuffix(cell.FaultNetwork, "_carrier_net") ||
		strings.TrimSuffix(cell.FaultNetwork, "_carrier_net") == "" {
		return invalid("fault network does not bind the dedicated Carrier topology")
	}
	roles := []string{"client", "initiator", "introduction", "rendezvous", "responder", "publisher"}
	if len(cell.InitialRouteContainers) != len(roles) || len(cell.RecoveredRouteContainers) != len(roles) ||
		len(cell.InitialRoutePIDs) != len(roles) || len(cell.RecoveredRoutePIDs) != len(roles) {
		return invalid("selected Route process evidence is incomplete")
	}
	for _, role := range roles {
		initialContainer, initialContainerOK := cell.InitialRouteContainers[role]
		recoveredContainer, recoveredContainerOK := cell.RecoveredRouteContainers[role]
		initialPID, initialPIDOK := cell.InitialRoutePIDs[role]
		recoveredPID, recoveredPIDOK := cell.RecoveredRoutePIDs[role]
		if !initialContainerOK || !recoveredContainerOK || !initialPIDOK || !recoveredPIDOK ||
			initialContainer == "" || initialPID == 0 || initialContainer != recoveredContainer || initialPID != recoveredPID {
			return fail("selected Route process changed during Carrier recovery: " + role)
		}
		if cell.FaultController == initialContainer {
			return invalid("fault controller identity overlaps a selected Route process")
		}
		if cell.ReplacementObserver.ContainerID == initialContainer {
			return invalid("replacement observer identity overlaps a selected Route process")
		}
	}
	for _, process := range []string{cell.ClientProcess, cell.PublisherProcess,
		cell.ClientApplicationProcess, cell.PublisherApplicationProcess} {
		if cell.FaultController == process {
			return invalid("fault controller identity overlaps an Endpoint or Application process")
		}
	}
	observer := cell.ReplacementObserver
	if !containerID(cell.FaultController) || !cell.FaultControllerRemoved {
		return invalid("fault controller identity or removal evidence is incomplete")
	}
	if observer.ContainerID == cell.FaultController {
		return invalid("replacement observation did not use a separately confined process")
	}
	for _, process := range []string{cell.ClientProcess, cell.PublisherProcess,
		cell.ClientApplicationProcess, cell.PublisherApplicationProcess} {
		if observer.ContainerID == process {
			return invalid("replacement observer identity overlaps an Endpoint or Application process")
		}
	}
	if !containerID(observer.ContainerID) || observer.ImageID != imageID ||
		observer.NetworkMode != "container:"+cell.InitialRouteContainers["rendezvous"] ||
		observer.User != "65532:65532" || !exactStrings(observer.Command,
		[]string{"/usr/local/bin/ardents-qualify", "carrier-fault", "observe"}) ||
		len(observer.CapAdd) != 0 || observer.PIDMode != "" || observer.IPCMode != "private" || observer.Privileged ||
		!exactStrings(observer.CapDrop, []string{"ALL"}) ||
		!exactStrings(observer.SecurityOpt, []string{"no-new-privileges"}) ||
		!observer.ReadOnly || !observer.Removed || observer.MountCount != 0 ||
		observer.PidsLimit != 16 || observer.MemoryLimit != 32<<20 || observer.NanoCPUs != 250_000_000 {
		return invalid("replacement observer confinement or cleanup projection is incomplete")
	}
	if cell.FaultContainer != cell.InitialRouteContainers["rendezvous"] {
		return invalid("faulted Carrier host identity does not match Rendezvous")
	}
	return Result{Verdict: "pass"}
}

func containerID(value string) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == 32 && value == strings.ToLower(value)
}

func exactStrings(actual, expected []string) bool {
	return len(actual) == len(expected) && strings.Join(actual, "\x00") == strings.Join(expected, "\x00")
}

func carrierEndpoint(value, expectedIP string, fixedPort bool) bool {
	host, portText, err := net.SplitHostPort(value)
	port, parseErr := strconv.ParseUint(portText, 10, 16)
	if err != nil || parseErr != nil || port == 0 || !net.ParseIP(host).Equal(net.ParseIP(expectedIP)) {
		return false
	}
	return !fixedPort || port == 4604
}
