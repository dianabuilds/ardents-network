package nameresolution

import (
	"bytes"
	"testing"
	"time"
)

func TestMessageCodecRejectsNonCanonicalMutations(t *testing.T) {
	t.Parallel()
	request := resolutionRequest{network: [32]byte{1}, nonce: [32]byte{2},
		deadline: time.Unix(1_800_000_000, 0).Add(time.Second).UnixNano(), name: "alice"}
	raw, err := encodeRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeRequest(raw)
	if err != nil || decoded != request {
		t.Fatalf("request round trip=%+v err=%v", decoded, err)
	}
	mutations := [][]byte{
		append(append([]byte(nil), raw...), 0),
		append([]byte(nil), raw[:len(raw)-1]...),
		append([]byte(nil), raw...),
	}
	mutations[2][2] = 99
	for _, changed := range mutations {
		if _, err := decodeRequest(changed); err == nil {
			t.Fatal("mutated request was accepted")
		}
	}
	fixed, err := padMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	fixed[len(fixed)-1] = 1
	if _, err := unpadMessage(fixed); err == nil {
		t.Fatal("non-canonical padding was accepted")
	}
}

func TestGatewayReplaySetIsFiniteAndRejectsDuplicates(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0)
	gateway := &gateway{config: GatewayConfig{MaximumPending: 1}, seen: make(map[[32]byte]int64)}
	first, second := [32]byte{1}, [32]byte{2}
	deadline := now.Add(time.Second).UnixNano()
	if !gateway.acceptNonce(first, deadline, now) {
		t.Fatal("first nonce was rejected")
	}
	if gateway.acceptNonce(first, deadline, now) {
		t.Fatal("duplicate nonce was accepted")
	}
	if gateway.acceptNonce(second, deadline, now) {
		t.Fatal("bounded replay set admitted excess work")
	}
	if !gateway.acceptNonce(second, now.Add(2*time.Second).UnixNano(), now.Add(time.Second)) {
		t.Fatal("expired replay state did not release capacity")
	}
}

func TestFixedEnvelopeHasOneShape(t *testing.T) {
	t.Parallel()
	a, err := padMessage([]byte("a"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := padMessage(bytes.Repeat([]byte{'b'}, 100))
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != fixedMessageSize || len(b) != fixedMessageSize {
		t.Fatalf("fixed sizes=%d/%d", len(a), len(b))
	}
}

func TestResponseCodecBindsNameAndRecordVersion(t *testing.T) {
	t.Parallel()
	response := resolutionResponse{network: [32]byte{1}, nonce: [32]byte{2}, deadline: 10,
		name: "alice", generation: 4, revision: 7, result: resultResolved, chain: [][]byte{{1}}}
	raw, err := encodeResponse(response)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeResponse(raw)
	if err != nil || decoded.name != response.name || decoded.generation != response.generation ||
		decoded.revision != response.revision || !bytes.Equal(decoded.chain[0], response.chain[0]) {
		t.Fatalf("response round trip=%+v err=%v", decoded, err)
	}
	response.result, response.chain = resultUnavailable, nil
	if _, err := encodeResponse(response); err == nil {
		t.Fatal("unavailable response carried a Record version")
	}
}
