//go:build browsercompat

package main

type peer struct {
	NodeID, PublicKey, Endpoint, Certificate, PrivateKey string
}

// transitCredential is test-only closed-alpha provisioning material for one
// State-authorized Transit Grant and its matching ephemeral TLS key pair.
// It is never a participant configuration format.
type transitCredential struct {
	Grant, Certificate, PrivateKey string
}

type stateSource struct {
	Address, ServerName, Root, LeafKeyDigest string
}

type stateClient struct {
	Certificate, PrivateKey string
}

type config struct {
	Schema, Network, Digest, Deadline, PublicationPath, PublisherRoot, ReadyRoot, CompletePath, ResourceProofPath      string
	HeldRouteReady, HeldRouteUserReady, HeldRouteRelease                                                               string
	AlphaGatewayReadyPath, AlphaRelayReadyPath, AlphaRelayListenAddress                                                string
	CarrierRelayListenAddress, CarrierRelayTargetAddress, CarrierRelayReadyPath                                        string
	CarrierRelayResetPath, CarrierRelayResetResultPath                                                                 string
	PublisherApplicationAddress, PublisherApplicationAddressPath, PublisherApplicationToken, PublisherApplicationReady string
	PublisherCrashReadyPath, PublisherApplicationFaultReadyPath, PublisherApplicationFaultReleasePath                  string
	TransitFaultReadyPath                                                                                              string
	Epoch                                                                                                              uint64
	Introduction, Rendezvous, Responder, Initiator, Gateway                                                            peer
	JoinHandle, Reachability, SlotAttachment, ServiceAttachment, ResolutionAttachment                                  string
	GatewayRoot, GatewayProfilePath                                                                                    string
	TransitAuthority, InviteID, Invite                                                                                 string
	SlotCredential, ResponderCredential, IntroductionCredential                                                        transitCredential
	TransitStateRoots                                                                                                  map[string]string
	TransitStateMaterials                                                                                              map[string]uint32
	TransitStateSources                                                                                                []stateSource
	TransitStateClient                                                                                                 stateClient
	// AlphaCorpusPrivate exists only because this separate-process fixture
	// manufactures a signed corpus. A real participant receives only the
	// independently enrolled public authority through alpha control.
	AlphaCorpusAuthority, AlphaCorpusPrivate, AlphaCorpusFloorRoot, AlphaObserverCorpusFloorRoot, AlphaServiceLink string
	FirefoxExecutable, BrowserEntryStatePath                                                                       string
	PublisherTerminal                                                                                              publisherTerminal
	TransitFault                                                                                                   transitFault
	PublisherOffline, TransparentApplication                                                                       bool
	DynamicWorkload                                                                                                dynamicWorkloadConfig
}

type publicationEnvelope struct {
	AuthorityPublic, Publication, Descriptor, AlphaAuthorityPublic, AlphaCorpus, AlphaLink string
}
