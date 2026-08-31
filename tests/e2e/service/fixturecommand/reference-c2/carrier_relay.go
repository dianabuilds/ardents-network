//go:build referencec2

package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

const maximumCarrierRelayBridges = 8

type carrierRelaySnapshot struct {
	AcceptedBridges        uint64 `json:"accepted_bridges"`
	ActiveBridges          uint32 `json:"active_bridges"`
	PeakActiveBridges      uint32 `json:"peak_active_bridges"`
	ResetCount             uint32 `json:"reset_count"`
	ResetBridges           uint32 `json:"reset_bridges"`
	ActiveBefore           uint32 `json:"active_before"`
	SelectedBridgeID       uint64 `json:"selected_bridge_id"`
	AcceptedAfterReset     uint64 `json:"accepted_after_reset"`
	ClientToNodeBytes      uint64 `json:"client_to_node_bytes"`
	NodeToClientBytes      uint64 `json:"node_to_client_bytes"`
	ListenerLiveAfterReset bool   `json:"listener_live_after_reset"`
}

type carrierRelayResetReceipt struct {
	Schema           string `json:"schema"`
	ResetCount       uint32 `json:"reset_count"`
	ResetBridges     uint32 `json:"reset_bridges"`
	ActiveBefore     uint32 `json:"active_before"`
	SelectedBridgeID uint64 `json:"selected_bridge_id"`
	ActiveBridges    uint32 `json:"active_bridges"`
	ListenerLive     bool   `json:"listener_live"`
}

type carrierRelay struct {
	listener net.Listener
	target   string
	slots    chan struct{}

	mu                  sync.Mutex
	bridges             map[uint64]carrierRelayBridge
	nextBridge          uint64
	accepted            uint64
	peak                uint32
	resetCount          uint32
	resetBridges        uint32
	activeBefore        uint32
	selectedBridgeID    uint64
	acceptedAtLastReset uint64
	clientToNodeBytes   uint64
	nodeToClientBytes   uint64
	listenerClosed      bool
	listenerLiveAtReset bool
	acceptDone          chan struct{}
	acceptFailure       chan error
	work                sync.WaitGroup
	closeOnce           sync.Once
}

type carrierRelayBridge struct {
	client, node net.Conn
	done         chan struct{}
}

func startCarrierRelay(listen, target string) (*carrierRelay, error) {
	if !literalCarrierRelayEndpoint(listen, true) || !literalCarrierRelayEndpoint(target, false) {
		return nil, errors.New("carrier relay endpoints are invalid")
	}
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return nil, err
	}
	relay := &carrierRelay{listener: listener, target: target, slots: make(chan struct{}, maximumCarrierRelayBridges),
		bridges: make(map[uint64]carrierRelayBridge), acceptDone: make(chan struct{}), acceptFailure: make(chan error, 1)}
	go relay.accept()
	return relay, nil
}

func literalCarrierRelayEndpoint(value string, zeroPort bool) bool {
	host, port, err := net.SplitHostPort(value)
	number, portErr := strconv.Atoi(port)
	return err == nil && net.ParseIP(host) != nil && portErr == nil && number >= 0 && number <= 65535 && (zeroPort || number != 0)
}

func (relay *carrierRelay) endpoint() string { return relay.listener.Addr().String() }

func (relay *carrierRelay) accept() {
	defer close(relay.acceptDone)
	for {
		client, err := relay.listener.Accept()
		if err != nil {
			relay.mu.Lock()
			expected := relay.listenerClosed
			relay.mu.Unlock()
			if !expected {
				select {
				case relay.acceptFailure <- fmt.Errorf("carrier relay accept failed: %w", err):
				default:
				}
			}
			return
		}
		select {
		case relay.slots <- struct{}{}:
			relay.work.Add(1)
			go relay.bridge(client)
		default:
			resetCarrierRelayConnection(client)
		}
	}
}

func (relay *carrierRelay) bridge(client net.Conn) {
	defer relay.work.Done()
	defer func() { <-relay.slots }()
	node, err := net.DialTimeout("tcp", relay.target, 3*time.Second)
	if err != nil {
		resetCarrierRelayConnection(client)
		return
	}
	id := relay.register(client, node)
	defer relay.unregister(id, client, node)

	var directions sync.WaitGroup
	directions.Add(2)
	go func() {
		defer directions.Done()
		relay.copy(node, client, true)
		resetCarrierRelayConnection(node)
	}()
	go func() {
		defer directions.Done()
		relay.copy(client, node, false)
		resetCarrierRelayConnection(client)
	}()
	directions.Wait()
}

func (relay *carrierRelay) register(client, node net.Conn) uint64 {
	relay.mu.Lock()
	defer relay.mu.Unlock()
	relay.nextBridge++
	id := relay.nextBridge
	relay.bridges[id] = carrierRelayBridge{client: client, node: node, done: make(chan struct{})}
	relay.accepted++
	if active := uint32(len(relay.bridges)); active > relay.peak {
		relay.peak = active
	}
	return id
}

func (relay *carrierRelay) unregister(id uint64, client, node net.Conn) {
	resetCarrierRelayConnection(client)
	resetCarrierRelayConnection(node)
	relay.mu.Lock()
	if bridge, exists := relay.bridges[id]; exists {
		delete(relay.bridges, id)
		close(bridge.done)
	}
	relay.mu.Unlock()
}

