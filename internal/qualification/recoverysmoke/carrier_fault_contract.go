package recoverysmoke

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// RunCarrierFaultAdapter owns the hidden controller mode of the authorized qualification command.
func RunCarrierFaultAdapter(arguments []string, output, diagnostics io.Writer) (int, bool) {
	if len(arguments) == 0 || arguments[0] != "carrier-fault" {
		return 0, false
	}
	if err := executeCarrierFault(arguments[1:], output); err != nil {
		fmt.Fprintln(diagnostics, err)
		return 2, true
	}
	return 0, true
}

type carrierObservation struct {
	SocketID       string `json:"socket_id"`
	SocketIDSHA256 string `json:"socket_id_sha256"`
	LocalAddress   string `json:"local_address"`
	RemoteAddress  string `json:"remote_address"`
	Inode          uint32 `json:"inode"`
	InterfaceName  string `json:"interface_name"`
	InterfaceIndex int    `json:"interface_index"`
}

type carrierFaultReceipt struct {
	Kind                 string `json:"kind"`
	SocketIDSHA256       string `json:"socket_id_sha256"`
	InterfaceName        string `json:"interface_name"`
	CarrierCutAfterNanos int64  `json:"carrier_cut_after_nanos"`
	AbsenceAfterNanos    int64  `json:"absence_after_nanos"`
	Absent               bool   `json:"absent"`
}

func executeCarrierFault(arguments []string, output io.Writer) error {
	encoder := json.NewEncoder(output)
	if len(arguments) == 1 && arguments[0] == "wait" {
		ready := func() { _ = encoder.Encode(map[string]string{"kind": "ready"}) }
		waitCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		err := carrierFaultWait(waitCtx, ready)
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}
	if len(arguments) == 1 && arguments[0] == "observe" {
		value, err := observeCarrierSocket(carrierRemote)
		if err != nil {
			return err
		}
		if err := validateDedicatedCarrier(value); err != nil {
			return err
		}
		return encoder.Encode(value)
	}
	if len(arguments) == 2 && arguments[0] == "fault" {
		value, err := faultCarrierSocket(arguments[1])
		if err != nil {
			return err
		}
		return encoder.Encode(value)
	}
	return errors.New("internal carrier-fault invocation is invalid")
}

func carrierFaultWait(ctx context.Context, ready func()) error {
	if ready != nil {
		ready()
	}
	<-ctx.Done()
	return ctx.Err()
}

func observeCarrierSocket(remote string) (carrierObservation, error) {
	host, port, err := net.SplitHostPort(remote)
	if err != nil || net.ParseIP(host) == nil || port == "" {
		return carrierObservation{}, errors.New("carrier remote must be a literal IP endpoint")
	}
	sockets, err := platformCarrierSockets(remote)
	if err != nil {
		return carrierObservation{}, err
	}
	if len(sockets) != 1 {
		return carrierObservation{}, errors.New("exactly one established Carrier socket is required")
	}
	return sockets[0], nil
}

func faultCarrierSocket(socketID string) (receipt carrierFaultReceipt, err error) {
	value, _, err := decodeCarrierSocketID(socketID)
	if err != nil {
		return receipt, err
	}
	if err := validateDedicatedCarrier(value); err != nil {
		return receipt, err
	}
	interfaceName, _, err := platformCarrierInterfaceForAddress(value.LocalAddress)
	if err != nil {
		return receipt, err
	}
	started := time.Now()
	if err := platformDeleteCarrierInterface(interfaceName); err != nil {
		return receipt, err
	}
	cutAfter := time.Since(started).Nanoseconds()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		present, statusErr := carrierInterfacePresent(interfaceName)
		if statusErr != nil {
			return receipt, statusErr
		}
		if !present {
			return carrierFaultReceipt{Kind: "faulted", SocketIDSHA256: value.SocketIDSHA256,
				InterfaceName: interfaceName, CarrierCutAfterNanos: cutAfter,
				AbsenceAfterNanos: time.Since(started).Nanoseconds(), Absent: true}, nil
		}
		time.Sleep(time.Millisecond)
	}
	return receipt, errors.New("faulted Carrier interface remained present")
}

func carrierInterfacePresent(name string) (bool, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return false, err
	}
	for _, candidate := range interfaces {
		if candidate.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func validateDedicatedCarrier(value carrierObservation) error {
	localHost, localPort, localErr := net.SplitHostPort(value.LocalAddress)
	if localErr != nil || localHost != carrierLocalIP || localPort == "" || value.RemoteAddress != carrierRemote {
		return errors.New("socket is outside the dedicated Carrier leg")
	}
	return nil
}

func decodeCarrierSocketID(encoded string) (carrierObservation, []byte, error) {
	raw, err := hex.DecodeString(encoded)
	if err != nil || len(raw) != carrierSocketIDBytes {
		return carrierObservation{}, nil, errors.New("carrier socket identity is invalid")
	}
	return carrierObservationFromID(raw, 0), raw, nil
}

func carrierObservationFromID(raw []byte, inode uint32) carrierObservation {
	digest := sha256.Sum256(raw)
	return carrierObservation{SocketID: hex.EncodeToString(raw), SocketIDSHA256: hex.EncodeToString(digest[:]),
		LocalAddress: carrierSocketEndpoint(raw, true), RemoteAddress: carrierSocketEndpoint(raw, false), Inode: inode}
}
