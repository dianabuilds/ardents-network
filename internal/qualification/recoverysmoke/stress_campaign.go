package recoverysmoke

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/qualification/campaign"
)

const stressCampaignManifestSchema = "ardents-h3-s43-attempt-manifest-v1"

type stressCampaignManifest struct {
	Schema, SourceCommit, ImageID, ToolImageID, TopologyDigest string
	Topology                                                   []byte
	HostScope                                                  json.RawMessage
	RouteCase                                                  json.RawMessage
	Candidates                                                 []replacementCandidate
	RouteManifest                                              [32]byte
	Prerequisites                                              []qualificationPrerequisite
	Cells                                                      []replacementCampaignCell
}

func prepareStressCampaignManifest(observer dockerObserver, fixture prepared, hostScope hostScopeEvidence,
	imageID string, topology []byte, candidates []replacementCandidate) (json.RawMessage, error) {
	seeds := make(map[string][32]byte, 2)
	for _, direction := range []string{"client-to-publisher", "publisher-to-client"} {
		seed, err := recoveryDirectionSeed(observer.generation, direction)
		if err != nil {
			return nil, err
		}
		seeds[direction] = seed
	}
	offsets, lifetime, delay, mode := overlapReplacementSchedule()
	overlap, err := buildReplacementManifest("client-to-publisher", mode, seeds["client-to-publisher"],
		[]string{"initiator"}, offsets, lifetime, delay)
	if err != nil {
		return nil, err
	}
	cells := []replacementCampaignCell{
		{CellID: "c2p-overlap", Direction: "client-to-publisher", Mode: "overlap", ManifestDigest: overlap.Digest},
		stressImpairedCell("client-to-publisher", seeds["client-to-publisher"]),
		stressImpairedCell("publisher-to-client", seeds["publisher-to-client"]),
	}
	scopeRaw, err := json.Marshal(hostScope)
	if err != nil {
		return nil, fmt.Errorf("encode S4.3 campaign HostScope: %w", err)
	}
	manifest := stressCampaignManifest{Schema: stressCampaignManifestSchema, SourceCommit: observer.sourceCommit,
		ImageID: imageID, ToolImageID: observer.input.ToolImage, Topology: append([]byte(nil), topology...),
		TopologyDigest: digestText(topology), HostScope: scopeRaw, RouteCase: append(json.RawMessage(nil), fixture.routeCase...),
		Candidates: append([]replacementCandidate(nil), candidates...), RouteManifest: fixture.routeManifest,
		Prerequisites: append([]qualificationPrerequisite(nil), observer.input.Prerequisites...), Cells: cells}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("encode S4.3 campaign manifest: %w", err)
	}
	if err := campaign.PublishManifest(observer.input.EvidenceRoot, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func stressImpairedCell(direction string, seed [32]byte) replacementCampaignCell {
	prefix := "c2p"
	if direction == "publisher-to-client" {
		prefix = "p2c"
	}
	value, _ := json.Marshal(struct {
		Schema, Direction, Profile string
		Seed                       [32]byte
		Bytes                      uint32
	}{Schema: "ardents-h3-s43-impaired-cell-manifest-v1", Direction: direction,
		Profile: "h3-s43-impaired-v1", Seed: seed, Bytes: 192 << 20})
	digest := sha256.Sum256(value)
	return replacementCampaignCell{CellID: prefix + "-impaired", Direction: direction, Mode: "impaired",
		ManifestDigest: hex.EncodeToString(digest[:])}
}
