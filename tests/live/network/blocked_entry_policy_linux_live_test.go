//go:build linux && live

package network_test

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

type blockedTimeline struct {
	ManifestStartNS uint64 `json:"manifest_start_ns"`
	ConditionNS     uint64 `json:"condition_ns,omitempty"`
	TransitionNS    uint64 `json:"transition_ns,omitempty"`
}

func runBlockedPolicy(t *testing.T) {
	t.Helper()
	start := blockedMonotonicNS(t)
	writeBlockedJSON(t, filepath.Join(blockedSync(), "timeline-start.json"), blockedTimeline{ManifestStartNS: start})
	targets := os.Getenv("ARDENTS_BLOCKED_POLICY_TARGETS")
	if targets == "" {
		targets = "172.31.20.11"
	}
	for _, target := range strings.Split(targets, ",") {
		if err := installBlockedRoute(net.ParseIP(target)); err != nil {
			t.Fatal(err)
		}
	}
	writeBlockedSignal(t, filepath.Join(blockedSync(), "policy-ready"))
	fmt.Println(`{"kind":"policy","state":"READY"}`)
	signal := os.Getenv("ARDENTS_BLOCKED_POLICY_SIGNAL")
	if signal == "" {
		signal = "blocked-condition.json"
	}
	waitBlockedFile(t, filepath.Join(blockedSync(), signal), 70*time.Second)
}

func blockedMonotonicNS(t *testing.T) uint64 {
	t.Helper()
	var value unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &value); err != nil {
		t.Fatal(err)
	}
	return uint64(value.Sec)*uint64(time.Second) + uint64(value.Nsec)
}

func startBlockedTimeline(t *testing.T) blockedTimeline {
	t.Helper()
	timeline := blockedTimeline{ManifestStartNS: blockedMonotonicNS(t)}
	writeBlockedJSON(t, filepath.Join(blockedSync(), "timeline-start.json"), timeline)
	return timeline
}

func readBlockedTimeline(t *testing.T) blockedTimeline {
	t.Helper()
	var timeline blockedTimeline
	readBlockedJSON(t, filepath.Join(blockedSync(), "timeline-start.json"), &timeline)
	if timeline.ManifestStartNS == 0 {
		t.Fatal("blocked manifest monotonic start is absent")
	}
	return timeline
}

func stampBlockedTransition(t *testing.T, transition []byte, timeline blockedTimeline) []byte {
	t.Helper()
	timeline.TransitionNS = blockedMonotonicNS(t)
	if timeline.TransitionNS <= timeline.ManifestStartNS ||
		timeline.ConditionNS != 0 && (timeline.TransitionNS < timeline.ConditionNS ||
			timeline.TransitionNS-timeline.ConditionNS > uint64(2*time.Second)) {
		t.Fatalf("authenticated transition has an invalid monotonic offset: %+v", timeline)
	}
	offset := timeline.TransitionNS - timeline.ManifestStartNS
	position := len("ardents-h3-bridge-transition-v1") + 32 + 1 + 32
	if len(transition) < position+8 {
		t.Fatal("blocked transition frame is truncated")
	}
	binary.BigEndian.PutUint64(transition[position:position+8], offset)
	writeBlockedJSON(t, filepath.Join(blockedSync(), "timeline.json"), timeline)
	return transition
}

func installBlockedRoute(destination net.IP) error {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, syscall.NETLINK_ROUTE)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)
	if err := syscall.Bind(fd, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}); err != nil {
		return err
	}
	const headerSize = 16
	request := make([]byte, headerSize+12+8)
	order := binary.NativeEndian
	order.PutUint32(request[0:4], uint32(len(request)))
	order.PutUint16(request[4:6], syscall.RTM_NEWROUTE)
	order.PutUint16(request[6:8], syscall.NLM_F_REQUEST|syscall.NLM_F_ACK|syscall.NLM_F_CREATE|syscall.NLM_F_EXCL)
	order.PutUint32(request[8:12], 1)
	request[16] = syscall.AF_INET
	request[17] = 32
	request[20] = syscall.RT_TABLE_MAIN
	request[21] = syscall.RTPROT_BOOT
	request[22] = syscall.RT_SCOPE_UNIVERSE
	request[23] = syscall.RTN_BLACKHOLE
	order.PutUint16(request[28:30], 8)
	order.PutUint16(request[30:32], syscall.RTA_DST)
	copy(request[32:36], destination.To4())
	if err := syscall.Sendto(fd, request, 0, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}); err != nil {
		return err
	}
	response := make([]byte, os.Getpagesize())
	count, _, err := syscall.Recvfrom(fd, response, 0)
	if err != nil {
		return err
	}
	messages, err := syscall.ParseNetlinkMessage(response[:count])
	if err != nil || len(messages) != 1 || messages[0].Header.Type != syscall.NLMSG_ERROR || len(messages[0].Data) < 4 {
		return errors.New("blackhole route acknowledgement is invalid")
	}
	if errno := int32(order.Uint32(messages[0].Data[:4])); errno != 0 {
		return syscall.Errno(-errno)
	}
	return nil
}
