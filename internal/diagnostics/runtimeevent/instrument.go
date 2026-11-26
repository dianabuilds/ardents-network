package runtimeevent

// Instrument identifies one measured owner quantity. It grants no authority.
type Instrument uint8

const (
	EndpointStartup Instrument = iota + 1
	NodeLifetime
	NetworkRefresh
	NetworkFetch
	AdmissionPresent
	AdmissionImport
	HostingTransaction
	HostingWait
	RouteOutputWait
	ExecutionLaunch
	PublicationPublish
	PublicationRefresh
	ConnectionAuthenticate
	ConnectionRead
	ConnectionWrite
	ReachabilityPublish
	ReachabilityLookup
	EnrollmentVerify
	ReleaseVerify
	CustodyEncrypt
	CustodyDecrypt
	RouteReservedBytes
	RouteChildren
	RouteChannels
	RouteOutputFrames
	HostingWaiters
	ExecutionCleanupSlots
	ConnectionSendBytes
	ConnectionReceiveBytes
	RouteSendCredit
	RouteReceiveCredit
	AdmissionStockTokens
	RouteQueuedFrames
	HostingObservedUsed
	HostingObservedReserved
	HostingObservedRemaining
	ReleaseEvaluate
	NetworkSourceHandle
	PublicationInitialSlots
	PublicationReadyCandidates
)

const MeasurementSchema = "ardents-domain-measurements-v1"
const MeasurementKind = "domain-measurements"
const MeasurementScope = "ardents.domain"

// InstrumentKind distinguishes a timed call, an aggregate retained quantity,
// and the last confirmed owning transaction. It is not an input/wire label.
type InstrumentKind uint8

const (
	StageInstrument InstrumentKind = iota + 1
	GaugeInstrument
	LastSampleInstrument
)

// MeasurementUnit is the catalogue's unit, never a caller-supplied attribute.
type MeasurementUnit string

const (
	BytesUnit         MeasurementUnit = "bytes"
	SecondsUnit       MeasurementUnit = "seconds"
	SlotsUnit         MeasurementUnit = "slots"
	FramesUnit        MeasurementUnit = "frames"
	TokensUnit        MeasurementUnit = "tokens"
	OperationsUnit    MeasurementUnit = "operations"
	CallsUnit         MeasurementUnit = "calls"
	SnapshotsUnit     MeasurementUnit = "snapshots"
	DimensionlessUnit MeasurementUnit = "1"
)

// OTLP returns the corresponding fixed UCUM annotation, or empty for no unit.
func (unit MeasurementUnit) OTLP() string {
	switch unit {
	case BytesUnit:
		return "By"
	case SecondsUnit:
		return "s"
	case SlotsUnit:
		return "{slot}"
	case FramesUnit:
		return "{frame}"
	case TokensUnit:
		return "{token}"
	case OperationsUnit:
		return "{operation}"
	case CallsUnit:
		return "{call}"
	case SnapshotsUnit:
		return "{snapshot}"
	case DimensionlessUnit:
		return "1"
	}
	return ""
}

// InstrumentSpec fixes cardinality, owner, quantity kind and unit.
type InstrumentSpec struct {
	Name, Owner string
	Kind        InstrumentKind
	Unit        MeasurementUnit
}

