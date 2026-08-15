package recoverysmoke

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

type overlapFaultReceipt struct {
	Kind                                           string             `json:"kind"`
	Observation                                    carrierObservation `json:"observation"`
	ObservedAfterNanos, FaultStartedAfterNanos     int64
	FaultCompletedAfterNanos, CarrierCutAfterNanos int64
	AbsenceAfterNanos                              int64
	Absent                                         bool
}

// RunOverlapFaultAdapter owns the hidden external overlap-controller mode.
func RunOverlapFaultAdapter(arguments []string, output, diagnostics io.Writer) (int, bool) {
	if len(arguments) == 0 || arguments[0] != "overlap-fault" {
		return 0, false
	}
	if len(arguments) != 2 {
		fmt.Fprintln(diagnostics, "internal overlap-fault invocation is invalid")
		return 2, true
	}
	receipt, err := waitAndFaultOverlap(context.Background(), arguments[1], platformCarrierSockets,
		platformDeleteCarrierInterface)
	if err == nil {
		err = json.NewEncoder(output).Encode(receipt)
	}
	if err != nil {
		fmt.Fprintln(diagnostics, err)
		return 2, true
	}
	return 0, true
}

func waitAndFaultOverlap(ctx context.Context, remote string,
	observe func(string) ([]carrierObservation, error), remove func(string) error) (overlapFaultReceipt, error) {
	host, port, err := net.SplitHostPort(remote)
	if err != nil || net.ParseIP(host) == nil || port == "" {
		return overlapFaultReceipt{}, errors.New("overlap remote must be a literal IP endpoint")
	}
	started, deadline := time.Now(), time.Now().Add(30*time.Second)
	for time.Now().Before(deadline) {
		values, observeErr := observe(remote)
		if observeErr != nil {
			return overlapFaultReceipt{}, observeErr
		}
		if len(values) > 1 {
			return overlapFaultReceipt{}, errors.New("overlap replacement Carrier is ambiguous")
		}
		if len(values) == 1 {
			value := values[0]
			observedAt := max(int64(1), time.Since(started).Nanoseconds())
			faultStarted, faultClock := max(observedAt, time.Since(started).Nanoseconds()), time.Now()
			if err := remove(value.InterfaceName); err != nil {
				return overlapFaultReceipt{}, err
			}
			cutAfter := time.Since(faultClock).Nanoseconds()
			for time.Now().Before(faultClock.Add(time.Second)) {
				present, stateErr := carrierInterfacePresent(value.InterfaceName)
				if stateErr != nil {
					return overlapFaultReceipt{}, stateErr
				}
				if !present {
					return overlapFaultReceipt{Kind: "overlap-faulted", Observation: value,
						ObservedAfterNanos: observedAt, FaultStartedAfterNanos: faultStarted,
						FaultCompletedAfterNanos: time.Since(started).Nanoseconds(),
						CarrierCutAfterNanos:     cutAfter, AbsenceAfterNanos: time.Since(faultClock).Nanoseconds(),
						Absent: true}, nil
				}
				time.Sleep(time.Millisecond)
			}
			return overlapFaultReceipt{}, errors.New("overlap Carrier interface remained present")
		}
		select {
		case <-ctx.Done():
			return overlapFaultReceipt{}, ctx.Err()
		case <-time.After(time.Millisecond):
		}
	}
	return overlapFaultReceipt{}, errors.New("replacement Carrier did not appear before the overlap deadline")
}
