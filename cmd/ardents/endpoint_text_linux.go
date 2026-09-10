//go:build linux

package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
)

func runTextHeadlessRuntime(ctx context.Context, plan decodedHeadlessRuntimePlan, output io.Writer) (outcome error) {
	inherited, ok := output.(*os.File)
	if !ok {
		return errors.New("text runtime requires a pollable local event output")
	}
	opened, err := openHeadlessTextOutput(inherited)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, opened.Close()) }()
	clock := time.Now
	network, refresh, err := headlessNetworkConfig(plan, clock)
	if err != nil {
		return err
	}
	if plan.NetworkSourcePlan == "" {
		network.ClockObservationFile = plan.TimeConfidenceFile
		network.LocalRoleStateRoot = plan.LocalRoleStateRoot
	}
	files := func(plan headlessPermissionPlan) endpointapi.TextPermissionFiles {
		return endpointapi.TextPermissionFiles{RequestPath: plan.RequestPath, ResponsePath: plan.ResponsePath, Maxima: plan.Maxima}
	}
	return endpointapi.RunTextParticipant(ctx, endpointapi.TextParticipantConfig{Network: network, RefreshNetwork: refresh, EntryRoot: plan.EntryStateRoot, LocalRoleRoot: plan.LocalRoleStateRoot, TokenRoot: plan.TextTokenRoot, PublicationRoot: plan.PublicationRoot, ServiceInstanceRoot: plan.ServiceInstanceRoot, ApplicationAddress: plan.ApplicationSocket, AdministrationAddress: plan.AdministrationSocket, BrokerID: plan.BrokerID, ConnectionPrincipal: plan.ConnectionPrincipal, AdministrationPrincipal: plan.AdministrationPrincipal, ReaderPermission: files(plan.ReaderPermission), PublisherPermission: files(plan.PublisherPermission), Clock: clock, Observe: func(reportCtx context.Context, event endpointapi.TextParticipantEvent) error {
		return writeHeadlessTextEvent(reportCtx, opened, event)
	}})
}

// Reopen this exact descriptor; changing nonblocking flags on a dup would
// mutate the shell's inherited open-file description too. No arbitrary path.
func openHeadlessTextOutput(inherited *os.File) (headlessTextOwnedOutput, error) {
	before, err := inherited.Stat()
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSocket != 0 {
		return openHeadlessTextSocket(inherited)
	}
	opened, err := os.OpenFile(fmt.Sprintf("/proc/self/fd/%d", inherited.Fd()), os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	after, statErr := opened.Stat()
	if statErr != nil || !os.SameFile(before, after) {
		return nil, errors.Join(errors.New("text runtime output changed"), statErr, opened.Close())
	}
	if err := opened.SetWriteDeadline(time.Time{}); err != nil {
		return nil, errors.Join(err, opened.Close())
	}
	return opened, nil
}

type headlessTextOwnedOutput interface {
	headlessTextEventOutput
	Close() error
}

type headlessTextEventOutput interface {
	Write([]byte) (int, error)
	SetWriteDeadline(time.Time) error
}

func writeHeadlessTextEvent(ctx context.Context, output headlessTextEventOutput, event endpointapi.TextParticipantEvent) (outcome error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	deadline := time.Now().Add(5 * time.Second)
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	if err := output.SetWriteDeadline(deadline); err != nil {
		return err
	}
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); output.SetWriteDeadline(time.Now()) })
	defer func() {
		if !stop() {
			<-interrupted
		}
		outcome = errors.Join(outcome, ctx.Err())
	}()
	digest := ""
	if event.RequestDigest != [32]byte{} {
		digest = hex.EncodeToString(event.RequestDigest[:])
	}
	return json.NewEncoder(output).Encode(struct {
		Kind                 string `json:"kind"`
		NetworkID            string `json:"network_id"`
		Surface              string `json:"surface,omitempty"`
		RequestDigest        string `json:"request_digest,omitempty"`
		ApplicationSocket    string `json:"application_socket,omitempty"`
		AdministrationSocket string `json:"administration_socket,omitempty"`
	}{Kind: "headless-runtime-" + event.Kind, NetworkID: hex.EncodeToString(event.NetworkID[:]), Surface: event.Surface, RequestDigest: digest, ApplicationSocket: event.ApplicationAddress, AdministrationSocket: event.AdministrationAddress})
}
