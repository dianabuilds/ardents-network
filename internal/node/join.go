package node

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	nodehosting "github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/node/join"
)

// ClosedDataJoinProfile retains the Node configuration name while JOIN owns
// its listener, spend ledger, pair set and Hosting handle.
type ClosedDataJoinProfile = join.Profile

func validateClosedDataJoinProfile(local ClosedDataJoinProfile, source authority.Source, snapshot state.NodeDuty, now time.Time) error {
	return join.Validate(local, source, snapshot, now, literalNodeEndpoint(snapshot.ProbeEndpoint))
}

func startClosedDataJoin(local ClosedDataJoinProfile, inputs roleInputs, snapshot state.NodeDuty) (*dutyHandle, error) {
	if err := validateClosedDataJoinProfile(local, inputs.authority, snapshot, inputs.now()); err != nil {
		return nil, err
	}
	listen, err := closedListenAddress(snapshot.ProbeEndpoint, inputs.listenOverride)
	if err != nil {
		return nil, err
	}
	role, err := join.Start(join.Config{Profile: local, Snapshot: snapshot,
		Authority: inputs.authority, CurrentDuty: inputs.currentDuty,
		Now: inputs.now, ListenAddress: listen, OpenHost: func(root string) (join.Host, error) {
			host, err := nodehosting.Open(root)
			if err != nil {
				return nil, err
			}
			return nodehosting.NewJoinHandle(host, inputs.authority, inputs.now), nil
		}})
	if err != nil {
		return nil, err
	}
	return &dutyHandle{Done: role.Done, Joined: role.Joined, Protect: func(bool) {}, Usage: role.Usage, Stop: role.Stop, Drain: role.Drain}, nil
}
