package recovery

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	schema      = "ardents-h3-recovery-evidence-v1"
	streamBytes = uint32(4 << 20)
	queueLimit  = uint32(256 << 10)
)

var negativeNames = [...]string{"no-alternate", "cancellation", "deadline", "forged-attachment", "replayed-attachment",
	"stale-attachment", "cross-binding", "queue-full", "endpoint-restart"}

// Verify recomputes every public S4.1 candidate conjunct without candidate packages.
func Verify(value Evidence) Result {
	if value.Schema != schema || len(value.SourceCommit) != 40 || value.ImageID == "" || value.VerifierImageID != value.ImageID || value.TopologyDigest == "" ||
		value.ManifestDigest == "" || value.Claim != "S4.1 local development evidence only" {
		return invalid("identity, schema, or claim binding is incomplete")
	}
	if len(value.Topology) == 0 || len(value.Topology) > 1<<20 || hexDigest(value.Topology) != value.TopologyDigest {
		return invalid("topology bytes do not match their commitment")
	}
	if err := verifyTopology(value.Topology); err != nil {
		return invalid(err.Error())
	}
	if result := verifyManifest(value); result.Verdict != "pass" {
		return result
	}
	for _, binary := range []string{"ardents-route", "ardents-qualify", "ardents-service", "ardents-stream-app", "ardents-recovery-qualify"} {
		if len(value.BinaryDigests[binary]) != 64 {
			return invalid("binary identity is missing: " + binary)
		}
	}
	for _, binding := range [][32]byte{value.Target, value.Instance, value.NetworkID, value.CandidateView,
		value.IsolationContext, value.DestinationBinding, value.AuthorityPublic, value.ClientPrincipal, value.PublisherPrincipal} {
		if binding == [32]byte{} {
			return invalid("public connection binding is incomplete")
		}
	}
	if value.RouteProfile == "" || value.CredentialGeneration != 1 || value.CredentialNotBefore >= value.CredentialNotAfter ||
		value.WorkSafetyNotAfter > value.WorkSafetyMaximum || value.WorkSafetyMaximum > value.CredentialNotAfter ||
		value.NoNewRecoveryAfter > value.WorkSafetyNotAfter {
		return invalid("credential or Work Safety history is incomplete")
	}
	if value.RequestedNanos < int64(10*time.Minute) || value.RequestedNanos > int64(30*time.Minute) ||
		value.CampaignNanos < value.RequestedNanos || value.CampaignNanos > value.RequestedNanos+int64(30*time.Second) {
		return invalid("campaign duration is outside its frozen bound")
	}
	if len(value.Cells) < 2 {
		return invalid("both directional cells are required")
	}
	directions := map[string]bool{}
	observerIDs := map[string]bool{}
	controllerIDs := map[string]bool{}
	faultNetwork := ""
	for index := range value.Cells {
		directions[value.Cells[index].Direction] = true
		if result := verifyCell(value.Cells[index], value.ManifestDigest, value.ImageID); result.Verdict != "pass" {
			return result
		}
		observerID := value.Cells[index].ReplacementObserver.ContainerID
		if observerIDs[observerID] {
			return invalid("directional cells reused a transient replacement observer")
		}
		observerIDs[observerID] = true
		controllerID := value.Cells[index].FaultController
		if controllerIDs[controllerID] {
			return invalid("directional cells reused a removed fault controller")
		}
		controllerIDs[controllerID] = true
		if faultNetwork == "" {
			faultNetwork = value.Cells[index].FaultNetwork
		} else if value.Cells[index].FaultNetwork != faultNetwork {
			return invalid("directional cells do not share the frozen Carrier network")
		}
	}
	if !directions["client-to-publisher"] || !directions["publisher-to-client"] {
		return invalid("both directional cells are required")
	}
	for _, name := range negativeNames {
		negative, ok := value.Negatives[name]
		if !ok {
			return invalid("mandatory negative is missing: " + name)
		}
		if !negative.Passed || negative.TerminalCount != 1 || negative.Class == "" ||
			negative.WithinNanos <= 0 || negative.WithinNanos > int64(15*time.Second) || negative.ContainerID == "" {
			return fail("mandatory negative did not terminate exactly once within 15 seconds: " + name)
		}
		if name == "endpoint-restart" && (negative.InjectedResource != "publisher-endpoint" ||
			negative.BeforeProcess == "" || negative.AfterProcess == "" || negative.BeforeProcess == negative.AfterProcess) {
			return invalid("Endpoint restart process identity is incomplete")
		}
		injections := map[string]string{"replayed-attachment": "recovery-replayed-attachment",
			"stale-attachment": "recovery-stale-attachment", "cross-binding": "recovery-cross-binding"}
		if kind, required := injections[name]; required {
			_, digestErr := hex.DecodeString(negative.InjectionDigest)
			if negative.InjectionKind != kind || len(negative.InjectionDigest) != 64 || digestErr != nil {
				return invalid("continuity-proof injection identity is incomplete: " + name)
			}
			if name == "replayed-attachment" && (negative.AttackAttempts != 2 ||
				negative.RecoveryCount != 1 || negative.RouteGeneration != 2) {
				return fail("replayed proof was not captured after one accepted replacement")
			}
			if name != "replayed-attachment" && (negative.AttackAttempts != 1 ||
				negative.RecoveryCount != 0 || negative.RouteGeneration != 1) {
				return fail("stale or cross-binding proof did not terminate the initial recovery")
			}
		}
	}
	if !value.Cleanup.DockerEmpty || !value.Cleanup.FixtureAbsent || !value.Cleanup.PrivateMaterialAbsent {
		return invalid("cleanup or private-material removal is incomplete")
	}
	return Result{Verdict: "pass", Reason: "all frozen S4.1 recovery conjuncts passed"}
}

