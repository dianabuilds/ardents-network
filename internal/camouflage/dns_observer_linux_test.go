//go:build linux

package camouflage

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

type dnsObservation struct {
	Packets, Controls, Ambiguous int64
}

func TestMain(tests *testing.M) {
	root := os.Getenv("ARDENTS_DNS_SYNC")
	if os.Getenv("ARDENTS_DNS_OBSERVER") == "1" {
		if err := observeDNS(root); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		os.Exit(0)
	}
	if root == "" {
		os.Exit(tests.Run())
	}
	if err := prepareDNSProof(root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	code := tests.Run()
	observation, err := stopDNSProof(root)
	if err != nil || observation.Controls < 6 || observation.Packets != 0 || observation.Ambiguous != 0 {
		fmt.Fprintf(os.Stderr, "DNS proof = %+v, error = %v\n", observation, err)
		os.Exit(2)
	}
	fmt.Printf("DNS proof: controls=%d candidate=%d ambiguous=%d\n",
		observation.Controls, observation.Packets, observation.Ambiguous)
	os.Exit(code)
}

func observeDNS(root string) error {
	if !filepath.IsAbs(root) {
		return errors.New("DNS observer root is not absolute")
	}
	if err := os.MkdirAll(root, 0o777); err != nil {
		return err
	}
	if err := os.Chmod(root, 0o777); err != nil {
		return err
	}
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(networkShort(0x0003)))
	if err != nil {
		return err
	}
	defer syscall.Close(fd)
	timeout := syscall.NsecToTimeval((100 * time.Millisecond).Nanoseconds())
	if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &timeout); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "ready"), []byte("ready\n"), 0o666); err != nil {
		return err
	}
	var observation dnsObservation
	packet := make([]byte, 65535)
	control := true
	controlDone := false
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if control && exists(filepath.Join(root, "control-done")) {
			controlDone = true
		}
		if exists(filepath.Join(root, "stop")) {
			raw, _ := json.Marshal(observation)
			return os.WriteFile(filepath.Join(root, "result.json"), raw, 0o666)
		}
		n, _, receiveErr := syscall.Recvfrom(fd, packet, 0)
		if receiveErr == syscall.EAGAIN || receiveErr == syscall.EWOULDBLOCK || receiveErr == syscall.EINTR {
			if controlDone && receiveErr != syscall.EINTR {
				control, controlDone = false, false
				if err := os.WriteFile(filepath.Join(root, "control-stopped"), []byte("stopped\n"), 0o666); err != nil {
					return err
				}
			}
			continue
		}
		if receiveErr != nil {
			return receiveErr
		}
		if _, _, ambiguous, ok := packetTransport(packet[:n]); ok && ambiguous {
			observation.Ambiguous++
		} else if isDNSControl(packet[:n], control) {
			observation.Controls++
		} else if isDNSPacket(packet[:n]) {
			observation.Packets++
		}
	}
	return errors.New("DNS observer timed out")
}

func prepareDNSProof(root string) error {
	if err := waitForFile(filepath.Join(root, "ready"), 5*time.Second); err != nil {
		return err
	}
	for _, target := range []struct{ network, address string }{
		{"udp4", "127.0.0.1:53"}, {"udp6", "[::1]:53"},
	} {
		connection, err := net.Dial(target.network, target.address)
		if err != nil {
			return err
		}
		_, err = connection.Write(dnsControlPayload)
		_ = connection.Close()
		if err != nil {
			return err
		}
	}
	if err := sendTCPDNSControl(); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "control-done"), []byte("done\n"), 0o600); err != nil {
		return err
	}
	return waitForFile(filepath.Join(root, "control-stopped"), 2*time.Second)
}

func sendTCPDNSControl() error {
	listener, err := net.Listen("tcp4", "127.0.0.1:53")
	if err != nil {
		return err
	}
	accepted := make(chan error, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			_, acceptErr = connection.Write(dnsControlPayload)
			_ = connection.Close()
		}
		accepted <- acceptErr
	}()
	dialer := net.Dialer{LocalAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 53053}}
	connection, err := dialer.Dial("tcp4", "127.0.0.1:53")
	if err == nil {
		_ = connection.Close()
	}
	_ = listener.Close()
	return errors.Join(err, <-accepted)
}

func stopDNSProof(root string) (dnsObservation, error) {
	if err := os.WriteFile(filepath.Join(root, "stop"), []byte("stop\n"), 0o600); err != nil {
		return dnsObservation{}, err
	}
	path := filepath.Join(root, "result.json")
	if err := waitForFile(path, 2*time.Second); err != nil {
		return dnsObservation{}, err
	}
	raw, err := os.ReadFile(path)
	var result dnsObservation
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return result, err
	}
	return result, nil
}

func waitForFile(path string, bound time.Duration) error {
	deadline := time.Now().Add(bound)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		time.Sleep(25 * time.Millisecond)
	}
	return errors.New("DNS observer synchronization timed out")
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func networkShort(value uint16) uint16 { return value<<8 | value>>8 }
