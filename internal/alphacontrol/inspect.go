package alphacontrol

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"time"
)

var errCatalogFloorConflict = errors.New("alpha control catalog conflicts with its floor")

// Inspect binds supplied component bytes to one verified catalog, applies the
// reader-local floor, then invokes each fixed component's own verifier. It
// never exposes the supplied bytes to an Endpoint or acceptance owner.
func Inspect(raw []byte, public ed25519.PublicKey, roots [3]ed25519.PublicKey, components [3][]byte, prior Floor, at time.Time,
	verify ComponentVerifier) (Inspection, Floor, error) {
	catalog, digest, err := Verify(raw, public, at)
	if err != nil {
		return Inspection{CatalogDigest: digest, Catalog: OutcomeInvalid}, prior, err
	}
	if err := validateFloor(catalog, digest, prior); err != nil {
		return Inspection{CatalogDigest: digest, Catalog: floorOutcome(err)}, prior, err
	}
	if verify == nil {
		return Inspection{CatalogDigest: digest, Catalog: OutcomeInvalid}, prior, errors.New("alpha control component verifier is nil")
	}
	result := Inspection{CatalogDigest: digest, Catalog: OutcomeAccepted}
	next := Floor{CatalogGeneration: catalog.Generation, CatalogDigest: digest}
	for index, component := range catalog.Components {
		outcome := OutcomeUnavailable
		value := components[index]
		if len(value) != 0 {
			actual := sha256.Sum256(value)
			switch {
			case uint64(len(value)) != uint64(component.Size) || actual != component.Digest:
				outcome = OutcomeDigestMismatch
			case !at.Before(component.NotAfter):
				outcome = OutcomeExpired
			default:
				statement, verified := verifiedComponent(component, value, roots[index], at)
				if verified == OutcomeAccepted {
					outcome = verify(component, statement, at)
				} else {
					outcome = verified
				}
			}
		}
		result.Components[index] = ComponentInspection{Class: component.Class, Outcome: outcome}
		if outcome == OutcomeAccepted {
			next.Components[index] = ComponentFloor{Generation: component.Generation, Digest: component.Digest}
		} else {
			next.Components[index] = prior.Components[index]
		}
	}
	return result, next, nil
}

func validateFloor(catalog Catalog, digest [32]byte, prior Floor) error {
	if prior.CatalogGeneration == 0 {
		return nil
	}
	if catalog.Generation < prior.CatalogGeneration {
		return errors.New("alpha control catalog is below its floor")
	}
	if catalog.Generation == prior.CatalogGeneration && digest != prior.CatalogDigest {
		return errCatalogFloorConflict
	}
	if catalog.Generation > prior.CatalogGeneration && catalog.PreviousDigest != prior.CatalogDigest {
		return errors.New("alpha control catalog predecessor does not match its floor")
	}
	for index, component := range catalog.Components {
		floor := prior.Components[index]
		if floor.Generation == 0 {
			continue
		}
		if component.Generation < floor.Generation {
			return errors.New("alpha control component is below its floor")
		}
		if component.Generation == floor.Generation && component.Digest != floor.Digest {
			return errors.New("alpha control component conflicts with its floor")
		}
	}
	return nil
}

func floorOutcome(err error) Outcome {
	if errors.Is(err, errCatalogFloorConflict) {
		return OutcomeConflict
	}
	return OutcomeLowerFloor
}