func verifyCell(cell Cell, manifestText, imageID string) Result {
	if cell.Direction != "client-to-publisher" && cell.Direction != "publisher-to-client" {
		return invalid("direction is invalid")
	}
	identities := []string{cell.ClientProcess, cell.PublisherProcess, cell.ClientApplicationProcess,
		cell.PublisherApplicationProcess, cell.InitialCarrier, cell.ReplacementCarrier,
		cell.FaultService, cell.FaultContainer, cell.FaultNetwork, cell.FaultController}
	for _, identity := range identities {
		if identity == "" {
			return invalid("process or carrier identity is missing")
		}
	}
	if result := verifyCarrierEvidence(cell, imageID); result.Verdict != "pass" {
		return result
	}
	manifest, err := hex.DecodeString(manifestText)
	if err != nil || len(manifest) != 32 {
		return invalid("manifest digest is malformed")
	}
	_ = manifest
	planned := (uint32(184) + uint32(cell.Seed[0]%8)) * 16_381
	if cell.CellManifestDigest != cellManifestDigest(cell.Direction, cell.Seed, planned) {
		return invalid("directional cell manifest does not bind seed and fault schedule")
	}
	if cell.Bytes != streamBytes || cell.PlannedFaultOffset != planned || cell.FaultOffset != cell.PlannedFaultOffset ||
		cell.DeliveredBeforeFault != cell.FaultOffset ||
		cell.FaultOffset%16_384 == 0 || cell.CanaryOffset < cell.FaultOffset || cell.CanaryOffset+32 > cell.Bytes {
		return invalid("fault, byte, or canary bounds are incomplete")
	}
	expected := workloadDigest(cell.Seed, cell.Bytes)
	canary := workloadRange(cell.Seed, cell.CanaryOffset)
	if expected != cell.ExpectedDigest || expected != cell.ObservedDigest || canary != cell.Canary {
		return fail("seeded bytes or unpredictable recovery canary differ")
	}
	if cell.LastDeliveryNanos <= 0 || cell.LastDeliveryNanos > cell.CarrierObservedNanos ||
		cell.CarrierObservedNanos > cell.FaultAtNanos || cell.FaultAtNanos <= 0 ||
		cell.FaultCompletedNanos < cell.FaultAtNanos ||
		cell.CarrierCutAfterNanos <= 0 || cell.AbsenceAfterNanos < cell.CarrierCutAfterNanos ||
		cell.AbsenceAfterNanos > cell.FaultCompletedNanos-cell.FaultAtNanos ||
		cell.CarrierAttachmentDeadlineNanos != int64(11*time.Second) ||
		cell.ChunkDelayNanos != int64(20*time.Millisecond) ||
		cell.OldCarrierRetiredNanos < cell.FaultAtNanos || cell.OldCarrierRetiredNanos > cell.FaultCompletedNanos ||
		cell.CanaryAtNanos < cell.FaultCompletedNanos ||
		cell.CanaryAtNanos <= cell.FaultAtNanos ||
		cell.CanaryAtNanos-cell.LastDeliveryNanos > int64(5*time.Second) || cell.TerminalAtNanos < cell.CanaryAtNanos ||
		cell.ReplacementObservedNanos < cell.CanaryAtNanos || cell.TerminalAtNanos < cell.ReplacementObservedNanos ||
		cell.TerminalAtNanos-cell.LastDeliveryNanos > int64(15*time.Second) {
		return fail("recovery timing missed its externally observed bound")
	}
	if cell.ClientRouteGeneration != 2 || cell.PublisherRouteGeneration != 2 ||
		cell.ClientRecoveryCount != 1 || cell.PublisherRecoveryCount != 1 ||
		cell.ClientApplicationAccepts != 1 || cell.PublisherApplicationAccepts != 1 ||
		cell.ClientRouteAccepts != 2 || cell.PublisherRouteAccepts != 2 ||
		cell.ClientContinuity == [32]byte{} || cell.ClientContinuity != cell.PublisherContinuity {
		return fail("generation or continuity observations do not prove one recovery")
	}
	if !cell.Ordered || !cell.Unique || !cell.SameConnection || cell.ApplicationReconnected ||
		cell.OldCarrierReused || !cell.OldCarrierRetired || !cell.FailedResourceUnavailable ||
		!cell.FaultResourceAbsent || !cell.TerminalClean {
		return fail("same-connection ordered recovery conjunct failed")
	}
	if cell.QueueHighWater > queueLimit {
		return fail(fmt.Sprintf("logical queue exceeded %d bytes", queueLimit))
	}
	if cell.MemoryHighWater == 0 || cell.OpenFilesHighWater == 0 || cell.GoroutinesHighWater == 0 ||
		cell.TimerHighWater == 0 || !cell.ExternalStatsObserved || cell.CarrierForwardBytes == 0 || cell.CarrierReverseBytes == 0 ||
		cell.CarrierForwardBytes > 64<<20 || cell.CarrierReverseBytes > 64<<20 || cell.CPUSeconds < 0 {
		return invalid("resource or traffic observations are incomplete")
	}
	if result := verifyResources(cell); result.Verdict != "pass" {
		return result
	}
	return Result{Verdict: "pass"}
}

