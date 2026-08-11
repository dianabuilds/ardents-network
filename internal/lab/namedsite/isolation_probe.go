package namedsite

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

const isolationObserverToken = "gatec-forbidden-boundary-observer/v1"

func runIsolationProbe(ctx context.Context, kind, observerName, observerAddress string) error {
	if ctx == nil || kind != "application" || observerName == "" || observerAddress == "" {
		return errors.New("isolation probe is outside the closed contract")
	}
	operation, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if addresses, err := net.DefaultResolver.LookupHost(operation, observerName); err == nil && len(addresses) > 0 {
		return errors.New("application DNS escape succeeded")
	}
	dialer := net.Dialer{Timeout: 300 * time.Millisecond}
	if connection, err := dialer.DialContext(operation, "tcp", observerAddress); err == nil {
		_ = connection.Close()
		return errors.New("application socket escape succeeded")
	}
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		if ordinaryTCPListener(path) {
			return errors.New("application ordinary listener is present")
		}
	}
	return nil
}

func runIsolationObserver(ctx context.Context) error {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", ":18080")
	if err != nil {
		return err
	}
	defer listener.Close()
	connection, err := listener.Accept()
	if err != nil {
		return err
	}
	defer connection.Close()
	_, err = io.WriteString(connection, isolationObserverToken)
	return err
}

func runIsolationControl(ctx context.Context, observerName, observerAddress string) error {
	operation, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for {
		addresses, lookupErr := net.DefaultResolver.LookupHost(operation, observerName)
		if lookupErr == nil && len(addresses) > 0 {
			connection, dialErr := (&net.Dialer{Timeout: 300 * time.Millisecond}).DialContext(operation, "tcp", observerAddress)
			if dialErr == nil {
				payload, readErr := io.ReadAll(io.LimitReader(connection, int64(len(isolationObserverToken)+1)))
				closeErr := connection.Close()
				if readErr == nil && closeErr == nil && string(payload) == isolationObserverToken {
					return nil
				}
				return errors.New("controlled isolation observer response is invalid")
			}
		}
		select {
		case <-operation.Done():
			return errors.New("controlled isolation observer DNS or socket is unreachable")
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func ordinaryTCPListener(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return true
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 3 && fields[3] == "0A" {
			return true
		}
	}
	return scanner.Err() != nil
}
