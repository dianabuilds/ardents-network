package recoverysmoke

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

const (
	carrierLocalIP = "172.31.21.13"
	carrierRemote  = "172.31.21.14:4604"
)

func (observer dockerObserver) observeCarrier(ctx context.Context, controller string) (carrierObservation, error) {
	raw, err := observer.docker(ctx, 10*time.Second, "exec", controller,
		"/usr/local/bin/ardents-qualify", "carrier-fault", "observe")
	if err != nil {
		return carrierObservation{}, err
	}
	return parseCarrierObservation(raw)
}

func (observer dockerObserver) observeCarrierInNamespace(ctx context.Context,
	rendezvous string) (carrierObservation, recovery.ObserverProcess, error) {
	rawID, err := observer.docker(ctx, 10*time.Second, "create", "--network", "container:"+rendezvous,
		"--ipc", "private", "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
		"--user", "65532:65532",
		"--pids-limit", "16", "--memory", "32m", "--cpus", "0.25",
		"--label", "com.docker.compose.project="+observer.project, observer.imageID,
		"/usr/local/bin/ardents-qualify", "carrier-fault", "observe")
	if err != nil {
		return carrierObservation{}, recovery.ObserverProcess{}, err
	}
	identity := strings.TrimSpace(string(rawID))
	if !validContainerID(identity) {
		return carrierObservation{}, recovery.ObserverProcess{}, errors.New("replacement Carrier observer identity is invalid")
	}
	projection, err := observer.inspectReplacementObserver(ctx, identity)
	if err != nil {
		_, removeErr := observer.docker(context.Background(), 10*time.Second, "rm", "-f", identity)
		return carrierObservation{}, recovery.ObserverProcess{}, errors.Join(err, removeErr)
	}
	raw, startErr := observer.docker(ctx, 10*time.Second, "start", "-a", identity)
	_, removeErr := observer.docker(context.Background(), 10*time.Second, "rm", "-f", identity)
	if startErr != nil || removeErr != nil {
		return carrierObservation{}, recovery.ObserverProcess{}, errors.Join(startErr, removeErr)
	}
	projection.Removed = true
	value, err := parseCarrierObservation(raw)
	return value, projection, err
}

func (observer dockerObserver) inspectReplacementObserver(ctx context.Context,
	identity string) (recovery.ObserverProcess, error) {
	raw, err := observer.docker(ctx, 10*time.Second, "inspect", identity)
	var values []struct {
		ID, Image string
		Config    struct {
			User string
			Cmd  []string
		}
		HostConfig struct {
			NetworkMode      string
			PidMode, IpcMode string
			ReadonlyRootfs   bool
			Privileged       bool
			CapAdd, CapDrop  []string
			SecurityOpt      []string
			PidsLimit        *int64
			Memory, NanoCpus int64
		}
		Mounts []json.RawMessage
	}
	if err != nil {
		return recovery.ObserverProcess{}, fmt.Errorf("inspect replacement observer: %w", err)
	}
	if decodeErr := json.Unmarshal(raw, &values); decodeErr != nil {
		return recovery.ObserverProcess{}, fmt.Errorf("decode replacement observer inspection: %w", decodeErr)
	}
	if len(values) != 1 || values[0].HostConfig.PidsLimit == nil {
		return recovery.ObserverProcess{}, errors.New("replacement observer inspection is invalid")
	}
	value := values[0]
	return recovery.ObserverProcess{ContainerID: value.ID, ImageID: value.Image,
		NetworkMode: value.HostConfig.NetworkMode, User: value.Config.User, Command: value.Config.Cmd,
		PIDMode: value.HostConfig.PidMode, IPCMode: value.HostConfig.IpcMode,
		CapAdd: value.HostConfig.CapAdd, CapDrop: value.HostConfig.CapDrop, SecurityOpt: value.HostConfig.SecurityOpt,
		ReadOnly: value.HostConfig.ReadonlyRootfs, Privileged: value.HostConfig.Privileged,
		MountCount: uint32(len(value.Mounts)), PidsLimit: *value.HostConfig.PidsLimit,
		MemoryLimit: value.HostConfig.Memory, NanoCPUs: value.HostConfig.NanoCpus}, nil
}

func parseCarrierObservation(raw []byte) (carrierObservation, error) {
	var value carrierObservation
	if json.Unmarshal(raw, &value) != nil || len(value.SocketID) != 96 || len(value.SocketIDSHA256) != 64 ||
		value.RemoteAddress != carrierRemote || value.Inode == 0 || value.InterfaceName == "" || value.InterfaceIndex <= 0 {
		return carrierObservation{}, errors.New("external Carrier socket observation is invalid")
	}
	return value, nil
}

