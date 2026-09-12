//go:build linux

package node

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func TestClosedResolutionPublishesAndLooksUpThroughAdmittedNodeCarrier(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			fixture := newResolutionNetworkFixture(t, carrier)
			issue := func(introduction reachability.PrivateIntroduction) []byte {
				t.Helper()
				raw, _, err := reachability.IssuePrivate(reachability.PrivateIssueInput{Current: fixture.current, ProfileDigest: fixture.profile.Digest, Introduction: introduction, InstanceSigner: fixture.signer})
				if err != nil {
					t.Fatal(err)
				}
				return raw
			}
			nonce := [32]byte{91}
			lookup, err := route.EncodeClosedDescriptorLookup(nonce, fixture.current.Credential.Target)
			if err != nil {
				t.Fatal(err)
			}
			publish := func(raw []byte) []byte {
				t.Helper()
				operation, err := route.EncodeClosedDescriptorPublication(nonce, raw)
				if err != nil {
					t.Fatal(err)
				}
				return operation
			}
			check := func(index int, operation []byte, wantStatus uint8, wantProof []byte) {
				t.Helper()
				status, proof, err := fixture.exchange(t.Context(), fixture.tokens[index], nonce, operation)
				if err != nil || status != wantStatus || !bytes.Equal(proof, wantProof) {
					t.Fatalf("operation %d: status=%d, proof=%d, err=%v; want status=%d proof=%d", index, status, len(proof), err, wantStatus, len(wantProof))
				}
			}
			first := issue(fixture.introduction)
			check(0, publish(first), 0, nil)
			check(1, lookup, 0, first)
			if _, _, err := fixture.exchange(t.Context(), fixture.tokens[1], nonce, lookup); err == nil {
				t.Fatal("spent token admitted on fresh TLS channel")
			}
			conflict := fixture.introduction
			conflict.Slot[1] = 1
			check(2, publish(issue(conflict)), 3, nil)
			check(3, lookup, 3, nil)
			successor := fixture.introduction
			successor.Revision++
			successor.Slot[1] = 2
			second := issue(successor)
			check(4, publish(second), 0, nil)
			check(5, lookup, 0, second)
			check(6, publish(first), 3, nil)
			invalid := bytes.Clone(second)
			invalid[len(invalid)-1] ^= 1
			check(7, publish(invalid), 1, nil)
			check(8, lookup, 0, second)
			if _, err := reachability.VerifyPrivate(second, fixture.current.Credential.Target, fixture.profile.NetworkID, fixture.profile.Digest, time.Now()); err != nil {
				t.Fatal(err)
			}
			badToken := bytes.Clone(fixture.tokens[9])
			badToken[len(badToken)-1] ^= 1
			if _, _, err := fixture.exchange(t.Context(), badToken, nonce, lookup); err == nil {
				t.Fatal("invalid token admitted")
			}
			check(9, lookup, 0, second)
			foreign := successor
			foreign.NodeID = [32]byte{99}
			foreign.Revision++
			check(10, publish(issue(foreign)), 1, nil)
			check(11, lookup, 0, second)
		})
	}
}

func (fixture *resolutionNetworkFixture) exchange(ctx context.Context, token []byte, nonce [32]byte, operation []byte) (uint8, []byte, error) {
	inner, closeCarrier, err := fixture.openTerminal(ctx, token, 1)
	if err != nil {
		return 0, nil, err
	}
	defer closeCarrier()
	if err := route.WriteClosedLaneFrame(inner, route.ClosedLaneFrame{Kind: 10, Body: operation}); err != nil {
		return 0, nil, err
	}
	result, err := route.ReadClosedLaneFrame(inner)
	if err != nil || result.Kind != 11 || result.Lane != 0 {
		return 0, nil, fmt.Errorf("Descriptor result: kind=%d error=%v", result.Kind, err)
	}
	status, proof, err := route.DecodeClosedDescriptorResult(result.Body, nonce)
	if err != nil {
		return 0, nil, err
	}
	var trailing [1]byte
	if n, err := inner.Read(trailing[:]); n != 0 || err != io.EOF {
		return 0, nil, fmt.Errorf("terminal close: n=%d error=%v", n, err)
	}
	return status, proof, nil
}
