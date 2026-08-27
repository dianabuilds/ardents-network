//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

type faultRelayResult struct {
	Schema          string `json:"schema"`
	Event           string `json:"event"`
	Case            string `json:"case"`
	Passed          bool   `json:"passed,omitempty"`
	Frontend        string `json:"frontend,omitempty"`
	Control         string `json:"control,omitempty"`
	Upstream        string `json:"upstream,omitempty"`
	OldUpstream     string `json:"old_upstream_local,omitempty"`
	NewUpstream     string `json:"new_upstream_local,omitempty"`
	ExpectedRebind  bool   `json:"expected_rebind,omitempty"`
	Rebound         bool   `json:"rebound,omitempty"`
	PacketsBefore   int64  `json:"packets_before,omitempty"`
	PacketsAfter    int64  `json:"packets_after,omitempty"`
	GoroutinesStart int    `json:"goroutines_start,omitempty"`
	GoroutinesEnd   int    `json:"goroutines_end,omitempty"`
	FDsStart        int    `json:"fds_start,omitempty"`
	FDsEnd          int    `json:"fds_end,omitempty"`
	CleanupJoined   bool   `json:"cleanup_joined,omitempty"`
	ElapsedMS       int64  `json:"elapsed_ms,omitempty"`
	Note            string `json:"note,omitempty"`
}

type faultUDPRelay struct {
	ctx             context.Context
	cancel          context.CancelFunc
	frontend        *net.UDPConn
	control         *net.UDPConn
	upstreamAddress *net.UDPAddr

	mu            sync.Mutex
	upstream      *net.UDPConn
	generation    uint64
	client        *net.UDPAddr
	rebound       bool
	oldLocal      string
	newLocal      string
	packetsBefore int64
	packetsAfter  int64

	wg     sync.WaitGroup
	errors chan error
}

func runFaultRelay(arguments []string) error {
	set := flag.NewFlagSet("fault-relay", flag.ContinueOnError)
	listen := set.String("listen", "", "client-facing UDP listen address")
	control := set.String("control", "", "lab control UDP listen address")
	upstream := set.String("upstream", "", "server UDP address")
	caseName := set.String("case", "", "stable case identifier")
	token := set.String("token", "", "shared lab control token")
	deadlineUnix := set.Int64("deadline-unix", 0, "absolute deadline in Unix seconds")
	expectRebind := set.Bool("expect-rebind", false, "require one observed upstream port change")
	if err := set.Parse(arguments); err != nil {
		return err
	}
	if *listen == "" || *control == "" || *upstream == "" || *caseName == "" || *token == "" ||
		*deadlineUnix <= time.Now().Unix() {
		return errors.New("fault relay arguments are incomplete")
	}
	deadline := time.Unix(*deadlineUnix, 0).UTC()
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	started := time.Now()
	startGoroutines, startFDs := runtime.NumGoroutine(), faultFDCount()
	relay, err := newFaultUDPRelay(ctx, *listen, *control, *upstream)
	if err != nil {
		return err
	}
	faultEncodeRelay(faultRelayResult{Schema: "ardents-r094-rebind-relay-v1", Event: "ready", Case: *caseName,
		Frontend: relay.frontend.LocalAddr().String(), Control: relay.control.LocalAddr().String(),
		Upstream: relay.upstreamAddress.String(), OldUpstream: relay.oldLocal, ExpectedRebind: *expectRebind})
	relayErr := relay.runControl(*token)
	relay.close()
	endGoroutines, endFDs, cleanup := faultWaitForCleanup(startGoroutines, startFDs)
	relay.mu.Lock()
	rebound, oldLocal, newLocal := relay.rebound, relay.oldLocal, relay.newLocal
	before, after := relay.packetsBefore, relay.packetsAfter
	relay.mu.Unlock()
	passed := relayErr == nil && cleanup && before > 0
	if *expectRebind {
		passed = passed && rebound && oldLocal != "" && newLocal != "" && oldLocal != newLocal && after > 0
	} else {
		passed = passed && !rebound && after == 0
	}
	faultEncodeRelay(faultRelayResult{Schema: "ardents-r094-rebind-relay-v1", Event: "outcome", Case: *caseName,
		Passed: passed, Frontend: relay.frontend.LocalAddr().String(), Control: relay.control.LocalAddr().String(),
		Upstream: relay.upstreamAddress.String(), OldUpstream: oldLocal, NewUpstream: newLocal,
		ExpectedRebind: *expectRebind, Rebound: rebound, PacketsBefore: before, PacketsAfter: after,
		GoroutinesStart: startGoroutines, GoroutinesEnd: endGoroutines, FDsStart: startFDs, FDsEnd: endFDs,
		CleanupJoined: cleanup, ElapsedMS: time.Since(started).Milliseconds(), Note: faultNote(relayErr)})
	if !passed {
		return errors.New("fault relay result did not match its oracle")
	}
	return nil
}

