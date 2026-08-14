package recoverysmoke

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
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
	var value carrierObservation
	if json.Unmarshal(raw, &value) != nil || len(value.SocketID) != 96 || len(value.SocketIDSHA256) != 64 ||
		value.RemoteAddress != carrierRemote || value.Inode == 0 || value.InterfaceName == "" || value.InterfaceIndex <= 0 {
		return carrierObservation{}, errors.New("external Carrier socket observation is invalid")
	}
	return value, nil
}

func (observer dockerObserver) destroyCarrier(ctx context.Context, controller string,
	value carrierObservation, cellClock time.Time) (digest string, faultAt, completedAt int64,
	downAfter, absenceAfter, restoredAfter int64,
	absent bool, err error) {
	faultAt = time.Since(cellClock).Nanoseconds()
	raw, err := observer.docker(ctx, 10*time.Second, "exec", controller,
		"/usr/local/bin/ardents-qualify", "carrier-fault", "fault", value.SocketID)
	if err != nil {
		return "", 0, 0, 0, 0, 0, false, err
	}
	completedAt = time.Since(cellClock).Nanoseconds()
	var receipt carrierFaultReceipt
	if json.Unmarshal(raw, &receipt) != nil || receipt.Kind != "faulted" || receipt.SocketIDSHA256 != value.SocketIDSHA256 ||
		receipt.InterfaceName != value.InterfaceName || receipt.CarrierDownAfterNanos <= 0 ||
		receipt.AbsenceAfterNanos < receipt.CarrierDownAfterNanos || receipt.RestoredAfterNanos < receipt.AbsenceAfterNanos ||
		!receipt.Absent || !receipt.Restored {
		return "", 0, 0, 0, 0, 0, false, errors.New("external Carrier fault receipt is invalid")
	}
	return receipt.SocketIDSHA256, faultAt, completedAt, receipt.CarrierDownAfterNanos,
		receipt.AbsenceAfterNanos, receipt.RestoredAfterNanos, true, nil
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
