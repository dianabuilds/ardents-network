package route

import (
	"bytes"
	"crypto/sha256"
	"net"
	"testing"
)

func TestRelayOpaqueCountAndDigestDescribeRecordedDirection(t *testing.T) {
	upstream, upstreamPeer := relayTCPPair(t)
	downstream, downstreamPeer := relayTCPPair(t)
	defer upstreamPeer.Close()
	defer downstreamPeer.Close()
	type outcome struct {
		forward opaqueDirection
		reverse opaqueDirection
		err     error
	}
	finished := make(chan outcome, 1)
	go func() {
		forward, reverse, err := relayOpaque(upstream, downstream)
		finished <- outcome{forward, reverse, err}
	}()
	recorded := bytes.Repeat([]byte{17}, 1024)
	reverse := bytes.Repeat([]byte{29}, 2048)
	go writeRelayDirection(t, upstreamPeer, recorded)
	go writeRelayDirection(t, downstreamPeer, reverse)
	readRelayDirection(t, upstreamPeer, len(reverse))
	readRelayDirection(t, downstreamPeer, len(recorded))
	result := <-finished
	if result.err != nil || result.forward.count != uint64(len(recorded)) ||
		result.forward.digest != sha256.Sum256(recorded) || result.reverse.count != uint64(len(reverse)) ||
		result.reverse.digest != sha256.Sum256(reverse) {
		t.Fatalf("recorded relay observation is dishonest: %+v", result)
	}
}

func relayTCPPair(t *testing.T) (net.Conn, *net.TCPConn) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan net.Conn, 1)
	go func() { connection, _ := listener.Accept(); accepted <- connection }()
	peer, err := net.DialTCP("tcp", nil, listener.Addr().(*net.TCPAddr))
	if err != nil {
		t.Fatal(err)
	}
	connection := <-accepted
	_ = listener.Close()
	return connection, peer
}

func writeRelayDirection(t *testing.T, connection *net.TCPConn, payload []byte) {
	t.Helper()
	if count, err := connection.Write(payload); err != nil || count != len(payload) {
		t.Errorf("relay write=%d err=%v", count, err)
	}
	if err := connection.CloseWrite(); err != nil {
		t.Error(err)
	}
}

func readRelayDirection(t *testing.T, connection net.Conn, count int) {
	t.Helper()
	payload := make([]byte, count)
	if read, err := readFull(connection, payload); err != nil || read != count {
		t.Fatalf("relay read=%d err=%v", read, err)
	}
}

func readFull(input net.Conn, output []byte) (int, error) {
	total := 0
	for total < len(output) {
		count, err := input.Read(output[total:])
		total += count
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