func newFaultUDPRelay(parent context.Context, listen, control, upstream string) (*faultUDPRelay, error) {
	frontAddress, err := net.ResolveUDPAddr("udp", listen)
	if err != nil {
		return nil, err
	}
	controlAddress, err := net.ResolveUDPAddr("udp", control)
	if err != nil {
		return nil, err
	}
	upstreamAddress, err := net.ResolveUDPAddr("udp", upstream)
	if err != nil {
		return nil, err
	}
	frontend, err := net.ListenUDP("udp", frontAddress)
	if err != nil {
		return nil, err
	}
	controlSocket, err := net.ListenUDP("udp", controlAddress)
	if err != nil {
		_ = frontend.Close()
		return nil, err
	}
	upstreamSocket, err := net.DialUDP("udp", nil, upstreamAddress)
	if err != nil {
		_ = frontend.Close()
		_ = controlSocket.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	relay := &faultUDPRelay{ctx: ctx, cancel: cancel, frontend: frontend, control: controlSocket,
		upstreamAddress: upstreamAddress, upstream: upstreamSocket, generation: 1,
		oldLocal: upstreamSocket.LocalAddr().String(), errors: make(chan error, 4)}
	relay.wg.Add(2)
	go relay.frontendLoop()
	go relay.upstreamLoop(upstreamSocket, 1)
	return relay, nil
}

func (relay *faultUDPRelay) frontendLoop() {
	defer relay.wg.Done()
	buffer := make([]byte, 64<<10)
	for {
		_ = relay.frontend.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		count, client, err := relay.frontend.ReadFromUDP(buffer)
		if err != nil {
			if relay.retryRead(err) {
				continue
			}
			relay.report(err)
			return
		}
		relay.mu.Lock()
		relay.client = client
		upstream, after := relay.upstream, relay.rebound
		relay.mu.Unlock()
		if _, err := upstream.Write(buffer[:count]); err != nil {
			relay.report(err)
			return
		}
		relay.recordPacket(after)
	}
}

func (relay *faultUDPRelay) upstreamLoop(connection *net.UDPConn, generation uint64) {
	defer relay.wg.Done()
	buffer := make([]byte, 64<<10)
	for {
		_ = connection.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		count, err := connection.Read(buffer)
		if err != nil {
			relay.mu.Lock()
			current := relay.generation == generation
			relay.mu.Unlock()
			if !current || errors.Is(err, net.ErrClosed) {
				return
			}
			if relay.retryRead(err) {
				continue
			}
			relay.report(err)
			return
		}
		relay.mu.Lock()
		if relay.generation != generation {
			relay.mu.Unlock()
			continue
		}
		client, after := relay.client, relay.rebound
		relay.mu.Unlock()
		if client == nil {
			continue
		}
		if _, err := relay.frontend.WriteToUDP(buffer[:count], client); err != nil {
			relay.report(err)
			return
		}
		relay.recordPacket(after)
	}
}

func (relay *faultUDPRelay) runControl(token string) error {
	buffer := make([]byte, 1024)
	for {
		select {
		case err := <-relay.errors:
			return err
		default:
		}
		_ = relay.control.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		count, peer, err := relay.control.ReadFromUDP(buffer)
		if err != nil {
			if relay.retryRead(err) {
				continue
			}
			return err
		}
		parts := strings.SplitN(string(buffer[:count]), "|", 3)
		if len(parts) != 3 || parts[0] != faultRelayProtocol || parts[2] != token {
			_, _ = relay.control.WriteToUDP([]byte(faultRelayProtocol+"|error|unauthorized|"), peer)
			continue
		}
		switch parts[1] {
		case "rebind":
			oldLocal, newLocal, err := relay.rebind()
			if err != nil {
				return err
			}
			response := faultRelayProtocol + "|ok|rebind|" + oldLocal + "->" + newLocal
			if _, err := relay.control.WriteToUDP([]byte(response), peer); err != nil {
				return err
			}
		case "stop":
			if _, err := relay.control.WriteToUDP([]byte(faultRelayProtocol+"|ok|stop|joined"), peer); err != nil {
				return err
			}
			time.Sleep(200 * time.Millisecond)
			return nil
		default:
			_, _ = relay.control.WriteToUDP([]byte(faultRelayProtocol+"|error|action|"), peer)
		}
	}
}

func (relay *faultUDPRelay) rebind() (string, string, error) {
	connection, err := net.DialUDP("udp", nil, relay.upstreamAddress)
	if err != nil {
		return "", "", err
	}
	relay.mu.Lock()
	if relay.rebound {
		relay.mu.Unlock()
		_ = connection.Close()
		return "", "", errors.New("fault relay rebinding was requested twice")
	}
	old := relay.upstream
	oldLocal := old.LocalAddr().String()
	newLocal := connection.LocalAddr().String()
	if oldLocal == newLocal {
		relay.mu.Unlock()
		_ = connection.Close()
		return "", "", errors.New("fault relay source port did not change")
	}
	relay.upstream = connection
	relay.generation++
	generation := relay.generation
	relay.rebound = true
	relay.newLocal = newLocal
	relay.mu.Unlock()
	relay.wg.Add(1)
	go relay.upstreamLoop(connection, generation)
	_ = old.Close()
	return oldLocal, newLocal, nil
}

func (relay *faultUDPRelay) recordPacket(after bool) {
	relay.mu.Lock()
	if after {
		relay.packetsAfter++
	} else {
		relay.packetsBefore++
	}
	relay.mu.Unlock()
}

func (relay *faultUDPRelay) retryRead(err error) bool {
	select {
	case <-relay.ctx.Done():
		return false
	default:
	}
	var networkError net.Error
	return errors.As(err, &networkError) && networkError.Timeout()
}

func (relay *faultUDPRelay) report(err error) {
	select {
	case <-relay.ctx.Done():
	case relay.errors <- err:
	default:
	}
}

func (relay *faultUDPRelay) close() {
	relay.cancel()
	_ = relay.frontend.Close()
	_ = relay.control.Close()
	relay.mu.Lock()
	upstream := relay.upstream
	relay.mu.Unlock()
	_ = upstream.Close()
	relay.wg.Wait()
}

func faultEncodeRelay(value faultRelayResult) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
}
