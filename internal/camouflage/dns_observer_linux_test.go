//go:build linux

package camouflage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

type dnsObservation struct {
	Packets, Controls, Ambiguous                      int64
	IPv4UDPControls, IPv6UDPControls, IPv4TCPControls int64
	BoundaryControls                                  map[string]dnsControlObservation
}

type dnsControlObservation struct {
	IPv4UDP, IPv6UDP, IPv4TCP int64
	IfIndex                   int
	Token                     string
}

type dnsControlTarget struct {
	Name    string `json:"name"`
	IfIndex int    `json:"ifindex"`
	Token   string `json:"token"`
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
	if wrapped := os.Getenv("ARDENTS_DNS_WRAPPED"); code == 0 && wrapped != "" {
		for _, executable := range filepath.SplitList(wrapped) {
			arguments := []string{"-test.count=1", "-test.v"}
			if run := os.Getenv("ARDENTS_DNS_WRAPPED_RUN"); run != "" {
				arguments = append(arguments, "-test.run", run)
			}
			command := exec.Command(executable, arguments...)
			command.Stdout, command.Stderr = os.Stdout, os.Stderr
			if err := command.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "wrapped DNS-proof test failed: %v\n", err)
				code = 1
				break
			}
		}
	}
	observation, err := stopDNSProof(root)
	if err != nil || !completeDNSControls(observation) || observation.Packets != 0 || observation.Ambiguous != 0 {
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
		return fmt.Errorf("prepare observer evidence root: %w", err)
	}
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(networkShort(0x0003)))
	if err != nil {
		return fmt.Errorf("open AF_PACKET observer: %w", err)
	}
	defer syscall.Close(fd)
	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, 4<<20); err != nil {
		return fmt.Errorf("bound observer receive buffer: %w", err)
	}
	timeout := syscall.NsecToTimeval((100 * time.Millisecond).Nanoseconds())
	if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &timeout); err != nil {
		return fmt.Errorf("bound observer receive timeout: %w", err)
	}
	if err := os.WriteFile(filepath.Join(root, "ready"), []byte("ready\n"), 0o666); err != nil {
		return fmt.Errorf("publish observer readiness: %w", err)
	}
	observation := dnsObservation{BoundaryControls: make(map[string]dnsControlObservation)}
	var paths pathObserver
	packet := make([]byte, 65535)
	control := true
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		if err := paths.poll(root); err != nil {
			return err
		}
		if exists(filepath.Join(root, "stop")) {
			raw, _ := json.Marshal(observation)
			return os.WriteFile(filepath.Join(root, "result.json"), raw, 0o666)
		}
		n, source, receiveErr := syscall.Recvfrom(fd, packet, 0)
		if receiveErr == syscall.EAGAIN || receiveErr == syscall.EWOULDBLOCK || receiveErr == syscall.EINTR {
			if control && receiveErr != syscall.EINTR && exists(filepath.Join(root, "control-done")) {
				if err := os.WriteFile(filepath.Join(root, "control-stopped"), []byte("stopped\n"), 0o666); err != nil {
					return err
				}
			}
			if control && receiveErr != syscall.EINTR && exists(filepath.Join(root, "controls-complete")) {
				control = false
				if err := os.WriteFile(filepath.Join(root, "controls-stopped"), []byte("stopped\n"), 0o666); err != nil {
					return err
				}
			}
			continue
		}
		if receiveErr != nil {
			return receiveErr
		}
		paths.observe(packet[:n])
		if _, _, ambiguous, ok := packetTransport(packet[:n]); ok && ambiguous {
			observation.Ambiguous++
		} else if class, name, token := dnsControlClass(packet[:n], control); class != 0 {
			if class == 4 {
				continue
			}
			target := dnsControlTarget{Name: name, IfIndex: socketIfIndex(source), Token: token}
			if !recordDNSControl(&observation, target, class) {
				observation.Ambiguous++
			}
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
	loopback, err := net.InterfaceByName("lo")
	if err != nil {
		return err
	}
	token, err := newDNSControlToken()
	if err != nil {
		return err
	}
	return runDNSControlPlan(root, []dnsControlTarget{{Name: "observer-self-test", IfIndex: loopback.Index,
		Token: token}})
}

func runDNSControlPlan(root string, boundaries []dnsControlTarget) error {
	for _, boundary := range boundaries {
		if boundary.Name == "" || len(boundary.Name) > 128 || boundary.IfIndex <= 0 || len(boundary.Token) != 32 {
			return errors.New("DNS control boundary is invalid")
		}
		if err := sendDNSControlFlows(encodeDNSControlPayload(boundary.Name, boundary.Token)); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(root, "control-done"), []byte("done\n"), 0o600); err != nil {
			return err
		}
		if err := waitForFile(filepath.Join(root, "control-stopped"), 2*time.Second); err != nil {
			return err
		}
		for _, name := range []string{"control-done", "control-stopped"} {
			if err := os.Remove(filepath.Join(root, name)); err != nil {
				return err
			}
		}
	}
	if err := os.WriteFile(filepath.Join(root, "controls-complete"), []byte("done\n"), 0o600); err != nil {
		return err
	}
	return waitForFile(filepath.Join(root, "controls-stopped"), 2*time.Second)
}

func sendDNSControlFlows(payload []byte) error {
	for _, target := range []struct{ network, address string }{
		{"udp4", "127.0.0.1:53"}, {"udp6", "[::1]:53"},
	} {
		connection, err := net.Dial(target.network, target.address)
		if err != nil {
			return err
		}
		_, err = connection.Write(payload)
		_ = connection.Close()
		if err != nil {
			return err
		}
	}
	if err := sendTCPDNSControl(payload); err != nil {
		return err
	}
	return nil
}

func sendTCPDNSControl(payload []byte) error {
	listener, err := net.Listen("tcp4", "127.0.0.1:53")
	if err != nil {
		return err
	}
	accepted := make(chan error, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			_, acceptErr = connection.Write(payload)
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

func completeDNSControls(value dnsObservation) bool {
	control, ok := value.BoundaryControls["observer-self-test"]
	return ok && len(value.BoundaryControls) == 1 && value.Controls >= 6 &&
		value.IPv4UDPControls >= 2 && value.IPv6UDPControls >= 2 && value.IPv4TCPControls >= 2 &&
		control.IfIndex > 0 && len(control.Token) == 32 && control.IPv4UDP >= 2 &&
		control.IPv6UDP >= 2 && control.IPv4TCP >= 2
}

func recordDNSControl(value *dnsObservation, target dnsControlTarget, class byte) bool {
	counts := value.BoundaryControls[target.Name]
	if target.Name == "" || target.IfIndex <= 0 || len(target.Token) != 32 ||
		counts.Token != "" && (counts.Token != target.Token || counts.IfIndex != target.IfIndex) {
		return false
	}
	value.Controls++
	counts.IfIndex = target.IfIndex
	counts.Token = target.Token
	switch class {
	case 1:
		value.IPv4UDPControls++
		counts.IPv4UDP++
	case 2:
		value.IPv6UDPControls++
		counts.IPv6UDP++
	case 3:
		value.IPv4TCPControls++
		counts.IPv4TCP++
	}
	value.BoundaryControls[target.Name] = counts
	return true
}

func newDNSControlToken() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func socketIfIndex(value syscall.Sockaddr) int {
	if link, ok := value.(*syscall.SockaddrLinklayer); ok {
		return link.Ifindex
	}
	return 0
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
