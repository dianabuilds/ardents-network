//go:build ignore

package main

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const ethernetAll = 0x0003

var observerControlPayload = []byte("ardents-r036-dns-observer-control")

type dnsObservation struct {
	Packets          int64  `json:"packets"`
	ControlPackets   int64  `json:"control_packets"`
	AmbiguousPackets int64  `json:"ambiguous_packets"`
	Capabilities     string `json:"observer_capabilities"`
}

func runDNSObserver(syncRoot string) error {
	if !filepath.IsAbs(syncRoot) {
		return errors.New("observer sync path must be absolute")
	}
	if err := os.MkdirAll(syncRoot, 0755); err != nil {
		return err
	}
	if err := os.Chmod(syncRoot, 0777); err != nil {
		return err
	}
	ready := filepath.Join(syncRoot, "observer-ready")
	stop := filepath.Join(syncRoot, "observer-stop")
	controlStop := filepath.Join(syncRoot, "observer-control-stop")
	controlStopped := filepath.Join(syncRoot, "observer-control-stopped")
	result := filepath.Join(syncRoot, "dns-observation.json")
	for _, stale := range []string{ready, stop, controlStop, controlStopped, result} {
		_ = os.Remove(stale)
	}
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(networkShort(ethernetAll)))
	if err != nil {
		return err
	}
	defer syscall.Close(fd)
	timeout := syscall.NsecToTimeval((100 * time.Millisecond).Nanoseconds())
	if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &timeout); err != nil {
		return err
	}
	self, err := observeProcess(os.Getpid())
	if err != nil {
		return err
	}
	if err := os.WriteFile(ready, []byte("ready\n"), 0666); err != nil {
		return err
	}
	observation := dnsObservation{Capabilities: self.Capabilities}
	packet := make([]byte, 65535)
	deadline := time.Now().Add(30 * time.Second)
	stopping := false
	controlPhase := true
	controlStopRequested := false
	for {
		if !stopping {
			if _, err := os.Stat(stop); err == nil {
				stopping = true
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		if controlPhase && !controlStopRequested {
			if _, err := os.Stat(controlStop); err == nil {
				controlStopRequested = true
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		if time.Now().After(deadline) {
			return errors.New("DNS observer timed out")
		}
		n, _, err := syscall.Recvfrom(fd, packet, 0)
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK || err == syscall.EINTR {
				if controlStopRequested && err != syscall.EINTR {
					controlPhase = false
					controlStopRequested = false
					if err := os.WriteFile(controlStopped, []byte("stopped\n"), 0666); err != nil {
						return err
					}
				}
				if stopping && err != syscall.EINTR {
					break
				}
				continue
			}
			return err
		}
		if isAmbiguousPacket(packet[:n]) {
			observation.AmbiguousPackets++
		} else if isObserverControl(packet[:n], controlPhase) {
			observation.ControlPackets++
		} else if isDNSPacket(packet[:n]) {
			observation.Packets++
		}
	}
	encoded, err := json.MarshalIndent(observation, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(result, append(encoded, '\n'), 0666)
}

func closeDNSObserverControl(syncRoot string) error {
	if err := os.WriteFile(filepath.Join(syncRoot, "observer-control-stop"), []byte("stop\n"), 0600); err != nil {
		return err
	}
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(syncRoot, "observer-control-stopped")); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		time.Sleep(25 * time.Millisecond)
	}
	return errors.New("DNS observer control-phase close timed out")
}

func readDNSObservation(syncRoot string) (dnsObservation, error) {
	path := filepath.Join(syncRoot, "dns-observation.json")
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		encoded, err := os.ReadFile(path)
		if err == nil {
			var result dnsObservation
			if err := json.Unmarshal(encoded, &result); err != nil {
				return result, err
			}
			return result, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return dnsObservation{}, err
		}
		time.Sleep(25 * time.Millisecond)
	}
	return dnsObservation{}, errors.New("DNS observer result timed out")
}

func waitDNSObserver(syncRoot string) error {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(syncRoot, "observer-ready")); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		time.Sleep(25 * time.Millisecond)
	}
	return errors.New("DNS observer readiness timed out")
}

func stopDNSObserver(syncRoot string) (dnsObservation, error) {
	if err := os.WriteFile(filepath.Join(syncRoot, "observer-stop"), []byte("stop\n"), 0600); err != nil {
		return dnsObservation{}, err
	}
	return readDNSObservation(syncRoot)
}

func sendDNSObserverControl() error {
	for _, target := range []struct {
		network string
		ip      string
	}{{"udp4", "127.0.0.1"}, {"udp6", "::1"}} {
		connection, err := net.DialUDP(target.network, nil,
			&net.UDPAddr{IP: net.ParseIP(target.ip), Port: 53})
		if err != nil {
			return err
		}
		if _, err = connection.Write(observerControlPayload); err != nil {
			_ = connection.Close()
			return err
		}
		_ = connection.Close()
	}
	return sendTCPObserverControl()
}

func sendTCPObserverControl() error {
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 53})
	if err != nil {
		return err
	}
	accepted := make(chan error, 1)
	go func() {
		connection, err := listener.AcceptTCP()
		if err == nil {
			_, err = connection.Write(observerControlPayload)
			_ = connection.Close()
		}
		accepted <- err
	}()
	dialer := net.Dialer{LocalAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 53053}}
	connection, err := dialer.Dial("tcp4", "127.0.0.1:53")
	if err == nil {
		_ = connection.Close()
	}
	_ = listener.Close()
	acceptErr := <-accepted
	return errors.Join(err, acceptErr)
}

func networkShort(value uint16) uint16 {
	return value<<8 | value>>8
}