func (relay *carrierRelay) copy(destination, source net.Conn, clientToNode bool) {
	buffer := make([]byte, 32<<10)
	for {
		count, readErr := source.Read(buffer)
		if count > 0 {
			written, writeErr := carrierRelayWriteAll(destination, buffer[:count])
			relay.recordBytes(clientToNode, uint64(written))
			if writeErr != nil {
				return
			}
		}
		if readErr != nil {
			return
		}
	}
}

func carrierRelayWriteAll(destination net.Conn, value []byte) (int, error) {
	total := 0
	for len(value) > 0 {
		count, err := destination.Write(value)
		total += count
		if err != nil {
			return total, err
		}
		if count <= 0 || count > len(value) {
			return total, io.ErrShortWrite
		}
		value = value[count:]
	}
	return total, nil
}

func (relay *carrierRelay) recordBytes(clientToNode bool, count uint64) {
	relay.mu.Lock()
	if clientToNode {
		relay.clientToNodeBytes += count
	} else {
		relay.nodeToClientBytes += count
	}
	relay.mu.Unlock()
}

func (relay *carrierRelay) reset() (carrierRelayResetReceipt, error) {
	relay.mu.Lock()
	if relay.resetCount != 0 {
		relay.mu.Unlock()
		return carrierRelayResetReceipt{}, errors.New("carrier relay reset was already applied")
	}
	if len(relay.bridges) != 2 {
		relay.mu.Unlock()
		return carrierRelayResetReceipt{}, errors.New("carrier relay reset requires exactly two active bridges")
	}
	connections := make([]carrierRelayBridge, 0, len(relay.bridges))
	for _, bridge := range relay.bridges {
		connections = append(connections, bridge)
	}
	relay.resetCount++
	relay.resetBridges = uint32(len(connections))
	relay.activeBefore = uint32(len(relay.bridges))
	relay.selectedBridgeID = 0
	relay.acceptedAtLastReset = relay.accepted
	relay.mu.Unlock()
	for _, bridge := range connections {
		resetCarrierRelayConnection(bridge.client)
		resetCarrierRelayConnection(bridge.node)
	}
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for _, bridge := range connections {
		select {
		case <-bridge.done:
		case <-timer.C:
			return carrierRelayResetReceipt{}, errors.New("carrier relay reset retained an active bridge")
		}
	}
	select {
	case acceptErr := <-relay.acceptFailure:
		return carrierRelayResetReceipt{}, acceptErr
	case <-relay.acceptDone:
		return carrierRelayResetReceipt{}, errors.New("carrier relay listener stopped during reset")
	default:
	}
	relay.mu.Lock()
	relay.listenerLiveAtReset = !relay.listenerClosed
	relay.mu.Unlock()
	snapshot := relay.snapshot()
	if snapshot.ActiveBridges != 0 || !snapshot.ListenerLiveAfterReset {
		return carrierRelayResetReceipt{}, errors.New("carrier relay retained a connection after reset")
	}
	return carrierRelayResetReceipt{Schema: "ardents-h4-8-a11-carrier-relay-reset-v1", ResetCount: snapshot.ResetCount,
		ResetBridges: snapshot.ResetBridges, ActiveBefore: snapshot.ActiveBefore, SelectedBridgeID: snapshot.SelectedBridgeID,
		ActiveBridges: snapshot.ActiveBridges, ListenerLive: snapshot.ListenerLiveAfterReset}, nil
}

func (relay *carrierRelay) snapshot() carrierRelaySnapshot {
	relay.mu.Lock()
	defer relay.mu.Unlock()
	after := uint64(0)
	if relay.resetCount != 0 {
		after = relay.accepted - relay.acceptedAtLastReset
	}
	return carrierRelaySnapshot{AcceptedBridges: relay.accepted, ActiveBridges: uint32(len(relay.bridges)), PeakActiveBridges: relay.peak,
		ResetCount: relay.resetCount, ResetBridges: relay.resetBridges, ActiveBefore: relay.activeBefore,
		SelectedBridgeID: relay.selectedBridgeID, AcceptedAfterReset: after,
		ClientToNodeBytes: relay.clientToNodeBytes, NodeToClientBytes: relay.nodeToClientBytes,
		ListenerLiveAfterReset: relay.listenerLiveAtReset}
}

func (relay *carrierRelay) close() {
	relay.closeOnce.Do(func() {
		relay.mu.Lock()
		relay.listenerClosed = true
		connections := make([]carrierRelayBridge, 0, len(relay.bridges))
		for _, bridge := range relay.bridges {
			connections = append(connections, bridge)
		}
		relay.mu.Unlock()
		_ = relay.listener.Close()
		<-relay.acceptDone
		for _, bridge := range connections {
			resetCarrierRelayConnection(bridge.client)
			resetCarrierRelayConnection(bridge.node)
		}
		relay.work.Wait()
	})
}

func resetCarrierRelayConnection(connection net.Conn) {
	if tcp, ok := connection.(*net.TCPConn); ok {
		_ = tcp.SetLinger(0)
	}
	_ = connection.Close()
}