var instrumentSpecs = [...]InstrumentSpec{
	{},
	ReleaseEvaluate:            {"release.evaluate", "release", StageInstrument, SecondsUnit},
	NetworkSourceHandle:        {"network.source_request", "network", StageInstrument, SecondsUnit},
	PublicationInitialSlots:    {"publication.initial_delivery_slots", "publication", GaugeInstrument, SlotsUnit},
	PublicationReadyCandidates: {"publication.ready_candidate_slots", "publication", GaugeInstrument, SlotsUnit},
	HostingObservedUsed:        {"hosting.last_observed_used_bytes", "hosting", LastSampleInstrument, BytesUnit},
	HostingObservedReserved:    {"hosting.last_observed_reserved_bytes", "hosting", LastSampleInstrument, BytesUnit},
	HostingObservedRemaining:   {"hosting.last_observed_remaining_bytes", "hosting", LastSampleInstrument, BytesUnit},
	RouteQueuedFrames:          {"route.queued_output_frames", "route", GaugeInstrument, FramesUnit},
	RouteSendCredit:            {"route.active_send_credit_bytes", "route", GaugeInstrument, BytesUnit},
	RouteReceiveCredit:         {"route.active_receive_credit_bytes", "route", GaugeInstrument, BytesUnit},
	AdmissionStockTokens:       {"admission.stock_tokens", "admission", GaugeInstrument, TokensUnit},
	EndpointStartup:            {"endpoint.startup", "endpoint", StageInstrument, SecondsUnit},
	NodeLifetime:               {"node.lifetime", "node", StageInstrument, SecondsUnit},
	NetworkRefresh:             {"network.refresh", "network", StageInstrument, SecondsUnit},
	NetworkFetch:               {"network.fetch_verify", "network", StageInstrument, SecondsUnit},
	AdmissionPresent:           {"admission.present", "admission", StageInstrument, SecondsUnit},
	AdmissionImport:            {"admission.import_batch", "admission", StageInstrument, SecondsUnit},
	HostingTransaction:         {"hosting.transaction", "hosting", StageInstrument, SecondsUnit},
	HostingWait:                {"hosting.owner_wait", "hosting", StageInstrument, SecondsUnit},
	RouteOutputWait:            {"route.output_admission_wait", "route", StageInstrument, SecondsUnit},
	ExecutionLaunch:            {"execution.launch", "execution", StageInstrument, SecondsUnit},
	PublicationPublish:         {"publication.publish", "publication", StageInstrument, SecondsUnit},
	PublicationRefresh:         {"publication.refresh", "publication", StageInstrument, SecondsUnit},
	ConnectionAuthenticate:     {"connection.authenticate", "connection", StageInstrument, SecondsUnit},
	ConnectionRead:             {"connection.read", "connection", StageInstrument, SecondsUnit},
	ConnectionWrite:            {"connection.write", "connection", StageInstrument, SecondsUnit},
	ReachabilityPublish:        {"reachability.publish", "reachability", StageInstrument, SecondsUnit},
	ReachabilityLookup:         {"reachability.lookup", "reachability", StageInstrument, SecondsUnit},
	EnrollmentVerify:           {"enrollment.verify", "enrollment", StageInstrument, SecondsUnit},
	ReleaseVerify:              {"release.verify_initial", "release", StageInstrument, SecondsUnit},
	CustodyEncrypt:             {"custody.encrypt", "custody", StageInstrument, SecondsUnit},
	CustodyDecrypt:             {"custody.decrypt", "custody", StageInstrument, SecondsUnit},
	RouteReservedBytes:         {"route.queue_budget_claim_bytes", "route", GaugeInstrument, BytesUnit},
	RouteChildren:              {"route.children", "route", GaugeInstrument, SlotsUnit},
	RouteChannels:              {"route.channels", "route", GaugeInstrument, SlotsUnit},
	RouteOutputFrames:          {"route.outstanding_output_frames", "route", GaugeInstrument, FramesUnit},
	HostingWaiters:             {"hosting.waiters", "hosting", GaugeInstrument, OperationsUnit},
	ExecutionCleanupSlots:      {"execution.cleanup_slots", "execution", GaugeInstrument, SlotsUnit},
	ConnectionSendBytes:        {"connection.active_send_queue_bytes", "connection", GaugeInstrument, BytesUnit},
	ConnectionReceiveBytes:     {"connection.active_receive_queue_bytes", "connection", GaugeInstrument, BytesUnit},
}

func (id Instrument) Spec() (InstrumentSpec, bool) {
	if id == 0 || int(id) >= len(instrumentSpecs) {
		return InstrumentSpec{}, false
	}
	spec := instrumentSpecs[id]
	validKind := spec.Kind == StageInstrument || spec.Kind == GaugeInstrument || spec.Kind == LastSampleInstrument
	return spec, spec.Name != "" && spec.Owner != "" && spec.Unit.OTLP() != "" && validKind
}
func Instruments() []Instrument {
	result := make([]Instrument, 0, len(instrumentSpecs)-1)
	for id := Instrument(1); int(id) < len(instrumentSpecs); id++ {
		result = append(result, id)
	}
	return result
}
func LookupInstrument(name string) Instrument {
	for id := Instrument(1); int(id) < len(instrumentSpecs); id++ {
		if instrumentSpecs[id].Name == name {
			return id
		}
	}
	return 0
}

// Measurement is an aggregate in one explicitly selected local capture, never
// a per-Route/Target/Job series. Missing instruments remain unobserved.
// Stage count/sum/maximum describe all ended calls, including refusals; active
// calls have no fabricated duration. Gauges retain current and observed peak.
type Measurement struct {
	Name           string  `json:"name"`
	Available      bool    `json:"available"`
	Current        uint64  `json:"current"`
	Peak           uint64  `json:"peak"`
	Count          uint64  `json:"count"`
	Seconds        float64 `json:"seconds"`
	MaximumSeconds float64 `json:"maximum_seconds"`
}
