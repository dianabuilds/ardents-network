package route

import "errors"

// ClosedChildRestriction is a negative constraint on one authenticated Node
// child. Ordinary does not supply any receiver admission or authority.
type ClosedChildRestriction uint8

const (
	ClosedChildOrdinary ClosedChildRestriction = iota
	ClosedChildIssuerBootstrap
)

// EncodeClosedNodeOpen is the mandatory 50-byte Node-outer grammar. Endpoint
// role channels keep EncodeClosedOpen and cannot supply this restriction.
func EncodeClosedNodeOpen(open ClosedOpen, restriction ClosedChildRestriction) ([]byte, error) {
	if restriction != ClosedChildOrdinary && restriction != ClosedChildIssuerBootstrap {
		return nil, errors.New("closed Node child restriction is invalid")
	}
	if restriction == ClosedChildIssuerBootstrap && open.Purpose != ClosedPurposeForwarding && open.Purpose != ClosedPurposeIssuer {
		return nil, errors.New("closed Node bootstrap purpose is unavailable")
	}
	body, err := EncodeClosedOpen(open)
	if err != nil {
		return nil, err
	}
	return append(body, byte(restriction)), nil
}

// DecodeClosedNodeOpen refuses the retired 49-byte outer form; absence never
// becomes ordinary admission. Channel authentication selects this decoder.
func DecodeClosedNodeOpen(body []byte) (ClosedOpen, ClosedChildRestriction, error) {
	if len(body) != 50 {
		return ClosedOpen{}, 0, errors.New("closed Node OPEN length is invalid")
	}
	open, err := DecodeClosedOpen(body[:49])
	restriction := ClosedChildRestriction(body[49])
	if err == nil {
		_, err = EncodeClosedNodeOpen(open, restriction)
	}
	if err != nil {
		return ClosedOpen{}, 0, err
	}
	return open, restriction, nil
}

// Restriction returns the immutable constraint authenticated when this Node
// child was allocated. A missing lane returns an unsupported value, not0.
func (lane *ClosedOuterBridgeLane) Restriction() ClosedChildRestriction {
	if lane == nil || lane.lane == nil {
		return ClosedChildRestriction(255)
	}
	return lane.lane.restriction
}
