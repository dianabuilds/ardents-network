package recovery

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

func verifyReplacementDockerTopology(raw []byte) error {
	blocks := composeServiceBlocks(string(raw))
	for _, role := range replacementRoles {
		for _, suffix := range []string{"-2", "-3"} {
			block := blocks[role+suffix]
			if block == "" || !strings.Contains(block, "route_net") || !strings.Contains(block, "restart: \"no\"") ||
				strings.Contains(block, "carrier_net") || strings.Contains(block, "network_mode:") {
				return errors.New("S4.2 alternate candidate topology is incomplete or widened")
			}
		}
	}
	if !strings.Contains(blocks["client"], "/run/ardents/recovery-introduction-user") ||
		!strings.Contains(blocks["introduction-3"], "/run/ardents/recovery-introduction-user") ||
		!strings.Contains(blocks["introduction-3"], "/run/ardents/recovery-introduction-service") ||
		!strings.Contains(blocks["publisher"], "/run/ardents/recovery-introduction-service") {
		return errors.New("S4.2 sealed Introduction control path is absent")
	}
	allowedUser := map[string]bool{"volume-init": true, "client": true, "introduction-3": true}
	allowedService := map[string]bool{"volume-init": true, "introduction-3": true, "publisher": true}
	for name, block := range blocks {
		if strings.Contains(block, "/run/ardents/recovery-introduction-user") && !allowedUser[name] ||
			strings.Contains(block, "/run/ardents/recovery-introduction-service") && !allowedService[name] {
			return errors.New("S4.2 sealed Introduction control path reached an unrelated process")
		}
	}
	return nil
}

type dockerProcessPublicProjection struct {
	Image, Path, Project, Service, PIDMode string
	Arguments                              []string
}

func validDockerReplacementProcesses(cell replacementCell, scope hostScopeEvidence) bool {
	for _, service := range replacementEndpointProcessRoles {
		if !validDockerProcessObservation(cell.HostProcesses[service], scope, service) {
			return false
		}
	}
	for _, service := range replacementRouteProcessRoles {
		if !validDockerProcessObservation(cell.HostProcesses[service], scope, service) {
			return false
		}
	}
	for _, route := range cell.Routes {
		for role, process := range route.Processes {
			if !strings.HasPrefix(process.Service, role) ||
				!validDockerCandidateProcess(process, scope) {
				return false
			}
		}
	}
	for _, proposal := range cell.Proposals {
		for index, process := range proposal.Processes {
			if !strings.HasPrefix(process.Service, replacementRoles[index]) ||
				!validDockerCandidateProcess(process, scope) ||
				proposal.Stopped[index].ContainerID != process.ContainerID {
				return false
			}
		}
	}
	for _, event := range cell.Events {
		for _, process := range []candidateProcess{event.Failed, event.Replacement,
			event.RendezvousBefore, event.RendezvousAfter, event.Introduction} {
			if !validDockerCandidateProcess(process, scope) {
				return false
			}
		}
		if event.FailedResource.ContainerID != event.Failed.ContainerID ||
			!fullContainerID(event.FailedResource.ContainerID) {
			return false
		}
	}
	return true
}

func validDockerReplacementFailureProcesses(cell replacementCell, scope hostScopeEvidence) bool {
	prefix := cell
	prefix.Events = nil
	if !validDockerReplacementProcesses(prefix, scope) {
		return false
	}
	for _, event := range cell.Events {
		for _, process := range []candidateProcess{event.Failed, event.Replacement,
			event.RendezvousBefore, event.RendezvousAfter, event.Introduction} {
			if !validDockerCandidateProcess(process, scope) {
				return false
			}
		}
	}
	return true
}

func validDockerCandidateProcess(process candidateProcess, scope hostScopeEvidence) bool {
	observation := processObservationEvidence{Host: process.Host, PID: process.PID,
		ObservedAtNanos: process.ObservedAtNanos, HostObservation: process.HostObservation,
		AdapterProjection: process.AdapterProjection}
	if !validDockerProcessObservation(observation, scope, process.Service) ||
		process.Host.Identity != process.ContainerID || process.Host.Incarnation != process.Incarnation {
		return false
	}
	return true
}

func validDockerProcessObservation(process processObservationEvidence, scope hostScopeEvidence,
	service string) bool {
	if len(process.AdapterProjection) == 0 || len(process.AdapterProjection) > 64<<10 {
		return false
	}
	value := process.Host
	if _, ok := processStartedAt(value.Incarnation, value.Identity); !ok {
		return false
	}
	var projection dockerProcessPublicProjection
	decoder := json.NewDecoder(bytes.NewReader([]byte(process.AdapterProjection)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&projection) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return false
	}
	canonical, err := json.Marshal(projection)
	if err != nil || string(canonical) != process.AdapterProjection {
		return false
	}
	if len(projection.Image) > 256 || len(projection.Path) > 4096 || len(projection.Project) > 128 ||
		len(projection.Service) > 128 || len(projection.PIDMode) > 32 || len(projection.Arguments) > 64 {
		return false
	}
	for _, argument := range projection.Arguments {
		if len(argument) > 4096 {
			return false
		}
	}
	executable := dockerProcessProjection("ardents-qualification-executable-v1\x00",
		append([]string{projection.Image, projection.Path}, projection.Arguments...)...)
	tree := dockerProcessProjection("ardents-qualification-process-tree-v1\x00",
		projection.Project, projection.Service, projection.PIDMode, value.Identity)
	return value.Adapter == "docker-compose-v1" && projection.Image != "" && projection.Path != "" &&
		sha256.Sum256([]byte(projection.Image)) == scope.Image && projection.Project == scope.AdapterProjection &&
		projection.Service == service && (projection.PIDMode == "" || projection.PIDMode == "private") &&
		value.Executable == executable && value.Tree == tree &&
		fullContainerID(value.Identity) && process.PID != 0
}

func dockerProcessProjection(domain string, values ...string) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte(domain))
	for _, value := range values {
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
	}
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}
