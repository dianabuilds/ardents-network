package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"strconv"
	"strings"
)

func verifyCarrierEvidence(cell Cell) Result {
	if cell.InitialCarrier == cell.ReplacementCarrier {
		return fail("replacement reused the failed carrier")
	}
	for _, carrier := range []string{cell.InitialCarrier, cell.ReplacementCarrier} {
		raw, err := hex.DecodeString(carrier)
		if err != nil || len(raw) != sha256.Size {
			return invalid("Carrier socket commitment is malformed")
		}
	}
	if cell.DestroyedCarrier != cell.InitialCarrier || cell.InitialCarrierInode == 0 || cell.ReplacementCarrierInode == 0 ||
		cell.InitialCarrierInterface == "" || cell.InitialCarrierInterface != cell.ReplacementCarrierInterface ||
		cell.InitialCarrierInterfaceIndex <= 0 || cell.InitialCarrierInterfaceIndex != cell.ReplacementCarrierInterfaceIndex ||
		!carrierEndpoint(cell.InitialCarrierLocal, "172.31.21.13", false) ||
		!carrierEndpoint(cell.InitialCarrierRemote, "172.31.21.14", true) ||
		!carrierEndpoint(cell.ReplacementCarrierLocal, "172.31.21.13", false) ||
		!carrierEndpoint(cell.ReplacementCarrierRemote, "172.31.21.14", true) {
		return invalid("native Carrier tuple, inode, or destroy receipt is incomplete")
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
	}
	if cell.FaultContainer != cell.InitialRouteContainers["rendezvous"] {
		return invalid("faulted Carrier host identity does not match Rendezvous")
	}
	return Result{Verdict: "pass"}
}

func carrierEndpoint(value, expectedIP string, fixedPort bool) bool {
	host, portText, err := net.SplitHostPort(value)
	port, parseErr := strconv.ParseUint(portText, 10, 16)
	if err != nil || parseErr != nil || port == 0 || !net.ParseIP(host).Equal(net.ParseIP(expectedIP)) {
		return false
	}
	return !fixedPort || port == 4604
}