type carrierFaultOutcome struct {
	commitment, closedCommitment                                 string
	faultAt, completedAt, cutAfter, absenceAfter, socketClosedAt int64
	controllerRemoved, resourceAbsent, socketClosed              bool
}

func (observer dockerObserver) destroyCarrier(ctx context.Context, controller, rendezvous, network string,
	value carrierObservation, cellClock time.Time) (carrierFaultOutcome, error) {
	result := carrierFaultOutcome{faultAt: time.Since(cellClock).Nanoseconds()}
	raw, err := observer.docker(ctx, 10*time.Second, "exec", controller,
		"/usr/local/bin/ardents-qualify", "carrier-fault", "fault", value.SocketID)
	if err != nil {
		return result, err
	}
	var receipt carrierFaultReceipt
	if json.Unmarshal(raw, &receipt) != nil || receipt.Kind != "faulted" || receipt.SocketIDSHA256 != value.SocketIDSHA256 ||
		receipt.InterfaceName != value.InterfaceName || receipt.CarrierCutAfterNanos <= 0 ||
		receipt.AbsenceAfterNanos < receipt.CarrierCutAfterNanos || !receipt.Absent {
		return result, errors.New("external Carrier fault receipt is invalid")
	}
	result.commitment, result.cutAfter = receipt.SocketIDSHA256, receipt.CarrierCutAfterNanos
	result.absenceAfter, result.resourceAbsent = receipt.AbsenceAfterNanos, true
	closedRaw, err := observer.docker(ctx, 6*time.Second, "exec", controller,
		"/usr/local/bin/ardents-qualify", "carrier-fault", "await-closed", value.SocketID)
	if err != nil {
		return result, err
	}
	var closed carrierClosureReceipt
	if decodeErr := json.Unmarshal(closedRaw, &closed); decodeErr != nil {
		return result, fmt.Errorf("decode external old-Carrier closure receipt: %w", decodeErr)
	}
	if closed.Kind != "closed" || closed.SocketIDSHA256 != value.SocketIDSHA256 ||
		closed.SocketAbsentAfterNanos <= 0 || !closed.Absent {
		return result, errors.New("external old-Carrier closure receipt is invalid")
	}
	result.closedCommitment = closed.SocketIDSHA256
	result.socketClosed, result.socketClosedAt = true, time.Since(cellClock).Nanoseconds()
	_, removeErr := observer.docker(ctx, 10*time.Second, "rm", "-f", controller)
	present, presenceErr := observer.docker(ctx, 10*time.Second, "ps", "-a", "-q", "--no-trunc", "--filter", "id="+controller)
	if presenceErr != nil || strings.TrimSpace(string(present)) != "" {
		return result,
			errors.Join(removeErr, presenceErr, errors.New("Carrier fault controller remained present after removal"))
	}
	result.controllerRemoved = true
	if err := observer.disconnectCarrierNetwork(ctx, network, rendezvous); err != nil {
		return result, err
	}
	if _, err := observer.docker(ctx, 10*time.Second, "network", "connect", "--ip", carrierLocalIP, network, rendezvous); err != nil {
		return result, fmt.Errorf("restore exact Carrier network: %w", err)
	}
	result.completedAt = time.Since(cellClock).Nanoseconds()
	return result, nil
}

func (observer dockerObserver) disconnectCarrierNetwork(ctx context.Context, network, rendezvous string) error {
	retryCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	var lastErr error
	for {
		_, lastErr = observer.docker(retryCtx, 500*time.Millisecond, "network", "disconnect", "-f", network, rendezvous)
		if lastErr == nil {
			return nil
		}
		select {
		case <-retryCtx.Done():
			return errors.Join(fmt.Errorf("disconnect exact Carrier network after controller removal: %w", retryCtx.Err()), lastErr)
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func (observer dockerObserver) routeProcessIdentities(ctx context.Context,
	containers map[string]string) (map[string]string, map[string]uint32, error) {
	containerResult, pidResult := make(map[string]string, 6), make(map[string]uint32, 6)
	for _, role := range []string{"client", "initiator", "introduction", "rendezvous", "responder", "publisher"} {
		container := containers[role]
		raw, err := observer.docker(ctx, 10*time.Second, "inspect", "--format", "{{.State.Pid}}", container)
		pid, parseErr := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 32)
		if err != nil || parseErr != nil || pid == 0 {
			return nil, nil, errors.New("selected Route process identity is unavailable")
		}
		containerResult[role], pidResult[role] = container, uint32(pid)
	}
	return containerResult, pidResult, nil
}