func cellManifestDigest(direction string, seed [32]byte, planned uint32) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte("ardents-h3-recovery-cell-manifest-v1\x00" + direction +
		"\x00carrier-channel\x00carrier-attachment-deadline=11s\x00chunk-delay=20ms\x00"))
	_, _ = hash.Write(seed[:])
	var values [12]byte
	binary.BigEndian.PutUint32(values[:4], streamBytes)
	binary.BigEndian.PutUint32(values[4:8], planned)
	binary.BigEndian.PutUint32(values[8:12], 32)
	_, _ = hash.Write(values[:])
	return hex.EncodeToString(hash.Sum(nil))
}

func workloadDigest(seed [32]byte, count uint32) [32]byte {
	hash := sha256.New()
	for offset := uint32(0); offset < count; {
		block := workloadBlock(seed, uint64(offset/32))
		length := min(uint32(len(block)), count-offset)
		_, _ = hash.Write(block[:length])
		offset += length
	}
	var value [32]byte
	copy(value[:], hash.Sum(nil))
	return value
}

func workloadRange(seed [32]byte, offset uint32) [32]byte {
	var value [32]byte
	for index := uint32(0); index < uint32(len(value)); index++ {
		block := workloadBlock(seed, uint64((offset+index)/32))
		value[index] = block[(offset+index)%32]
	}
	return value
}

func workloadBlock(seed [32]byte, counter uint64) [32]byte {
	input := make([]byte, 40)
	copy(input, seed[:])
	binary.BigEndian.PutUint64(input[32:], counter)
	return sha256.Sum256(input)
}

func invalid(reason string) Result { return Result{Verdict: "invalid", Reason: reason} }
func fail(reason string) Result    { return Result{Verdict: "fail", Reason: reason} }
