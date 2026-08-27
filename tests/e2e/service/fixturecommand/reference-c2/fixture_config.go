package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

func readConfig(path string) (config, error) {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 || len(raw) > 16<<10 {
		return config{}, errors.New("C2 fixture configuration is unavailable")
	}
	var input config
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return config{}, err
	}
	if input.Schema != "ardents-e2e-reference-c2-v1" || input.Epoch == 0 || input.PublicationPath == "" || input.PublisherRoot == "" || input.GatewayRoot == "" || input.GatewayProfilePath == "" || input.ReadyRoot == "" || input.CompletePath == "" || input.ResourceProofPath == "" || input.AlphaGatewayReadyPath == "" || input.AlphaRelayReadyPath == "" || input.PublisherApplicationAddress == "" || input.PublisherApplicationAddressPath == "" || input.PublisherApplicationReady == "" ||
		input.TransitAuthority == "" || input.Invite == "" || !input.SlotCredential.valid() ||
		!input.ResponderCredential.valid() || !input.IntroductionCredential.valid() || len(input.TransitStateRoots) != 4 || len(input.TransitStateMaterials) != 4 ||
		len(input.TransitStateSources) != 2 || input.TransitStateClient.Certificate == "" || input.TransitStateClient.PrivateKey == "" ||
		input.AlphaCorpusAuthority == "" || input.AlphaCorpusPrivate == "" || input.AlphaCorpusFloorRoot == "" || input.AlphaObserverCorpusFloorRoot == "" || input.AlphaServiceLink == "" {
		return config{}, errors.New("C2 fixture configuration is incomplete")
	}
	if _, err := input.deadline(); err != nil {
		return config{}, err
	}
	for _, value := range []string{input.Network, input.Digest, input.JoinHandle, input.Reachability, input.SlotAttachment, input.ServiceAttachment, input.ResolutionAttachment, input.InviteID, input.PublisherApplicationToken} {
		if _, err := fixed(value); err != nil {
			return config{}, err
		}
	}
	for _, value := range []peer{input.Introduction, input.Rendezvous, input.Responder, input.Initiator, input.Gateway} {
		if _, err := value.decode(); err != nil {
			return config{}, err
		}
	}
	for _, role := range []string{"rendezvous", "initiator", "introduction", "responder"} {
		if input.TransitStateRoots[role] == "" {
			return config{}, errors.New("C2 fixture transit State root is unavailable")
		}
		if _, present := input.TransitStateMaterials[role]; !present {
			return config{}, errors.New("C2 fixture transit State materialization is unavailable")
		}
	}
	for _, source := range input.TransitStateSources {
		if source.Address == "" || source.ServerName == "" || source.Root == "" {
			return config{}, errors.New("C2 fixture transit State source is unavailable")
		}
		if _, err := fixed(source.LeafKeyDigest); err != nil {
			return config{}, err
		}
	}
	if _, err := input.entryInvite(); err != nil {
		return config{}, err
	}
	if (input.HeldRouteReady == "") != (input.HeldRouteRelease == "") ||
		(input.HeldRouteReady == "") != (input.HeldRouteUserReady == "") ||
		input.HeldRouteReady != "" && (!validPublisherApplicationPath(input.HeldRouteReady) ||
			!validPublisherApplicationPath(input.HeldRouteUserReady) || !validPublisherApplicationPath(input.HeldRouteRelease)) {
		return config{}, errors.New("C2 fixture held-route control is invalid")
	}
	if _, _, err := input.alphaCorpusSigner(); err != nil {
		return config{}, err
	}
	if !filepath.IsAbs(input.AlphaCorpusFloorRoot) || !filepath.IsAbs(input.AlphaObserverCorpusFloorRoot) ||
		input.AlphaCorpusFloorRoot == input.AlphaObserverCorpusFloorRoot {
		return config{}, errors.New("C2 fixture alpha corpus floor root is invalid")
	}
	for _, path := range []string{input.AlphaGatewayReadyPath, input.AlphaRelayReadyPath} {
		if !filepath.IsAbs(path) {
			return config{}, errors.New("C2 fixture alpha private ready path is invalid")
		}
	}
	if _, err := alpha.ParseServiceLink(input.AlphaServiceLink); err != nil {
		return config{}, err
	}
	if input.FirefoxExecutable != "" && !filepath.IsAbs(input.FirefoxExecutable) {
		return config{}, errors.New("C2 fixture Firefox executable path is invalid")
	}
	if input.BrowserEntryStatePath != "" && (!input.TransparentApplication || input.FirefoxExecutable != "" || !filepath.IsAbs(input.BrowserEntryStatePath)) {
		return config{}, errors.New("C2 fixture Browser Entry state path is invalid")
	}
	if input.PublisherTerminal != "" && !input.TransparentApplication {
		return config{}, errors.New("C2 fixture Publisher terminal scenario requires the transparent Service")
	}
	switch input.PublisherTerminal {
	case "", publisherTerminalWithdrawal, publisherTerminalApplicationReset:
	case publisherTerminalEndpointStop:
		if !validPublisherApplicationPath(input.PublisherCrashReadyPath) {
			return config{}, errors.New("C2 fixture Publisher Endpoint stop control is invalid")
		}
	default:
		return config{}, errors.New("C2 fixture Publisher terminal scenario is invalid")
	}
	if !validPublisherApplicationAddress(input.PublisherApplicationAddress) {
		return config{}, errors.New("C2 fixture Publisher Application address is invalid")
	}
	if !filepath.IsAbs(input.PublisherApplicationAddressPath) {
		return config{}, errors.New("C2 fixture Publisher Application address path is invalid")
	}
	return input, nil
}

func (input config) deadline() (time.Time, error) {
	deadline, err := time.Parse(time.RFC3339, input.Deadline)
	if err != nil || !time.Now().UTC().Before(deadline) {
		return time.Time{}, errors.New("C2 fixture deadline is invalid")
	}
	return deadline.UTC(), nil
}

func (value peer) decode() (endpointapi.TransitPeer, error) {
	nodeID, err := fixed(value.NodeID)
	if err != nil {
		return endpointapi.TransitPeer{}, err
	}
	public, err := fixed(value.PublicKey)
	if err != nil || value.Endpoint == "" {
		return endpointapi.TransitPeer{}, errors.New("C2 fixture peer is invalid")
	}
	return endpointapi.TransitPeer{NodeID: nodeID, PublicKey: public, Family: sha256.Sum256(nodeID[:]), Endpoint: value.Endpoint}, nil
}
