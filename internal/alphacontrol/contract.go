package alphacontrol

import "time"

const (
	// MaximumCatalogSize bounds the complete signed alpha disclosure catalog.
	MaximumCatalogSize = 4096
	// ComponentRelease is the fixed release-evidence component.
	ComponentRelease ComponentClass = 1
	// ComponentNetwork is the fixed network-evidence component.
	ComponentNetwork ComponentClass = 2
	// ComponentCompatibility is the fixed compatibility-evidence component.
	ComponentCompatibility ComponentClass = 3
	// ComponentCorpus is the independently signed Alpha Name Corpus
	// component available only in versioned ACA2 control catalogs.
	ComponentCorpus ComponentClass = 4
)

// ComponentClass is a closed alpha-control component vocabulary.
type ComponentClass byte

// Component binds one reader-owned fixed filename to externally verified
// content. RootID is disclosure only: it must never be supplied to an Endpoint
// acceptance path.
type Component struct {
	Class      ComponentClass
	RootID     [32]byte
	Generation uint64
	NotAfter   time.Time
	Size       uint32
	Digest     [32]byte
}

// Catalog is one signed, finite disclosure of an alpha cohort. Signature is
// always Ed25519 over the canonical binary payload and never grants component
// acceptance authority.
type Catalog struct {
	Cohort              string
	Generation          uint64
	NotBefore, NotAfter time.Time
	PreviousDigest      [32]byte
	Components          [3]Component
	Signature           [64]byte
}

// CatalogV2 is the versioned four-component alpha-control successor. It is
// intentionally distinct from ACA1's closed three-component Catalog.
type CatalogV2 struct {
	Cohort              string
	Generation          uint64
	NotBefore, NotAfter time.Time
	PreviousDigest      [32]byte
	Components          [4]Component
	Signature           [64]byte
}

// Floor retains only reader-local non-decreasing catalog/component facts. It
// is intentionally distinct from Release and Network State floors.
type Floor struct {
	CatalogGeneration uint64
	CatalogDigest     [32]byte
	Components        [3]ComponentFloor
}

// ComponentFloor is the reader-local floor for one fixed component class.
type ComponentFloor struct {
	Generation uint64
	Digest     [32]byte
}

// Outcome is one component-local reader result. It has no Endpoint readiness
// or permission meaning.
type Outcome string

const (
	OutcomeAccepted       Outcome = "accepted"
	OutcomeUnavailable    Outcome = "unavailable"
	OutcomeDigestMismatch Outcome = "digest-mismatch"
	OutcomeExpired        Outcome = "expired"
	OutcomeLowerFloor     Outcome = "lower-floor"
	OutcomeConflict       Outcome = "conflict"
	OutcomeInvalid        Outcome = "invalid"
)

// ComponentVerifier performs one component-local verification after the
// catalog has bound the exact bytes and the independently supplied component
// root has verified its statement. It must not call or start an Endpoint.
type ComponentVerifier func(Component, ComponentStatement, time.Time) Outcome

// Inspection reports a catalog and each independently verified component.
type Inspection struct {
	CatalogDigest [32]byte
	Catalog       Outcome
	Components    [3]ComponentInspection
}

// ComponentInspection reports one fixed component class without exposing its
// payload or an arbitrary location.
type ComponentInspection struct {
	Class   ComponentClass `json:"class"`
	Outcome Outcome        `json:"outcome"`
}
